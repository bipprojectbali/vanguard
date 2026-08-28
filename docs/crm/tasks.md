# Task CRM Desa+ — 10 Agustus → 1 September 2026

> **Status:** artefak siap-import. MCP `pm-dashboard` belum tersambung di sesi ini
> (hanya `penpot` di `.mcp.json`). Begitu pm-dashboard connect, tiap baris di bawah
> jadi satu task/card.
>
> **Cakupan:** seluruh 9 modul CRM. Breakdown **per-layer** di dalam modul.
>
> **⚠️ Catatan realistis:** 9 modul dalam 17 hari kerja itu sangat padat. Tanggal di
> bawah JUJUR (tidak dipadatkan agar "muat"). Titik luber ada di **Modul 6 (Customer
> Success, 9 tabel — 8/9 selesai, sisa `surveys`)**
> dan **Modul 9 (FLS, audit menyeluruh belum dikerjakan sebagai pass tersendiri)** —
> lihat penanda 🔴. Ini fitur, bukan bug: lebih baik lihat luber di rencana daripada
> di hari-H. **Modul 1 (Dashboard), 7 (Activities), dan 8 (Reports) sudah selesai**
> (17–18 Agu) — ketiganya dikerjakan lebih awal & di luar urutan rencana, karena tak
> benar-benar butuh Modul 6 tuntas (cukup tabel yang sudah ada saat itu).

## Konvensi & aturan wajib (berlaku di SEMUA task)

Diambil dari `CLAUDE.md` — dipasang di sini supaya tiap task mewarisi definition-of-done yang sama.

- **Branch:** semua kerja di `feature/crm-*`, jangan commit ke `main`. Tunggu konfirmasi user sebelum merge.
- **Gate:** `make check` (sqlc · vet · gofmt · build · test) **harus hijau** sebelum task ditandai selesai.
- **Schema:** ubah schema → tulis migrasi goose (`IF NOT EXISTS`) → jalankan lokal → `sqlc generate` → perbaiki ripple. **Jangan edit `internal/db/*` (generated).**
- **Migrasi baru inkremental** mulai `00002_*` — jangan sunting `00001`.
- **Query wajib `h.q(ctx)`** (bukan `h.DB`), filter `table_schema` pakai `current_schema()`.
- **Isolasi desa (inter-village) = filter query app-layer**, BUKAN RLS lapis kedua. RLS hanya isolasi workspace.
- **Test wajib** di pekerjaan yang sama; pakai `TEST_DATABASE_URL` (Postgres, jangan tukar engine). Modul yang bergantung mode wajib diuji di single & multi.
- **UI mobile-first:** verifikasi 3 lebar (375/768/1280) + screenshot sebelum selesai; bungkus tiap `<table>` dengan `ui.TableScroll`; helper `dsx.go` (`ClassOn`/`FormPostSelect`), bukan `data.Class`/`@post` mentah.
- **Test/verifikasi UI mencakup SUBMIT**, bukan cuma render (form action pernah 404 senyap).
- **Estimasi** dalam hari-kerja (d). **Depends** = ID task prasyarat.

---

## FASE 0 — Fondasi (shared, sekali di awal)

| ID | Task | Layer | Est | Depends | Rencana |
|----|------|-------|-----|---------|---------|
| F0 | Buka branch `feature/crm-foundation` + kerangka migrasi `00002_crm_schema.sql` | infra | 0.5d | — | 10 Agu |
| F1 | Sumbu role bisnis: `ALTER memberships ADD business_role` (+CHECK) + `sqlc generate` | migrasi | 0.5d | F0 | 10 Agu |
| F2 | Objek Casbin business-role (admin/manager/sales/csm/support) + policy CSV (komentar di baris `#` tersendiri) | authz | 0.5d | F1 | 11 Agu |
| F3 | Helper ownership-filter app-layer (`WHERE account_owner=$uid OR assigned_csm=$uid …`) + test unit | backend | 0.5d | F1 | 11 Agu |
| F4 | Helper Field-Level Security masking di handler (pola `maskEmail`) + test | backend | 0.5d | F1 | 11 Agu |

**Detail F0–F4**
- **F0** — branch dari `main`. Migrasi `00002` memuat SEMUA tabel CRM 19 (satu file, urut FK per §11 skema) ATAU dipecah per modul; keputusan: **satu migrasi fondasi** untuk tabel inti (accounts, contacts) + tabel master tanpa FK maju (plans, sla_policies), sisanya menyusul per modul. GRANT `app_rw` ke `current_schema()`, `FORCE RLS`, policy `tenant_isolation` verbatim per §0. `GRANT app_rw TO CURRENT_USER`.
- **F1** — `business_role TEXT NULL` + `CHECK (business_role IS NULL OR business_role IN ('admin','manager','sales','csm','support'))`. Nullable = sengaja (member tanpa peran bisnis).
- **F2** — objek `sales:*`, `csm:*`, dst. Deny-default. Uji: sales tak bisa akses objek ticket-only support, dsb.
- **F3** — satu tempat merakit klausa kepemilikan per business_role (sales=assigned villages, csm=managed, support=all-tickets-only, manager=cross-team FLS, admin=all). Dipakai SEMUA list-query modul. **Acceptance:** query tanpa filter ini = test gagal.
- **F4** — masking ARR/field sensitif di handler sebelum ke view (view murni-data). **Acceptance:** field masked tak pernah dioper ke view.

---

## MODUL 2 — Accounts (Desa / HUB)

Hub semua relasi. `account_owner`/`assigned_csm`/`backup_csm` otoritatif; `village_code` unique partial index; region sebagai teks; geo NUMERIC(10,7); `parent_account_id` self-FK (opsional v1).

| ID | Task | Layer | Est | Depends | Rencana |
|----|------|-------|-----|---------|---------|
| M2-1 | Migrasi `accounts` (di 00002) + index (village_code unique partial, owner, tenant) + `sqlc generate` | migrasi | 0.5d | F0 | 11 Agu |
| M2-2 | Query sqlc: create/get/list(keyset)/update/soft-delete/assign-CSM + ownership-filter | repo | 1d | M2-1,F3 | 12 Agu |
| M2-3 | Handler + route `/w/{slug}/accounts` (list, detail, new, edit, assign) — `canManageMembers`-style gate | handler | 1d | M2-2 | 12–13 Agu |
| M2-4 | View: daftar desa (TableScroll, keyset pagination `?after=`), form desa (mobile-first), detail hub | view | 1d | M2-3 | 13 Agu |
| M2-5 | Test: RLS isolasi workspace + ownership-filter antar-desa + handler CRUD + keyset | test | 1d | M2-3 | 14 Agu |
| M2-6 | Verifikasi 3-lebar (375/768/1280) + screenshot; `make check` hijau | qa | 0.5d | M2-4,M2-5 | 14 Agu |
| M2-7 | Detail: surfacekan kolom identitas yang sudah ada di skema sejak awal — Pemilik Akun, Induk Akun (best-effort, terhapus/gagal → kosong), Lintang/Bujur | view+handler | 0.5d | M2-4 | ✅ 26 Agu |
| M2-8 | Detail: kartu ringkasan lintas-modul — Langganan (MRR/ARR ter-mask F4 spt VillageBudget), Customer Success (health/lifecycle, tanpa mask), Sistem (audit) | view+handler | 1d | M2-4,M5,M6 | ✅ 26 Agu |
| M2-9 | Detail: baris "Terkait" — 4 chip ringkasan (Kontak, Deal, Langganan, Tiket); Deal/Tiket sengaja tanpa href (belum ada rute daftar ber-filter desa) | view+handler | 0.5d | M2-8 | ✅ 26 Agu |

**Acceptance modul:** owner/CSM hanya lihat desa miliknya (kecuali admin); duplikat `village_code` ditolak; halaman desa nol overflow di 375px; submit form assign-CSM benar-benar menulis (bukan 404).

**Catatan M2-7..M2-9** (`feature/crm-accounts-detail-rollup`, di luar urutan awal — rebuild halaman detail sesuai wireframe Penpot "Accounts — 2.3 Account Detail"): **satu layout untuk semua business_role**, bukan 4 halaman terpisah per POV — F2 (Casbin)/F3 (ownership)/F4 (masking) di handler yang memutuskan visibilitas per role, konsisten dgn pola modul lain. Tak perlu migrasi baru (kolom `account_owner`/`parent_account_id`/`latitude`/`longitude` sudah ada sejak M2-1, sebelumnya tak pernah disurfacekan). Dua kotak anotasi "BUG DESAIN" di wireframe adalah artefak review Penpot saja (rujukan dokumen non-repo, tak ada cacat nyata yg cocok) — sengaja TIDAK ikut ke UI nyata.

---

## MODUL 3 — Contacts (perangkat desa)

1:N ke accounts. `email_opt_out`/`do_not_contact` writable. `reports_to_id` self-FK. `is_primary` per desa.

| ID | Task | Layer | Est | Depends | Rencana |
|----|------|-------|-----|---------|---------|
| M3-1 | Migrasi `contacts` (di 00002) + index (account_id, owner) + `sqlc generate` | migrasi | 0.5d | M2-1 | 17 Agu |
| M3-2 | Query sqlc: CRUD + list-by-account + set-primary + ownership-filter (ikut desa induk) | repo | 0.5d | M3-1,F3 | 17 Agu |
| M3-3 | Handler + route `/w/{slug}/accounts/{id}/contacts` (nested) + `/contacts` global list | handler | 1d | M3-2 | 17–18 Agu |
| M3-4 | View: daftar kontak per desa (TableScroll), form kontak, badge primary/opt-out | view | 1d | M3-3 | 18 Agu |
| M3-5 | Test: kontak ikut isolasi desa induk + primary-uniqueness + opt-out flag | test | 0.5d | M3-3 | 19 Agu |
| M3-6 | Verifikasi 3-lebar + screenshot; `make check` hijau | qa | 0.5d | M3-4,M3-5 | 19 Agu |

**Acceptance modul:** kontak mewarisi kepemilikan desa (bukan filter sendiri); tepat 1 primary per desa; opt-out tampil jelas. **← Titik keputusan build entry point (2+3) selesai di sini.**

---

## MODUL 4 — Sales (leads · deals · quotes · quote_items)

Konversi Salesforce-style: leads flat (region teks mentah) → **1 TX atomik** buat accounts+contacts+deals, set `converted_*`. Quote punya snapshot harga.

| ID | Task | Layer | Est | Depends | Rencana |
|----|------|-------|-----|---------|---------|
| M4-1 | Migrasi `leads`,`deals`,`quotes`,`quote_items` (migrasi 00003) + index + `sqlc generate` | migrasi | 1d | M3-1 | 19–20 Agu |
| M4-2 | Query: leads CRUD + duplicate-check (village_code/name+region) + deals pipeline + quotes/items | repo | 1d | M4-1 | 20 Agu |
| M4-3 | **Konversi lead → 1 TX atomik** (INSERT accounts+contacts+deals, UPDATE leads.converted_*, audit) | backend | 1d | M4-2,M2-2,M3-2 | 20–21 Agu |
| M4-4 | Handler + route: `/leads`, `/deals` (kanban stage), `/deals/{id}/quotes` | handler | 1d | M4-3 | 21 Agu |
| M4-5 | View: daftar lead, tombol convert (guard duplikat), papan deal per-stage, quote+items | view | 1d | M4-4 | 21–24 Agu |
| M4-6 | Test: konversi atomik (all-or-nothing) + duplicate-check collision + snapshot harga quote | test | 1d | M4-3 | 24 Agu |
| M4-7 | Verifikasi 3-lebar + screenshot; `make check` hijau | qa | 0.5d | M4-5,M4-6 | 24 Agu |

**Acceptance modul:** konversi gagal-sebagian = rollback penuh; duplikat desa muncul di konversi (bukan bikin desa ganda); harga quote beku walau plan berubah.

---

## MODUL 5 — Subscriptions (plans · subscriptions)

Renewal = INSERT baris baru (`previous_subscription_id` self-FK + `previous_value` snapshot), bukan update. Satu aktif per plan-chain; ARR tersimpan; churn + renewal-action columns.

| ID | Task | Layer | Est | Depends | Rencana |
|----|------|-------|-----|---------|---------|
| M5-1 | Migrasi `plans`(master),`subscriptions` (00004) + `idx_subs_one_active` partial + `sqlc generate` | migrasi | 0.5d | M4-1 | 25 Agu |
| M5-2 | Query: plans master CRUD + subscription create-from-deal + renewal-INSERT + churn-update | repo | 1d | M5-1 | 25 Agu |
| M5-3 | Handler + route: `/plans`, `/subscriptions`, aksi renew/churn (approval upsell → Manager) | handler | 1d | M5-2 | 25–26 Agu |
| M5-4 | View: katalog plan, daftar langganan (status/MRR/ARR — ARR di-mask utk non-manager via F4), riwayat renewal (chain) | view | 1d | M5-3 | 26 Agu |
| M5-5 | Test: renewal bikin baris baru (lama Expired) + one-active-per-chain + churn + ARR FLS | test | 1d | M5-3 | 27 Agu |
| M5-6 | Verifikasi 3-lebar + screenshot; `make check` hijau | qa | 0.5d | M5-4,M5-5 | 27 Agu |

**Acceptance modul:** renew tak menimpa baris lama; tepat 1 subscription aktif per chain; ARR tak bocor ke role tanpa hak.

---

## 🔴 MODUL 6 — Customer Success (9 tabel) — TITIK LUBER

`customer_success` (1:1) + `cs_impl_tasks`, `cs_trainings`, `success_plans`, `surveys`, `tickets`(+SLA), `playbooks`, `kb_articles`, `sla_policies`. Terlalu besar untuk 1 modul di sisa waktu — **kandidat kuat geser lewat 1 Sep.**

| ID | Task | Layer | Est | Depends | Rencana |
|----|------|-------|-----|---------|---------|
| M6-1 | Migrasi 9 tabel CS (00005) + index + `sqlc generate` | migrasi | 1d | M5-1 | ✅ 28 Agu |
| M6-2 | Query: `customer_success` 1:1 (health/lifecycle/onboarding/adoption) + tickets+SLA | repo | 1d | M6-1 | ✅ 28 Agu |
| M6-3 | Query: cs_impl_tasks, cs_trainings, success_plans, surveys (1:N children) | repo | 1d | M6-1 | ✅ cs_impl_tasks/cs_trainings/success_plans selesai; `surveys` belum |
| M6-4 | Handler + route: `/accounts/{id}/success` (health), `/tickets` (support-only gate), master playbooks/KB/SLA | handler | 1.5d | M6-2,M6-3 | ✅ (`surveys` di luar cakupan) |
| M6-5 | View: panel health desa, daftar tiket (TableScroll+SLA), onboarding tracker, survey | view | 1.5d | M6-4 | ✅ onboarding tracker (Implementation Tracker + Training Schedule) selesai; survey belum |
| M6-6 | Test: 1:1 constraint + ticket ownership (support = all-village tapi ticket-only) + SLA breach | test | 1d | M6-4 | ✅ |
| M6-7 | Verifikasi 3-lebar + `make check` | qa | 0.5d | M6-5,M6-6 | ✅ |

**Status terkini:** 8/9 tabel selesai — `customer_success`, `success_plans`, `tickets`+SLA, `playbooks`, `kb_articles`, `sla_policies`, **`cs_impl_tasks`** (Implementation Tracker, branch `feature/crm-cs-onboarding-tasks-trainings`), **`cs_trainings`** (Training Schedule, branch sama). Sisa **`surveys`** — belum dikerjakan, di luar cakupan sesi terakhir.

**Acceptance modul:** health per desa 1:1; support lihat semua desa tapi hanya objek tiket; SLA breach terdeteksi.

---

## MODUL 7 — Activities (polymorphic)

`activities` (target_type + target_id, target_id BUKAN FK, bertahan setelah hard-delete) + `activity_attendees` junction. 6 sub-tipe via `kind` enum.

| ID | Task | Layer | Est | Depends | Rencana |
|----|------|-------|-----|---------|---------|
| M7-1 | Migrasi `activities`,`activity_attendees` (00011) + index (target_type,target_id) + `sqlc generate` | migrasi | 0.5d | M6-1 | ✅ 17 Agu |
| M7-2 | Query: log activity polymorphic + list-by-target + attendees (meeting) | repo | 1d | M7-1 | ✅ 17 Agu |
| M7-3 | Handler + view: timeline aktivitas di detail desa/deal/tiket (reusable component) + halaman lintas-context `/activity-log` | handler+view | 1d | M7-2 | ✅ 17 Agu |
| M7-4 | Test: polymorphic target bertahan setelah target hard-delete + attendees | test | 0.5d | M7-2 | ✅ 17 Agu |

**Acceptance modul:** timeline muncul di semua entity; baris activity tetap terbaca walau target dihapus. Branch `feature/crm-activities-timeline` (M7-A) → `feature/crm-activities-part-b` (M7-B, target filter + pre-fill) → `feature/crm-activities-all` (halaman lintas-context), semuanya sudah merge ke `main`.

---

## MODUL 1 — Dashboard (view & agregasi)

Rollup dihitung di query (tak disimpan). Timezone: simpan UTC, agregasi `AT TIME ZONE`.

| ID | Task | Layer | Est | Depends | Rencana |
|----|------|-------|-----|---------|---------|
| M1-1 | Query agregasi: ARR total, pipeline per-stage, health distribution, renewal due (COALESCE::bigint) | repo | 1d | M5-2,M6-2 | ✅ 17 Agu |
| M1-2 | Handler + view dashboard (kartu KPI + chart ECharts vendored, CSP-safe via `<script type=json>`) | handler+view | 1.5d | M1-1 | ✅ 17 Agu |
| M1-3 | Test agregasi + FLS (manager cross-team, ARR terbatas) + verifikasi 3-lebar | test+qa | 1d | M1-2 | ✅ 17 Agu |

**Catatan:** sama seperti Modul 8, dikerjakan di luar urutan rencana awal — cukup
memakai `customer_success`/`tickets` yang sudah ada saat itu (M6 baru sebagian),
bukan menunggu Modul 6 tuntas. Branch `feature/crm-dashboard`, sudah merge ke `main`.

---

## MODUL 8 — Reports

Report Builder & Scheduled Export (8.5) **ditunda v1** (§10 skema) — item nav-nya
DIHAPUS dari sidebar (bukan disabled). Preset read-only 8.1–8.4 selesai lebih awal
dari rencana (bukan menunggu M1-1/dashboard — pipeline/churn/renewal-forecast tak
bergantung agregasi dashboard).

| ID | Task | Layer | Est | Depends | Rencana |
|----|------|-------|-----|---------|---------|
| M8-1 | Preset report read-only: Sales (8.1, pipeline/win-loss), Subscription (8.4, renewal-forecast/churn), Customer Success (8.2, Health/Adoption — NPS/CSAT ditunda, `surveys` belum ada), Support (8.3, ticket volume/SLA/resolution) — query + view tabel + export-CSV manual | repo+view | 1.5d | F2 | ✅ 17–18 Agu |
| M8-2 | Test preset (F2 gate + F3 ownership + agregasi) + verifikasi 3-lebar; 8.5 (Custom Reports) dihapus dari nav, bukan placeholder | test+qa | 0.5d | M8-1 | ✅ 18 Agu |

**Catatan:** dikerjakan di luar urutan dependensi rencana awal (F2 langsung, tak
menunggu M1-1/M5-2/M6-2 seperti draf semula) karena tiap preset menyaring langsung
tabel sumbernya (`deals`, `subscriptions`, `customer_success`, `tickets`) via F3
ownership filter yang sudah ada, bukan lewat agregasi dashboard M1. Branch
`feature/crm-reports-m8` (8.1+8.4) lalu `feature/crm-reports-cs-support` (8.2+8.3
+ hapus nav 8.5), keduanya sudah merge ke `main`.

---

## MODUL 9 — Field-Level Security (pass menyeluruh)

FLS sudah dianyam di F4 + tiap handler. Task ini = audit menyeluruh + test lintas-role.

| ID | Task | Layer | Est | Depends | Rencana |
|----|------|-------|-----|---------|---------|
| M9-1 | Audit tiap view: pastikan field sensitif di-mask di HANDLER (bukan view), lintas 5 business_role | audit | 1d | semua modul | ✅ 18 Agu |
| M9-2 | Test matriks FLS: tiap role × tiap field sensitif (ARR, budget, PII kontak) | test | 1d | M9-1 | ✅ 18 Agu |

---

## Ringkasan timeline (jujur)

| Periode | Modul | Status realistis |
|---------|-------|------------------|
| 10–11 Agu | Fondasi (F0–F4) | ✅ muat |
| 11–14 Agu | **Modul 2 Accounts** | ✅ muat |
| 17–19 Agu | **Modul 3 Contacts** | ✅ muat (build entry point 2+3 selesai) |
| 19–24 Agu | **Modul 4 Sales** | ✅ muat (mepet) |
| 25–27 Agu | **Modul 5 Subscriptions** | ✅ muat (mepet) |
| 28 Agu–1 Sep | **Modul 6 Customer Success** | 🟡 sebagian (8/9 tabel); sisa `surveys` |
| 17 Agu | **Modul 7 Activities** | ✅ selesai lebih awal, di luar urutan (tak menunggu Modul 6 tuntas) |
| 17 Agu | **Modul 1 Dashboard** | ✅ selesai lebih awal, di luar urutan (cukup tabel M6 yang sudah ada saat itu) |
| 17–18 Agu | **Modul 8 Reports** | ✅ selesai lebih awal, di luar urutan (langsung dari F2, tak menunggu M1/M6 tuntas) |
| 18 Agu | **Modul 9 M9-1 (Audit FLS)** | ✅ selesai lebih awal, di luar urutan (1 gap live + 3 gap laten diperbaiki) |
| 18 Agu | **Modul 9 M9-2 (Test matriks FLS)** | ✅ selesai lebih awal, di luar urutan (4 gap cakupan matriks ditambal: ARR subscription, admin di ARR subscription, matriks ARR reports jadi 5-role, Deals Amount [modul tanpa test sebelumnya], PII phone Support di accounts/contacts) |
| 26 Agu | **Modul 2 M2-7..M2-9 (rollup detail Account)** | ✅ selesai lebih awal, di luar urutan (rebuild halaman detail sesuai wireframe Penpot; satu layout semua role, F2/F3/F4 di handler; tak perlu migrasi baru) |
| >1 Sep | sisa Modul 6 | 🔴 luber ke September |

**Rekomendasi:** kunci komitmen 1 Sep pada **Modul 2–5, 1, 7, 8, 9 tuntas + Modul 6 sebagian**. Kalau kamu mau Modul 6 benar-benar tuntas sebelum 1 Sep, satu-satunya tuas jujur adalah mengurangi kedalaman (mis. `customer_success`+`tickets` cukup, `cs_impl_tasks`/`cs_trainings`/`surveys` jadi v1.1) — bukan menambah jam.
