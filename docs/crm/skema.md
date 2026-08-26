# Skema Database CRM Desa+

> Status: **Rancangan untuk direview** (2026-08-07). Belum ada migrasi/kode.
> Sumber: spec 22 halaman (Modul 1–9) + [`sistem-dan-role.md`](sistem-dan-role.md).
> Dibaca bersama [`STARTER.md`](../../STARTER.md) & [`CLAUDE.md`](../../CLAUDE.md).

Dokumen ini mendefinisikan **19 tabel** untuk seluruh 9 modul CRM, plus view
untuk Dashboard & Reports. Ia menjadi acuan tunggal sebelum migrasi goose ditulis
(`00002_crm_schema.sql` dst). Setiap tabel mencantumkan kolom, tipe Postgres, FK,
index, aturan RLS, dan **catatan keputusan** — kenapa begini, bukan sekadar apa.

---

## 0. Konvensi umum (berlaku untuk SEMUA tabel CRM)

Mengikuti template, bukan menciptakan pola baru:

- **PK** `id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY` — bukan serial, bukan
  uuid. Konsisten dengan `users`/`tenants`. Kolom identitas tak butuh GRANT
  sequence terpisah untuk `app_rw` (terverifikasi di template).
- **Semua waktu `TIMESTAMPTZ` (UTC).** Konversi ke lokal (`APP_TIMEZONE`,
  Asia/Jakarta) HANYA saat agregasi via `AT TIME ZONE` di SQL — tak pernah
  disimpan lokal (gotcha #14). Field spec bertipe "Date" murni (tanpa jam,
  mis. `start_date` langganan) → `DATE`; yang bertipe "DateTime" → `TIMESTAMPTZ`.
- **Uang** `NUMERIC(15,2)` — bukan float. Currency IDR bisa besar (APBDes miliaran).
- **`tenant_id BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE`** di
  SETIAP tabel CRM (kecuali junction yang mewarisi lewat parent). Ini kunci
  isolasi RLS, bukan sekadar kolom filter.
- **Audit kolom** di tiap tabel utama:
  ```
  created_by     BIGINT REFERENCES users(id) ON DELETE SET NULL,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by     BIGINT REFERENCES users(id) ON DELETE SET NULL,
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
  ```
  `ON DELETE SET NULL` (bukan CASCADE): jejak "siapa membuat" harus bertahan
  meski pembuatnya terhapus — sama alasan dengan `audit_logs.actor_user_id`.
- **Soft-delete** `deleted_at TIMESTAMPTZ` pada tabel yang barisnya bisa dibuang
  user (accounts, contacts, leads, deals, dst). Query WAJIB `WHERE deleted_at IS
  NULL`; index utama partial pada kondisi itu. Master/log tertentu tak perlu.
- **RLS** — setiap tabel tenant-scoped memakai policy `tenant_isolation` yang
  IDENTIK dengan template:
  ```sql
  ALTER TABLE <t> ENABLE ROW LEVEL SECURITY;
  ALTER TABLE <t> FORCE  ROW LEVEL SECURITY;   -- pemilik tabel pun terkena
  CREATE POLICY tenant_isolation ON <t>
      USING (COALESCE(current_setting('app.is_super', true),'off')='on'
             OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
      WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
             OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
  ```
  GRANT `app_rw` untuk tabel baru **otomatis** lewat `ALTER DEFAULT PRIVILEGES`
  yang sudah dipasang migrasi 00001 — migrasi CRM tak perlu GRANT manual, TAPI
  wajib `ENABLE`+`FORCE`+`POLICY` per tabel (RLS tak diwariskan).
- **Enum = `TEXT` + `CHECK (... IN (...))`**, bukan tipe `ENUM` Postgres. Sama
  seperti `status`/`role` di template — menambah nilai = ubah CHECK, tak perlu
  `ALTER TYPE` yang mengunci. Picklist di spec → CHECK constraint.
- **`table_schema` di query mana pun WAJIB `current_schema()`**, bukan `'public'`
  harfiah (gotcha CLAUDE.md — test tiap paket punya schema sendiri).
- **Semua query digenerate sqlc** — tak ada SQL mentah dari user (menjaga asumsi
  keamanan `SET LOCAL ROLE app_rw`; injection tak bisa `RESET ROLE`).

### Kenapa "desa" = DATA, bukan tenant

Desa+ (perusahaan) adalah **tenant/workspace**. Desa-desa pelanggannya adalah
**baris `accounts`** di dalam workspace itu. Jadi `accounts.tenant_id` selalu
menunjuk workspace Desa+, dan RLS mengisolasi antar-workspace Desa+ (kalau kelak
ada reseller/multi-org), BUKAN antar-desa. Isolasi antar-desa (ownership Sales/CSM)
ditegakkan **di layer aplikasi** (filter query sqlc), sesuai keputusan awal —
bukan RLS lapis kedua.

---

## 1. Sumbu role bisnis — kolom di `memberships` (bukan tabel baru)

Modul 9 (Settings) & `sistem-dan-role.md` §4b: role bisnis (Admin/Manager/Sales/
CSM/Support) disimpan **per-keanggotaan**, bukan per-user — orang yang sama bisa
Manager di satu workspace, CSM di workspace lain.

**Perubahan pada tabel `memberships` yang SUDAH ADA** (migrasi inkremental,
`ALTER TABLE`, idempotent):

```sql
ALTER TABLE memberships
    ADD COLUMN IF NOT EXISTS business_role TEXT;   -- NULL = belum diberi peran CRM
ALTER TABLE memberships
    ADD CONSTRAINT memberships_business_role_chk
    CHECK (business_role IS NULL OR business_role IN
           ('admin','manager','sales','csm','support'));
```

- **`NULL` = tak punya peran CRM** (mis. super_admin/owner platform yang bukan
  bagian tim penjualan — §4 dok role: owner bisa hadir tanpa memegang desa).
  Nullable, sama pola dengan `workspace_quota`.
- **Satu orang = satu `business_role` per workspace** — perangkapan ditolak di
  handler (bukan constraint DB; role gabungan `Sales-CSM` kelak jadi nilai enum
  tersendiri bila diperlukan).
- Sumbu ini **ortogonal** dari `memberships.role` (owner/admin/member = sumbu
  tenant) dan dari role platform. Nesting hak dari role, keputusan bypass RLS
  tetap dari role platform — tak pernah dari `business_role`.

---

## 2. `accounts` — Desa (HUB) · Modul 2

Pusat sistem. Semua modul lain menyambung ke sini.

| Kolom | Tipe | Sumber / Catatan |
|---|---|---|
| id | BIGINT IDENTITY PK | |
| tenant_id | BIGINT NN → tenants | workspace Desa+ |
| **account_owner** | BIGINT → users | 2.A · Sales/staf penanggung jawab. **Kolom di hub** (dipakai filter ownership hampir semua modul → hindari JOIN) |
| **assigned_csm** | BIGINT → users | 2.E · CSM utama (§8.1). Otoritatif di sini, bukan rollup dari M6 |
| **backup_csm** | BIGINT → users | §8.1 · CSM cadangan opsional |
| village_name | TEXT NN | 2.A · nama resmi desa |
| village_code | TEXT | 2.A · Kode Kemendagri (`33.09.12.2001`). Nullable (prospect awal belum tentu punya) |
| account_type | TEXT NN | 2.A picklist: `prospect`/`customer`/`former_customer`. CHECK |
| parent_account_id | BIGINT → accounts | 2.A · self-FK hierarki (Kec/Kab). Nullable, **opsional v1** |
| website | TEXT | 2.A |
| description | TEXT | 2.A |
| district_id | BIGINT → regions | 2.B · Kecamatan (master wilayah 3-level, ADR 0009). Nullable — NULL pada baris pre-migrasi sampai di-assign ulang |
| province_legacy, regency_legacy, district_legacy | TEXT | 2.B · **legacy**, bekas kolom teks bebas (lihat keputusan) — tak lagi ditulis, jejak bantu backfill manual |
| village_address | TEXT | 2.B |
| postal_code | TEXT | 2.B |
| latitude | NUMERIC(10,7) | 2.B · Geo. Bukan PostGIS |
| longitude | NUMERIC(10,7) | 2.B |
| territory | TEXT | 2.B · wilayah kerja tim internal (mis. "Regional Jawa Timur") |
| village_status | TEXT | 2.C · Desa/Kelurahan/Nagari/Gampong (picklist, CHECK) |
| village_classification | TEXT | 2.C · IDM: Mandiri/Maju/Berkembang/Tertinggal/Sangat Tertinggal |
| population | INTEGER | 2.C |
| hamlets_count | INTEGER | 2.C · jumlah dusun |
| village_budget | NUMERIC(15,2) | 2.C · APBDes (indikator kapasitas beli) |
| contact_phone | TEXT | 2.C |
| office_phone | TEXT | 2.C |
| office_email | TEXT | 2.C |
| deleted_at | TIMESTAMPTZ | soft-delete |
| created_by/at, updated_by/at | | audit standar |

**TIDAK jadi kolom** (rollup — dihitung saat query, gotcha sumber-kebenaran-ganda):
- 2.D Subscription Summary (status/plan/start/renewal) → dari `subscriptions`
- 2.E Health Score / Lifecycle Stage / Last Engagement → dari `customer_success`

**Index:**
```sql
CREATE UNIQUE INDEX idx_accounts_code ON accounts (tenant_id, village_code)
    WHERE village_code IS NOT NULL AND deleted_at IS NULL;   -- guard duplikat desa
CREATE INDEX idx_accounts_live   ON accounts (tenant_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_accounts_owner  ON accounts (tenant_id, account_owner) WHERE deleted_at IS NULL;
CREATE INDEX idx_accounts_csm    ON accounts (tenant_id, assigned_csm)  WHERE deleted_at IS NULL;
CREATE INDEX idx_accounts_district ON accounts (tenant_id, district_id)
    WHERE deleted_at IS NULL;   -- Territory/Region View (2.3), GROUP BY district_id
```

**Keputusan:**
- **Region = master wilayah administratif 3-level** (Provinsi/Kabupaten-Kota/
  Kecamatan) di tabel global `regions`, direferensikan via `district_id` —
  lihat [ADR 0009](../decisions/0009-master-wilayah-administratif.md). Desa
  itu sendiri (`village_name`/`village_code`) **tetap** teks bebas milik akun
  — `accounts` masih menampung desa *pipeline* saja (bukan ~83rb desa
  Indonesia), hanya jenjang administratif di atasnya yang jadi master.
  Territory View = `GROUP BY district_id` (join `regions` untuk nama).
- **`village_code` unique partial** = pengaman duplikat saat konversi Lead →
  tabrakan ketahuan di DB, bukan jadi desa ganda.
- **Ownership authoritative di hub** (§8) — `account_owner`/`assigned_csm`/
  `backup_csm` kolom di sini karena filter `WHERE account_owner=$uid OR
  assigned_csm=$uid OR backup_csm=$uid` dipakai lintas modul. `customer_success`
  merujuk, tak menduplikasi.
- **`parent_account_id` self-FK opsional v1** — kolom ada (nullable, murah), tapi
  pohon administratif tak wajib diisi. `account_type` belum punya nilai
  "administrative"; ditambah bila hierarki dipakai serius.

---

## 2b. `regions` — Master Wilayah Administratif (dipakai §2 `accounts` & §4a `leads`)

Tabel **GLOBAL** (tanpa `tenant_id`, tanpa RLS — pola sama `platform_staff`),
lihat [ADR 0009](../decisions/0009-master-wilayah-administratif.md) untuk
rasional lengkap.

| Kolom | Tipe | Sumber / Catatan |
|---|---|---|
| id | BIGINT IDENTITY PK | |
| parent_region_id | BIGINT → regions | self-FK, nullable (NULL di level 1). `ON DELETE CASCADE` |
| level | SMALLINT NN CHECK (1,2,3) | 1=Provinsi, 2=Kabupaten/Kota, 3=Kecamatan |
| code | TEXT NN UNIQUE | kode resmi berjenjang titik (`"11.01.01"`), sumber cahyadsn/wilayah |
| name | TEXT NN | |

Di-seed sekali via migrasi `00026_crm_regions.sql` (±7.833 baris: 34 provinsi
+ ~514 kab/kota + ~7.285 kecamatan), sumber dataset **cahyadsn/wilayah**
(GitHub, MIT License). **Tidak** mencakup level Desa/Kelurahan — level itu
tetap teks bebas milik akun (`village_name`).

---

## 3. `contacts` — Kontak (perangkat desa) · Modul 3

| Kolom | Tipe | Sumber / Catatan |
|---|---|---|
| id | BIGINT IDENTITY PK | |
| tenant_id | BIGINT NN → tenants | |
| account_id | BIGINT NN → accounts | 3.A · desa tempat kontak bertugas |
| contact_owner | BIGINT → users | 3.A · staf pemegang relasi |
| reports_to_id | BIGINT → contacts | 3.A · self-FK hierarki internal desa. Nullable |
| first_name | TEXT NN | 3.A |
| last_name | TEXT | 3.A |
| salutation | TEXT | 3.A · Bapak/Ibu/Sdr (picklist) |
| job_title | TEXT | 3.A · jabatan bebas ("Kepala Desa") |
| position_category | TEXT | 3.B · Kepala Desa/Sekdes/Kaur/Kasi/Operator/Bendahara/BPD/Lainnya (CHECK) |
| contact_role | TEXT | 3.B · Decision Maker/Influencer/User/Finance/Gatekeeper |
| is_primary_contact | BOOLEAN NN DEFAULT false | 3.B |
| is_technical_contact | BOOLEAN NN DEFAULT false | 3.B · operator yg benar-benar pakai software |
| term_period | TEXT | 3.B · masa jabatan ("2021–2027") |
| mobile_phone | TEXT | 3.C |
| whatsapp_number | TEXT | 3.C · kanal utama |
| office_phone | TEXT | 3.C |
| email | TEXT | 3.C |
| preferred_channel | TEXT | 3.C · WhatsApp/Telepon/Email/Kunjungan (CHECK) |
| mailing_address | TEXT | 3.D |
| city | TEXT | 3.D |
| postal_code | TEXT | 3.D |
| **email_opt_out** | BOOLEAN NN DEFAULT false | 3.E · **writable** (KOREKSI spec: bukan rollup — flag kontrol manual) |
| **do_not_contact** | BOOLEAN NN DEFAULT false | 3.E · **writable** |
| deleted_at | TIMESTAMPTZ | |
| created_by/at, updated_by/at | | audit |

**TIDAK jadi kolom** (rollup dari `activities`): `last_contacted_date`, `last_activity`.

**Index:**
```sql
CREATE INDEX idx_contacts_account ON contacts (tenant_id, account_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_contacts_live    ON contacts (tenant_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX idx_contacts_primary ON contacts (tenant_id, account_id)
    WHERE is_primary_contact AND deleted_at IS NULL;   -- maks 1 kontak utama per desa
```

**Keputusan — koreksi label spec:** 3.E menandai Email Opt-Out & Do Not Contact
"read-only rollup", **itu keliru**. Keduanya flag kontrol yang harus bisa ditulis
manual saat kontak minta berhenti dihubungi. Yang benar rollup di 3.E hanya
Last Contacted / Last Activity.

---

## 4. Sales · Modul 4 (`leads`, `deals`, `quotes`, `quote_items`)

### 4a. `leads` — entity flat, dikonversi

Lead = entity terpisah dengan field region/kontak **mentah** (text), BUKAN
account. Konversi (Salesforce-style) membuat account+contact+deal.

| Kolom | Tipe | Sumber |
|---|---|---|
| id, tenant_id | | |
| lead_name | TEXT NN | 4.1.A · nama desa/lead |
| contact_person | TEXT | 4.1.A |
| job_title | TEXT | 4.1.A |
| lead_owner | BIGINT → users | 4.1.A |
| lead_source | TEXT | 4.1.A · picklist |
| lead_status | TEXT NN | 4.1.B · New/Contacted/Qualified/Unqualified/Converted (CHECK) |
| rating | TEXT | 4.1.B · Hot/Warm/Cold |
| unqualified_reason | TEXT | 4.1.B · termasuk Duplicate |
| estimated_value | NUMERIC(15,2) | 4.1.B |
| district_id | BIGINT → regions | 4.1.C · Kecamatan (master wilayah 3-level, ADR 0009), mentah — belum jadi account |
| province_legacy, regency_legacy, district_legacy | TEXT | 4.1.C · **legacy**, bekas kolom teks bebas — tak lagi ditulis |
| mobile_phone, whatsapp, email | TEXT | 4.1.C |
| converted | BOOLEAN NN DEFAULT false | 4.1.B |
| converted_account_id | BIGINT → accounts | hasil konversi (nullable) |
| converted_contact_id | BIGINT → contacts | |
| converted_deal_id | BIGINT → deals | |
| converted_at | TIMESTAMPTZ | |
| deleted_at, audit | | |

Index: `(tenant_id, lead_status, created_at DESC)` partial live; `(tenant_id, lead_owner)`.

### 4b. `deals` — pipeline

| Kolom | Tipe | Sumber |
|---|---|---|
| id, tenant_id | | |
| deal_name | TEXT NN | 4.2.A |
| account_id | BIGINT NN → accounts | 4.2.A |
| primary_contact_id | BIGINT → contacts | 4.2.A |
| deal_owner | BIGINT → users | 4.2.A · Sales |
| deal_type | TEXT | 4.2.A · New Business/Renewal/Upsell/Cross-sell (CHECK) |
| stage | TEXT NN | 4.2.B · Prospecting/Qualification/Demo/Proposal/Negotiation/Closed Won/Closed Lost |
| amount | NUMERIC(15,2) | 4.2.B |
| probability | SMALLINT | 4.2.B · 0–100 |
| expected_close_date | DATE | 4.2.B |
| forecast_category | TEXT | 4.2.B |
| next_step | TEXT | 4.2.B |
| closed_date | DATE | 4.2.C |
| win_loss_reason | TEXT | 4.2.C |
| competitor | TEXT | 4.2.C |
| loss_notes | TEXT | 4.2.C |
| plan_requested_id | BIGINT → plans | 4.2.D |
| subscription_term | TEXT | 4.2.D · Monthly/Annual/Multi-year |
| created_subscription_id | BIGINT → subscriptions | 4.2.D · hasil Closed Won |
| deleted_at, audit | | |

Index: `(tenant_id, stage, created_at DESC)` partial live; `(tenant_id, deal_owner)`;
`(tenant_id, account_id)`.

### 4c. `quotes` + 4d. `quote_items`

`quotes` (header):

| Kolom | Tipe | Sumber |
|---|---|---|
| id, tenant_id | | |
| quote_number | TEXT NN | 4.3.A · auto (di-generate app, unik per tenant) |
| quote_name | TEXT | 4.3.A |
| deal_id | BIGINT → deals | 4.3.A |
| account_id | BIGINT NN → accounts | 4.3.A |
| quote_status | TEXT NN | 4.3.A · Draft/Sent/Under Review/Accepted/Rejected/Expired |
| expiration_date | DATE | 4.3.A |
| payment_terms | TEXT | 4.3.C |
| notes_terms | TEXT | 4.3.C |
| prepared_by | BIGINT → users | 4.3.C |
| grand_total | NUMERIC(15,2) | 4.3.B · disimpan (snapshot; item bisa berubah) |
| tax_amount | NUMERIC(15,2) | 4.3.B · PPN |
| deleted_at, audit | | |

Index: `(tenant_id, quote_status, created_at DESC)`; unique `(tenant_id, quote_number)`.

`quote_items` (line-item, 4.3.B — spec eksplisit punya baris item):

| Kolom | Tipe | Sumber |
|---|---|---|
| id | BIGINT IDENTITY PK | |
| quote_id | BIGINT NN → quotes ON DELETE CASCADE | anak dari quote |
| tenant_id | BIGINT NN → tenants | (untuk RLS langsung; diisi = quote.tenant_id) |
| plan_id | BIGINT → plans | 4.3.B · Product/Plan |
| quantity | INTEGER NN DEFAULT 1 | 4.3.B |
| unit_price | NUMERIC(15,2) NN | 4.3.B |
| discount_pct | NUMERIC(5,2) | 4.3.B |
| subtotal | NUMERIC(15,2) NN | 4.3.B · dihitung app, disimpan (snapshot harga saat kuotasi) |
| line_no | SMALLINT | urutan tampil |

Index: `(quote_id, line_no)`. RLS via `tenant_id` sendiri (bukan hanya lewat parent —
FORCE RLS butuh kolom di tabelnya).

**Keputusan:** `quote_items` menyimpan `unit_price`/`subtotal` sebagai **snapshot** —
harga plan bisa berubah di katalog, tapi kuotasi yang sudah dikirim tak boleh
ikut berubah. Beda dari `subscriptions.plan_id` yang single-lookup (bukan line-item).

---

## 5. Subscriptions · Modul 5 (`plans`, `subscriptions`)

### 5a. `plans` — katalog master (tenant-scoped)

| Kolom | Tipe | Sumber |
|---|---|---|
| id, tenant_id | | katalog milik workspace Desa+ |
| plan_name | TEXT NN | 5.3.A |
| plan_code | TEXT NN | 5.3.A · SKU |
| description | TEXT | 5.3.A |
| plan_category | TEXT NN | 5.3.A · Core/Add-on/Module (CHECK) |
| is_active | BOOLEAN NN DEFAULT true | 5.3.A |
| base_price | NUMERIC(15,2) | 5.3.B |
| billing_frequency | TEXT | 5.3.B · Monthly/Annual |
| setup_fee | NUMERIC(15,2) | 5.3.B |
| currency | TEXT NN DEFAULT 'IDR' | 5.3.B |
| included_features | TEXT | 5.3.B |
| created_by/at, updated_by/at | | audit (tanpa soft-delete: `is_active=false` = pensiun) |

Index: unique `(tenant_id, plan_code)`; `(tenant_id, is_active)`.

### 5b. `subscriptions` — inti + renewal(5.2) + churn(5.4) + renewal-action(6.6)

**Satu baris = satu periode langganan satu plan.** Multi-plan per desa = beberapa
baris paralel (spec memilih ini: `Plan` = lookup tunggal, add-on/module = plan
tersendiri). Renewal & churn = kolom yang menempel (spec 5.2/5.4: "bukan objek baru").

| Kolom | Tipe | Sumber |
|---|---|---|
| id, tenant_id | | |
| subscription_number | TEXT NN | 5.A · auto, unik per tenant |
| account_id | BIGINT NN → accounts | 5.A |
| plan_id | BIGINT NN → plans | 5.A · **tunggal** |
| subscription_owner | BIGINT → users | 5.A · CSM/AM |
| source_deal_id | BIGINT → deals | 5.A |
| **previous_subscription_id** | BIGINT → subscriptions | self-FK · rantai renewal (§ keputusan) |
| status | TEXT NN | 5.B · Trial/Active/Suspended/Expired/Cancelled/Churned (CHECK) |
| start_date | DATE | 5.B |
| end_date | DATE | 5.B |
| billing_cycle | TEXT | 5.B · Monthly/Quarterly/Annual/Multi-year |
| auto_renew | BOOLEAN NN DEFAULT false | 5.B |
| contract_term_months | INTEGER | 5.B |
| mrr | NUMERIC(15,2) | 5.C · Monthly Recurring Revenue |
| **arr** | NUMERIC(15,2) | 5.C · **disimpan** (annual bisa diskon → ARR ≠ MRR×12) |
| quantity_seats | INTEGER | 5.C |
| discount_pct | NUMERIC(5,2) | 5.C |
| payment_status | TEXT | 5.C · Paid/Pending/Overdue/Partial |
| *— Renewal (5.2) —* | | field menempel |
| renewal_status | TEXT | 5.2 · Upcoming/In Progress/Renewed/Not Renewed/At Risk |
| renewal_type | TEXT | 5.2 · Auto/Manual/Upsell/Downgrade |
| renewal_owner | BIGINT → users | 5.2 · staf CS |
| renewal_quote_id | BIGINT → quotes | 5.2 |
| previous_value | NUMERIC(15,2) | 5.2 · snapshot nilai periode lalu (deteksi upsell/downgrade tanpa self-JOIN) |
| *— Renewal action (6.6, CS) —* | | spec 6.6: DATA milik 5.2, AKSI milik CS |
| renewal_stage | TEXT | 6.6 · Not Started/Outreach/Negotiation/Won/Lost |
| renewal_risk | TEXT | 6.6 · Low/Medium/High |
| renewal_action_plan | TEXT | 6.6 |
| renewal_next_action_date | DATE | 6.6 |
| *— Churn (5.4) —* | | diisi saat status Cancelled/Churned |
| cancellation_date | DATE | 5.4 |
| churn_reason | TEXT | 5.4 · Budget/No Adoption/Change of Leadership/Competitor/Dissatisfaction/Feature Gap |
| churn_type | TEXT | 5.4 · Voluntary/Involuntary |
| churn_notes | TEXT | 5.4 |
| lost_value_mrr | NUMERIC(15,2) | 5.4 |
| win_back_eligible | BOOLEAN | 5.4 |
| deleted_at, audit | | |

**TIDAK jadi kolom** (Formula — dihitung saat query):
- `renewal_date` = `end_date` (5.2 def: "= End Date periode berjalan") → tak duplikat
- `days_to_renewal` = `end_date - CURRENT_DATE`

**Index:**
```sql
CREATE UNIQUE INDEX idx_subs_number ON subscriptions (tenant_id, subscription_number);
CREATE INDEX idx_subs_account ON subscriptions (tenant_id, account_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_subs_renewal ON subscriptions (tenant_id, end_date)
    WHERE status='Active' AND deleted_at IS NULL;   -- Menu 6.6 daftar jatuh tempo
CREATE INDEX idx_subs_prev ON subscriptions (previous_subscription_id)
    WHERE previous_subscription_id IS NOT NULL;
-- Satu chain plan hanya boleh SATU active pada satu waktu:
CREATE UNIQUE INDEX idx_subs_one_active ON subscriptions (tenant_id, account_id, plan_id)
    WHERE status='Active' AND deleted_at IS NULL;
```

**Keputusan (dikunci di diskusi):**
- **Renewal = INSERT baris baru**, bukan update. `previous_subscription_id` →
  baris lama; `previous_value` = snapshot nilai lama (biar deteksi upsell/downgrade
  tak perlu self-JOIN — persis field spec 5.2). `renewal_type` diturunkan dari
  perbandingan nilai baru vs `previous_value`.
- **ARR disimpan** terpisah dari MRR — billing Annual/Multi-year bisa diskon.
- **Beberapa langganan aktif paralel per desa** (Option B, pilihan spec).
  `idx_subs_one_active` menjaga satu active *per plan*, bukan per desa — desa
  boleh punya banyak active dengan plan berbeda.
- **ARR/renewal/churn per desa = agregasi** lintas baris (di view/report), bukan
  disimpan di `accounts`. Partial churn = nyata (berhenti satu plan, tetap plan lain).

---

## 6. Customer Success · Modul 6 (9 tabel)

### 6a. `customer_success` — 1:1 desa (health + lifecycle + onboarding + adoption)

Konsolidasi 6.1 + 6.2 + 6.2.1 + 6.4 — semua 1:1 dengan desa, selalu dilihat
bersama di satu tab CS. Satu baris per account.

| Kolom | Tipe | Sumber |
|---|---|---|
| id, tenant_id | | |
| account_id | BIGINT NN UNIQUE → accounts | 1:1 |
| *— Health (6.1) —* | | |
| overall_health_score | SMALLINT | 6.1 · 0–100 |
| health_status | TEXT | 6.1 · Healthy/At-Risk/Critical |
| adoption_score | SMALLINT | 6.1 · komponen |
| engagement_score | SMALLINT | 6.1 |
| support_score | SMALLINT | 6.1 |
| sentiment_score | SMALLINT | 6.1 |
| score_trend | TEXT | 6.1 · Improving/Stable/Declining |
| health_last_calculated | TIMESTAMPTZ | 6.1 |
| *— Lifecycle (6.2) —* | | |
| lifecycle_stage | TEXT | 6.2 · Onboarding/Adoption/Retention/Renewal/Advocacy |
| stage_entry_date | DATE | 6.2 |
| *— Onboarding (6.2.1) —* | | |
| onboarding_status | TEXT | 6.2.1 · Not Started/In Progress/Completed/Stalled |
| kickoff_date | DATE | 6.2.1 |
| target_go_live_date | DATE | 6.2.1 |
| actual_go_live_date | DATE | 6.2.1 |
| onboarding_progress | SMALLINT | 6.2.1 · % |
| *— Adoption/Usage (6.4) —* | | |
| last_login_date | DATE | 6.4 |
| active_users | INTEGER | 6.4 |
| login_frequency | TEXT | 6.4 · Daily/Weekly/Monthly/Rarely/Inactive |
| feature_adoption_rate | NUMERIC(5,2) | 6.4 · % |
| key_features_used | TEXT | 6.4 |
| usage_trend | TEXT | 6.4 · Increasing/Stable/Decreasing |
| usage_data_source | TEXT | 6.4 · Manual/Product Telemetry (v1: Manual) |
| audit (created/updated) | | tanpa soft-delete (ikut hidup account) |

`days_in_stage` (6.2 Formula) = `CURRENT_DATE - stage_entry_date`, dihitung.
`assigned_csm` (6.2) = **tidak di sini** — sudah di `accounts` (authoritative).

Index: unique `(tenant_id, account_id)`; `(tenant_id, health_status)`;
`(tenant_id, lifecycle_stage)`.

**Keputusan:** Health = 4 komponen **input manual/semi** (v1), overall dihitung app.
Mesin auto-calc dari telemetry ditunda (`usage_data_source` menandai asal).

### 6b. `cs_impl_tasks` — Implementation Tracker (6.2.1.1), 1:N

| id, tenant_id · account_id NN → accounts · task_name TEXT NN · task_status TEXT
(To Do/In Progress/Done/Blocked) · owner_id → users · due_date DATE · audit |

Index: `(tenant_id, account_id)`; `(tenant_id, task_status)`.

### 6c. `cs_trainings` — Training Schedule (6.2.1.2), 1:N

| id, tenant_id · account_id NN → accounts · training_topic TEXT · training_date
TIMESTAMPTZ · trainer_id → users · participants INTEGER · training_status TEXT
(Scheduled/Completed/Rescheduled/Cancelled) · attendance NUMERIC(5,2) · audit |

Index: `(tenant_id, account_id)`; `(tenant_id, training_date)`.

### 6d. `success_plans` — Success Plans (6.3), 1:N

| id, tenant_id · account_id NN → accounts · plan_name TEXT · objective TEXT ·
success_metric TEXT · target_date DATE · plan_status TEXT (Draft/Active/Achieved/
At-Risk/Cancelled) · progress SMALLINT · owner_csm → users · deleted_at · audit |

Index: `(tenant_id, account_id)`; `(tenant_id, plan_status)`.

### 6e. `surveys` — Voice of Customer / NPS/CSAT (6.8), 1:N

| id, tenant_id · account_id NN → accounts · survey_type TEXT (NPS/CSAT/CES/Custom) ·
respondent_contact_id → contacts · score NUMERIC(4,1) · category TEXT (Promoter/
Passive/Detractor) · feedback TEXT · survey_date DATE · follow_up_required BOOLEAN ·
audit |

Index: `(tenant_id, account_id)`; `(tenant_id, survey_type, survey_date)`.

### 6f. `tickets` — Tickets/Cases + SLA (6.9), 1:N

| Kolom | Tipe | Sumber |
|---|---|---|
| id, tenant_id | | |
| ticket_number | TEXT NN | 6.9.A · auto, unik per tenant |
| subject | TEXT NN | 6.9.A |
| account_id | BIGINT NN → accounts | 6.9.A |
| reporter_contact_id | BIGINT → contacts | 6.9.A |
| description | TEXT | 6.9.A |
| ticket_owner | BIGINT → users | 6.9.A · agen |
| type | TEXT | 6.9.B · Question/Problem/Bug/Feature Request/Complaint |
| category | TEXT | 6.9.B · Surat/Keuangan/Penduduk/Login/Lainnya |
| priority | TEXT | 6.9.B · Low/Medium/High/Urgent |
| channel | TEXT | 6.9.B · WhatsApp/Email/Phone/Portal/Web Form |
| product_module_id | BIGINT → plans | 6.9.B · modul bermasalah |
| status | TEXT NN | 6.9.C · New/Open/In Progress/Pending/Resolved/Closed/Reopened |
| resolution | TEXT | 6.9.C |
| resolution_code | TEXT | 6.9.C · Solved/Workaround/Duplicate/Not Reproducible/Won't Fix |
| closed_date | TIMESTAMPTZ | 6.9.C |
| reopened_count | INTEGER NN DEFAULT 0 | 6.9.C |
| sla_policy_id | BIGINT → sla_policies | 6.9.D |
| first_response_due | TIMESTAMPTZ | 6.9.D |
| first_response_at | TIMESTAMPTZ | 6.9.D |
| resolution_due | TIMESTAMPTZ | 6.9.D |
| sla_status | TEXT | 6.9.D · On Track/At Risk/Breached |
| csat_rating | SMALLINT | 6.9.E · 1–5 |
| linked_health_impact | BOOLEAN | 6.9.E |
| deleted_at, audit | | |

`time_to_resolution` (6.9.D Formula) = `closed_date - created_at`, dihitung.

Index: unique `(tenant_id, ticket_number)`; `(tenant_id, status, created_at DESC)`
partial live; `(tenant_id, account_id)`; `(tenant_id, ticket_owner)`;
`(tenant_id, sla_status)` untuk laporan SLA breach.

### 6g. `playbooks` — master (6.7)

| id, tenant_id · playbook_name TEXT · trigger_scenario TEXT (Health Drop/Low
Adoption/Renewal Approaching/New Onboarding) · description TEXT · steps TEXT ·
recommended_owner TEXT (CSM/Support/Sales) · is_active BOOLEAN · audit |

Index: `(tenant_id, is_active)`.

### 6h. `kb_articles` — Knowledge Base master (6.10)

| id, tenant_id · article_title TEXT · article_body TEXT · category TEXT · keywords
TEXT · status TEXT (Draft/Published/Archived) · visibility TEXT (Public/Internal/
Portal Only) · author_id → users · view_count INTEGER DEFAULT 0 · helpful_votes
INTEGER DEFAULT 0 · updated_at · audit |

Index: `(tenant_id, status)`; `(tenant_id, category)`. Attachment/rich media ditunda v1.

### 6i. `sla_policies` — master (6.11)

| id, tenant_id · sla_name TEXT · applies_to_priority TEXT · first_response_target_minutes
INTEGER · resolution_target_minutes INTEGER · business_hours TEXT · escalation_rule
TEXT · is_active BOOLEAN · audit |

Index: `(tenant_id, is_active)`. Target SLA satuan **menit** (bukan jam, sejak migrasi
`00016`) — prioritas Kritis butuh granularitas di bawah 1 jam (mis. 15 menit).

---

## 7. Activities · Modul 7 (`activities`, `activity_attendees`)

### 7a. `activities` — polymorphic, satu tabel untuk 6 sub-tipe

Spec 7 eksplisit: "Sales Activity (4.4) & Engagement (6.5) adalah **view terfilter**
dari objek di sini — bukan objek baru." Satu tabel + discriminator `kind`.
`related_to` = polymorphic (Village/Contact/Deal/Ticket/Subscription), pola
`audit_logs`.

| Kolom | Tipe | Catatan |
|---|---|---|
| id, tenant_id | | |
| kind | TEXT NN | `task`/`meeting`/`call`/`chat`/`email`/`note` (CHECK) |
| subject | TEXT NN | bersama semua kind |
| **target_type** | TEXT NN | polymorphic: `account`/`contact`/`deal`/`ticket`/`subscription` |
| **target_id** | BIGINT NN | BUKAN FK (tetap terbaca setelah target hard-delete, pola audit_logs) |
| owner_id | BIGINT → users | Assigned To / Owner |
| activity_context | TEXT | filter tampilan: `sales`/`cs`/`general` (memisah view 4.4 vs 6.5) |
| status | TEXT | Not Started/In Progress/Completed/Deferred/Planned/Held/Cancelled/No-Show |
| notes | TEXT | bersama (Description/Notes/Outcome) |
| *— Task (7.1) —* | | |
| due_date | DATE | |
| priority | TEXT | Low/Normal/High |
| reminder_at | TIMESTAMPTZ | |
| *— Meeting (7.2) —* | | |
| start_at | TIMESTAMPTZ | |
| end_at | TIMESTAMPTZ | |
| all_day | BOOLEAN | |
| location | TEXT | |
| meeting_type | TEXT | Demo/Training/QBR/Site Visit/Internal |
| *— Call/Chat (7.3/7.4) —* | | |
| contact_id | BIGINT → contacts | orang yang ditelepon/di-chat |
| direction | TEXT | Inbound/Outbound |
| activity_at | TIMESTAMPTZ | waktu call/chat/email |
| duration_min | INTEGER | |
| call_result | TEXT | Connected/No Answer/Busy/Voicemail/Follow-up/No Respond |
| *— Email (7.5) —* | | |
| email_from | TEXT | |
| email_to | TEXT | |
| email_status | TEXT | Sent/Delivered/Opened/Replied/Bounced |
| body | TEXT | isi email/note |
| *— Engagement/Check-in (6.5) —* | | field khas CS engagement |
| engagement_type | TEXT | Touch Point/QBR/Onboarding Call/Escalation/Check-in |
| frequency | TEXT | Weekly/Monthly/Quarterly/Ad-hoc |
| channel | TEXT | WhatsApp/Call/Video/Site Visit |
| scheduled_date | TIMESTAMPTZ | |
| next_due_date | DATE | |
| deleted_at, created_by/at, updated_by/at | | |

**Index:**
```sql
CREATE INDEX idx_activities_target  ON activities (tenant_id, target_type, target_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_activities_owner   ON activities (tenant_id, owner_id, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_activities_kind    ON activities (tenant_id, kind, created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX idx_activities_context ON activities (tenant_id, activity_context, created_at DESC) WHERE deleted_at IS NULL;
```

**Keputusan:** satu tabel (bukan 6) karena `related_to` polymorphic + timeline
gabungan ("semua aktivitas desa X") + view terfilter menuntut satu sumber.
`activity_context` memisah tampilan Sales (4.4) vs CS (6.5) vs umum. Kolom
tipe-spesifik nullable — kind menentukan mana yang relevan (validasi di handler).

### 7b. `activity_attendees` — junction (meeting saja, multi)

Attendees (7.2) = multi user & contact → tak bisa jadi kolom.

| id · activity_id NN → activities ON DELETE CASCADE · tenant_id NN (RLS) ·
attendee_type TEXT (`user`/`contact`) · attendee_id BIGINT · UNIQUE(activity_id,
attendee_type, attendee_id) |

Index: `(activity_id)`.

---

## 8. View & agregasi (Modul 1 Dashboard, Modul 8 Reports)

**Nol tabel baru.** Modul 8 spec: "Report bukan objek data." Semua laporan =
query agregasi via sqlc, sumbernya tabel di atas. Contoh pemetaan:

| Report | Query dari |
|---|---|
| Pipeline / Forecast / Win-Loss (8.1) | `deals` GROUP BY stage |
| MRR/ARR / Revenue by Plan (8.4) | `subscriptions` SUM per plan/period |
| Renewal / Churn Report (8.4) | `subscriptions` WHERE renewal_status / churn |
| Health / Adoption (8.2) | `customer_success` |
| NPS/CSAT (8.2) | `surveys` |
| Ticket Volume / SLA / Resolution (8.3) | `tickets` + `sla_policies` |
| Dashboard KPI (M1) | agregasi lintas tabel per role/ownership |

Boleh dibuat sebagai Postgres `VIEW` bila query dipakai ulang, atau query sqlc
langsung. Agregasi waktu pakai `AT TIME ZONE $tz`. `SUM/COUNT` bungkus
`COALESCE(...)::bigint` (bukan `pgtype.Numeric`) — gotcha #14 sqlc.

---

## 9. Field-Level Security (Modul 9, §5 dok role)

Penyamaran kolom per `business_role`, ditegakkan **di layer handler** (bukan DB) —
kolom asli tetap ada, handler memilih apa yang dioper ke view. Sama pola dengan
`maskEmail` yang menyamarkan di handler, bukan view.

| Field | Aturan | Tabel |
|---|---|---|
| MRR/ARR/amount | disembunyikan dari Support | subscriptions, deals |
| Nomor HP kontak | tersamar untuk non-Sales | contacts |
| Catatan internal | hanya Admin & CSM | activities (notes), success_plans |

Mengikat juga Manager kecuali ARR (§8.3). Super_admin lihat semua (audit lebih keras).

---

## 10. Yang DITUNDA eksplisit untuk v1 (dicatat, bukan dilupakan)

Bukan dead-end — task terbuka dengan alasan:

1. **Report Builder / Custom Reports (8.5)** — query builder dinamis melawan premis
   sqlc (SQL statis type-safe). Laporan standar 8.1–8.4 dikerjakan; builder = fase lanjut.
2. **Scheduled Reports & Export (8.5)** — email berkala butuh scheduler (pola ada
   di `internal/maintenance`); Export CSV relatif murah, bisa menyusul. Tak ubah skema.
3. **Attachment file** (email 7.5, KB 6.10) — butuh storage/upload, belum di stack.
4. **Auto-calc Health dari telemetry** (6.4) — v1 input manual; `usage_data_source` menandai.
5. **Customer Portal (6.12)** — kanal/konfigurasi eksternal (role EKSTERNAL perangkat
   desa), bukan objek data. `portal_access`/`portal_user_id` → kolom di `accounts` kelak.
6. **`parent_account_id` hierarki administratif** — kolom ada, pengisian pohon opsional.

---

## 11. Rencana migrasi (urutan FK)

Satu migrasi `00002_crm_schema.sql` (atau dipecah bila terlalu besar), urutan
mengikuti dependency FK:

```
1. ALTER memberships ADD business_role
2. plans                       (master, tanpa FK CRM)
3. accounts                    (self-FK parent — nullable, aman)
4. contacts                    (→ accounts, self-FK reports_to)
5. leads                       (FK converted_* nullable → accounts/contacts/deals)
6. deals                       (→ accounts, contacts, plans; created_subscription nullable)
7. quotes → quote_items        (→ deals, accounts, plans)
8. subscriptions               (→ accounts, plans, deals, quotes; self-FK previous)
9. sla_policies, playbooks, kb_articles   (master M6)
10. customer_success           (→ accounts 1:1)
11. cs_impl_tasks, cs_trainings, success_plans, surveys   (→ accounts)
12. tickets                    (→ accounts, contacts, plans, sla_policies)
13. activities → activity_attendees   (polymorphic, tanpa FK ke target)
14. RLS: ENABLE+FORCE+POLICY tenant_isolation untuk SEMUA tabel di atas
```

Beberapa FK melingkar (deals↔subscriptions, leads↔deals) diselesaikan dengan
kolom nullable + `ALTER TABLE ADD CONSTRAINT` di akhir bila perlu, atau
dibiarkan nullable tanpa deferrable (isi belakangan saat konversi/closed-won).

Setelah migrasi: `sqlc generate` → perbaiki ripple → `make check` hijau.
