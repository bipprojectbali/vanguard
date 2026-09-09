# 0011 — Quote otoritatif: nilai & termin langganan bersumber dari quote Accepted

Status: **Diterima & SELESAI PENUH** (2026-09-09) — dikerjakan **bertahap**. PR1
("quote otoritatif") SELESAI; PR2 ("langganan multi-baris") dipecah lagi jadi
**PR2a** (fondasi `subscription_items` + tulis-ganda, `plan_id` tetap NOT NULL —
SELESAI 9 Sep) dan **PR2b** (`plan_id` nullable + flip laporan + retire
single-plan — SELESAI 9 Sep; multi-plan kini aktif, BL-88 & BL-100 tertutup
penuh; belum merge/push). Men-supersede jalur nilai/termin di
[0004 §BL-21](0004-workspace-di-path-url.md)? tidak — melengkapi alur Closed Won
(`sales_deals_won_subscription.go`) yang lahir di BL-21.

## Konteks

Sebelum BL-88, **revenue yang diakui (MRR/ARR di laporan) hanya berasal dari
`deal.Amount` yang diketik manual.** Total quote (`quotes.grand_total`) &
`plans.base_price` tak pernah mengalir ke sana. Akibatnya user bisa meng-Accept
quote senilai X lalu mengisi `deal.Amount` = Y (≠ X) tanpa peringatan — laporan
mengikuti Y, bukan yang benar-benar ditawarkan & disetujui Desa.

Tiga celah struktural saat itu:

1. **Closed Won** (`subscriptionFromWonDeal`) menurunkan langganan dari
   `deal.Amount` + `deal.SubscriptionTerm` + `deal.PlanRequestedID` — **tak
   melihat quote sama sekali**.
2. **Tak ada penegakan "satu quote Accepted per deal"** — logika lama membiarkan
   "accept terakhir menang", jadi beberapa quote bisa berstatus Accepted
   bersamaan pada deal yang sama.
3. **Langganan hanya single-plan** (`subscriptions.plan_id NOT NULL`) — satu
   quote dengan banyak paket (`quote_items`) tak bisa jadi satu langganan
   ber-banyak-baris.

Diskusi 7 Sep (dari pertanyaan *"quote buat apa kalau langganan cuma dari
deal"*) menegaskan quote & langganan hidup di dua fase — quote = dokumen tawaran
pra-closing; langganan = pemenuhan pasca-Won — tapi nilai yang **ditawarkan**
tak mengalir ke yang **ditagih**. Keputusan penuh dikunci 9 Sep (tercatat di
`docs/crm/tasks.md` baris BL-88).

Ini **pergeseran arsitektur** (revenue manual→quote; langganan single→multi-
line), jadi dipecah dua PR agar tiap fase bisa direview & di-test terpisah.

## Keputusan (7 poin terkunci)

### 1. Nilai diakui = `quotes.grand_total`, disalin ke `deal.Amount` saat Accept

Saat quote di-Accept, `grand_total` quote disalin OTOMATIS ke `deal.Amount`
(`SetDealRecognizedValue`, ter-audit `deal.value.recognized`). Ini jalur UTAMA,
bukan fallback. Sejak Accept, `deal.Amount` = nilai DIAKUI (otoritatif).

### 2. Nilai manual deal dimaknai ulang jadi "nilai perkiraan"

Kolom `deal.Amount` TIDAK dihapus. Sebelum ada quote Accepted, ia = perkiraan
forecast pipeline pra-quote (diketik manual, pola `sales_convert.go` lead→deal).
Dua makna beda dalam satu kolom, dibedakan oleh KEBERADAAN quote Accepted:
detail deal melabeli "Nilai perkiraan" vs "Nilai diakui (dari quote)"
(precompute flag di handler via `GetAcceptedQuoteForDeal`; view murni-data).

### 3. Termin langganan PINDAH ke quote (kolom baru, bukan reuse `expiration_date`)

`quotes` dapat kolom baru `subscription_term` (Monthly/Annual/Multi-year) +
`contract_term_months`. **JANGAN pakai ulang `expiration_date`** — itu masa
berlaku quote, bukan tenor kontrak. `subscriptionFromWonDeal` membaca termin dari
quote Accepted, bukan `deal.SubscriptionTerm`. CHECK `quotes_term_chk` mirror
`deals_term_chk`.

### 4. `expected_close_date` TETAP di deal

Perkiraan tutup adalah atribut PIPELINE tahap awal (kapan deal diprediksi
closing), bukan atribut komersial quote. Tak dipindah — usul pindah ditolak
eksplisit.

### 5. Form deal: angkat termin, reframe nilai

Input `subscription_term` & preview MRR/ARR dihapus dari form deal (bergantung
termin yang kini milik quote; `dealpreview.js` dihapus). Money field direlabel
"Nilai per periode termin (Rp)" → **"Nilai perkiraan (Rp)"** + hint forecast.
`CreateDeal`/`UpdateDeal` berhenti mengeset `SubscriptionTerm` (kirim `nil`;
kolom tetap ada). Perkiraan tutup tetap. Termin ditambahkan ke form quote
(create & edit) → derive `contract_term_months` via peta `termContractMonths`.

### 6. Multi-paket per 1 quote → langganan MULTI-BARIS **(PR2 SAJA)**

Satu quote dengan banyak paket (`quote_items`) → satu langganan ber-banyak-baris
via tabel baru `subscription_items`; `subscriptions.plan_id` jadi NULLABLE
(identitas paket sepenuhnya di items). **Tidak dikerjakan di PR1** — lihat "PR2"
di bawah. PR1 mempertahankan jalur single-plan (`deal.PlanRequestedID` +
`backfillDealPlanFromQuote` + `singlePlanID`).

### 7. 1 deal = TEPAT 1 quote Accepted

Partial unique index `idx_quotes_one_accepted ON quotes (tenant_id, deal_id)
WHERE quote_status='Accepted' AND deleted_at IS NULL AND deal_id IS NOT NULL`
(jaring keras DB) + guard handler `GetAcceptedQuoteForDeal` sebelum set Accepted
(PRG `?err=quote_already_accepted`). Logika "accept terakhir menang" DIHAPUS.

## Pemisahan PR1 / PR2

**PR1 (sesi ini — quote otoritatif):** poin 1–5 & 7. Skema: migrasi
`00041_crm_quote_term_and_accept_unique.sql` (kolom termin quote + index
1-Accepted; idempotent). Alur Won: gerbang lama `plan_required`/`term_required`
diganti — tanpa quote Accepted → `quote_required`; `months` dari
`quote.ContractTermMonths` (fallback peta `termContractMonths`); value tetap
`deal.Amount` (= grand_total hasil Accept); `mrr = deal.Amount / months`. Plan
MASIH dari `deal.PlanRequestedID` (single-plan) — dipertahankan sengaja.

**PR2a (SELESAI 9 Sep — fondasi + tulis-ganda):** poin 6 sebagian. Migrasi
`00042_crm_subscription_items.sql`:

- **Tabel `subscription_items`** (mirror `quote_items`): `subscription_id`,
  `tenant_id` (RLS langsung, ENABLE+FORCE pola verbatim 00010/00012), `plan_id`,
  `quantity`, `unit_price`/`subtotal` snapshot, `mrr`/`arr` per-item, `line_no`;
  GRANT `app_rw`, index `(subscription_id, line_no)`.
- **Backfill idempotent** (`WHERE NOT EXISTS`): 1 item per langganan LAMA →
  agregasi via item bisa diandalkan penuh di PR2b.
- **Alur Won**: setelah `CreateSubscription`, salin `quote_items`→
  `subscription_items` (MRR/ARR per-item `subtotal/bulan`); **fail-soft** (item
  gagal tak menggagalkan Won — parent sudah lahir).
- **`subscriptions.plan_id` MASIH `NOT NULL` & terisi (TULIS GANDA)** → laporan
  & 8 INNER JOIN `plans` tak berubah, `make check` hijau. Gerbang Won `plan_required`
  (single-plan) DIPERTAHANKAN.
- Queries `AddSubscriptionItem`/`ListSubscriptionItems`; tes Won-buat-item +
  backfill idempotent.

**PR2b (SELESAI 9 Sep — enable multi-plan):** sisa poin 6. Migrasi
`00043_crm_subscription_multiplan.sql`. Multi-plan kini benar-benar aktif:

- **`subscriptions.plan_id` → NULLABLE** (`ALTER COLUMN … DROP NOT NULL`);
  identitas paket SEPENUHNYA di items (TANPA `primary_plan_id`).
  `Subscription.PlanID`/`CreateSubscriptionParams.PlanID` jadi `*int64` — pointer
  churn ~15 situs baca ditangani nil-safe.
- **Invarian 1-Active → pindah ke `subscription_items`**: `DROP INDEX
  idx_subs_one_active`; partial unique `idx_subscription_items_one_active
  (tenant_id, account_id, plan_id) WHERE parent_active AND plan_id IS NOT NULL`.
  Kolom turunan-trigger `subscription_items.account_id` + `parent_active`
  (trigger `sync_subscription_item_parent` BEFORE INSERT menurunkan dari parent;
  `resync_subscription_items_active` AFTER UPDATE status/deleted_at parent
  menyegarkan `parent_active`) — invarian melihat status parent tanpa join saat
  INSERT item. Konflik Won via `HasActiveItemForQuotePlans`.
- **Alur Won**: gerbang `plan_required` DIHAPUS; `singleQuotePlan(items)` →
  1 paket distinct → parent `plan_id` terisi (label cepat); >1 distinct → parent
  `plan_id` NULL + N item; 0 item ber-paket → tetap `plan_required` (menutup akar
  BL-100 — paket dibaca LANGSUNG dari `quote_items`, bukan `deal.plan_requested_id`).
- **Renewal**: `renewUpsell`/`renewStraight` kloning `subscription_items` dari
  `previous_subscription_id` ke baris baru (tx `h.q(ctx)` sama).
- **Laporan**: `ReportRevenueByPlan`/`ReportSubscriptionPlans` + `plan_filter`
  re-agregasi per-produk via `subscription_items` (bukan `s.plan_id`); 8 INNER
  JOIN `plans` → LEFT JOIN (plan bisa NULL → `PlanName *string`); list tampil
  `"N paket"` bila `item_count>1`.
- **UI**: kolom "Paket" `"N paket"` bila >1 + tabel item di detail langganan
  (`subscriptions_detail.go` + `ui/pages/panel/subscriptions_detail.go`,
  `TableScroll`).
- **Retire**: `sales_quotes_plan_backfill.go`(+test) DIHAPUS;
  `deal.PlanRequestedID`/`backfillDealPlanFromQuote`/`singlePlanID` disingkirkan
  (kolom `deals.plan_requested_id` dibiarkan usang, tak dibaca lagi).

## Konsekuensi

- **Satu sumber kebenaran komersial.** Nilai & termin yang diakui = yang
  di-Accept di quote; tak bisa lagi divergen dari yang diketik di deal.
- **Perkiraan tetap berguna.** Nilai manual pra-quote masih menopang forecast
  pipeline — tak dibuang, hanya dilabeli beda.
- **BL-100 tertutup penuh di PR2b** — Won membaca paket langsung dari
  `quote_items`; multi-paket (>1 distinct) didukung penuh (parent `plan_id` NULL
  + N item), tak lagi di-skip seperti Fix B lama.
- **`deal.PlanRequestedID` di-retire di PR2b** (kolom `deals.plan_requested_id`
  dibiarkan usang, tak dibaca lagi). `deal.SubscriptionTerm` tetap warisan
  transisi (kolom ada, tak diset — termin milik quote sejak PR1).

## Di luar cakupan (ditunda / ditolak eksplisit)

- Pindah `expected_close_date` ke quote — **ditolak** (poin 4).
- Reuse `expiration_date` sebagai tenor kontrak — **ditolak** (poin 3).
- Multi-baris `subscription_items` & `plan_id` NULLABLE — **SELESAI di PR2a+PR2b**
  (poin 6), bukan lagi di luar cakupan.
