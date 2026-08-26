# 0009 — Master wilayah administratif (Provinsi/Kabupaten-Kota/Kecamatan)

Status: **Diterima** (2026-08-26) — membalik sebagian keputusan region di
`docs/crm/skema.md` §2 (`accounts`) dan §4a (`leads`)

## Konteks

Sejak migrasi fondasi CRM (00005), `province`/`regency`/`district` di
`accounts` dan `leads` sengaja dibuat kolom **TEXT bebas**, dengan alasan
tertulis: `accounts` cuma menampung desa *pipeline* (bukan ~83.000 desa
Indonesia), dan Territory View cukup `GROUP BY` teks.

Keputusan itu berbiaya nyata setelah data bertambah:

1. **Typo/variasi ejaan memecah dedup & agregasi.** `GROUP BY regency` melihat
   `"Kab. Bandung"`, `"Kabupaten Bandung"`, dan `"bandung"` sebagai tiga
   wilayah berbeda. `FindDuplicateAccountsByNameRegion` (deteksi desa duplikat
   saat konversi Lead) terpaksa fuzzy-match (`lower(trim(regency))`) untuk
   menutupi ini — tetap rapuh terhadap variasi yang tak tertangkap normalisasi
   sederhana.
2. **Tak ada UX dropdown berjenjang** — staf mengetik bebas, rawan salah
   ketik, tak ada validasi "kecamatan ini benar-benar bagian dari kabupaten
   ini".
3. **Tak ada kode resmi** untuk pelaporan/integrasi ke sistem lain (mis. data
   Kemendagri/BPS yang memakai kode wilayah baku).

## Keputusan

### 1. Master table GLOBAL, bukan tenant-scoped

`regions` **tanpa** `tenant_id`, **tanpa** RLS — data wilayah sama untuk semua
tenant, sama pola dengan `platform_staff` (bukan `plans`, yang ternyata
tenant-scoped dan bukan analog yang tepat untuk data referensi global).
Konsekuensi: semua query di `queries/regions.sql` aman dipanggil lewat
`h.q(ctx)` (tx ber-tenant, RLS tak menyaring karena tabel ini tak diproteksi
RLS) **maupun** `db.WithSuper` — keduanya membaca baris yang sama.

### 2. Satu tabel self-referencing, 3 level

Bukan tiga tabel terpisah (`provinces`/`regencies`/`districts`). Mencerminkan
struktur data sumber (flat, kode berjenjang) dan pola self-FK yang sudah ada
di repo (`parent_account_id`, `reports_to_id`).

```sql
CREATE TABLE regions (
    id               BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    parent_region_id BIGINT REFERENCES regions(id) ON DELETE CASCADE,
    level            SMALLINT NOT NULL CHECK (level IN (1, 2, 3)),
    code             TEXT NOT NULL UNIQUE,
    name             TEXT NOT NULL
);
```

`ON DELETE CASCADE` pada self-FK (bukan `RESTRICT`) — data referensi jarang
dihapus; cascade menghindari cleanup manual berjenjang bila suatu saat perlu
dibongkar-pasang.

**Cakupan sengaja dibatasi 3 level** (Provinsi → Kabupaten/Kota → Kecamatan).
Level Desa/Kelurahan (~83.000 baris) **tetap teks bebas** milik akun itu
sendiri (`village_name`) — desa yang dilacak CRM adalah entitas *pipeline*,
bukan seluruh desa Indonesia; memaksakannya jadi master row akan membengkak
tanpa manfaat (lihat "Di luar cakupan").

### 3. Sumber data: cahyadsn/wilayah (MIT License)

Dataset GitHub `cahyadsn/wilayah`, bersumber dari Kemendagri/BIG resmi
mengikuti Kepmendagri No 300.2.2-2430 Tahun 2025. File `db/wilayah.sql` = satu
tabel flat `(kode, nama)` dengan kode berjenjang titik (`"11"` provinsi,
`"11.01"` kab/kota, `"11.01.01"` kecamatan) — difilter ke 3 level pertama saja
(buang level kelurahan/desa 4-digit). Total ter-seed: 34 provinsi + ~514
kab/kota + ~7.285 kecamatan (±7.833 baris).

Dipilih di atas mengetik manual/API pihak ketiga karena: aktif di-maintain,
lisensi MIT aman untuk proyek proprietary, dan format flat kode-berjenjang
cocok langsung dengan skema self-referencing di atas.

### 4. Seed via SQL migration murni

Repo ini tidak punya wiring untuk goose Go-migration (`.go` + `go:embed`
CSV) — semua migrasi `.sql`, `main.go` cuma meng-embed `migrations/*.sql`.
Alih-alih menambah jalur migrasi baru untuk satu kasus, data di-seed lewat 3
blok `INSERT` berurutan dalam migrasi SQL yang sama
(`00026_crm_regions.sql`):

1. Level 1 (provinsi): `INSERT` polos.
2. Level 2 (kab/kota): `INSERT ... SELECT r.id, 2, v.code, v.name FROM
   (VALUES (...)) AS v(code,name,parent_code) JOIN regions r ON
   r.code = v.parent_code` — resolve `parent_region_id` tanpa perlu tahu ID
   auto-increment level 1 sebelumnya.
3. Level 3 (kecamatan): pola sama, join ke level 2.

Baris besar (~7.800 baris `VALUES`) sah sesuai pengecualian file-health untuk
file migration (Rule 8, CLAUDE.md).

### 5. Kolom teks lama di-RENAME `*_legacy`, bukan di-drop

`accounts.province/regency/district` → `province_legacy/regency_legacy/
district_legacy`; `leads.province/regency/district` → pola sama. Nullable,
tak lagi ditulis kode baru. FK baru (`district_id`) pada baris lama otomatis
`NULL` — **tidak** ada fuzzy-match/auto-backfill dari teks lama; staf memilih
ulang manual dari dropdown baru.

Alasan rename ketimbang drop: drop kolom = hilang permanen (hard-to-reverse
butuh konfirmasi eksplisit, lihat prinsip di CLAUDE.md §"Delivering work" soal
tindakan tak-mudah-dibalik); rename+biarkan = reversibel, gampang di-drop di
migrasi susulan setelah semua akun/lead lama sudah di-assign ulang, dan nilai
lamanya tetap terlihat sebagai jejak bantu saat staf memilih ulang.

### 6. Cascading dropdown 100% client-side

Dataset penuh (~7.833 baris `{id, parent_region_id, level, name}`) di-embed
sekali sebagai `<script type="application/json">` (pola sama ECharts, gotcha
#12 CLAUDE.md — CSP-safe, bukan inline eval), dibaca `static/regions.js`
(vanilla JS, same-origin, lolos CSP `script-src 'self'`) yang mem-populate 3
`<select>` berjenjang murni di browser. Hanya `district_id` (kecamatan) yang
benar-benar ter-submit sebagai field form (`name="district_id"`) — Provinsi
dan Kabupaten/Kota di atasnya cuma alat bantu filter, tanpa `name` — satu
sumber kebenaran.

Dipilih di atas endpoint `@get`/fragment-swap Datastar baru karena: dataset
kecil (~300KB), menghindari infra baru + latensi round-trip per pilihan, dan
`data-*` Datastar sengaja dibatasi ke ekspresi ID+literal saja (lihat
"Batasan" CLAUDE.md soal `unsafe-eval`) — bukan tempat menaruh logic filter
berjenjang.

### 7. Dedup Lead→Account jadi exact match

`FindDuplicateAccountsByNameRegion` yang sebelumnya membandingkan `regency`
via `lower(trim())` (fuzzy teks, untuk menutupi variasi ejaan) sekarang exact
match `district_id = $2` (atau `$2::bigint IS NULL` bila kosong). FK
menghapus kebutuhan fuzzy compare sama sekali — dua desa di `district_id`
yang sama pasti merujuk kecamatan resmi yang sama persis.

## Konsekuensi

- **Data lama (akun/lead pre-migrasi) kehilangan region sampai di-assign
  ulang manual.** Ini kerugian yang diterima sadar (bukan efek samping
  diabaikan) — auto-fuzzy-match berisiko salah pasang wilayah tanpa staf
  sadar, sementara `*_legacy` yang tetap tampak memberi konteks cukup untuk
  memilih ulang dengan cepat.
- **Territory View / agregasi wilayah** kini `GROUP BY district_id` (atau
  join ke `regions` untuk nama), bukan lagi `GROUP BY` teks — konsisten
  otomatis, tak perlu normalisasi.
- **Constraint FK baru** (`accounts_district_id_fkey`, `leads_district_id_fkey`)
  berarti `district_id` yang tak ada di master ditolak DB (SQLSTATE 23503) —
  ditangkap `accountWriteErr`/`leadWriteErr`, dipetakan ke pesan
  "Kecamatan yang dipilih tidak valid." (`workspace_errmsg.go`), bukan 500
  mentah.
- **Update dataset wilayah di masa depan** (pemekaran daerah, dsb.) berarti
  migrasi SQL baru yang menambah baris ke `regions` — bukan mengedit
  `00026` (migrasi sudah tersebar/dijalankan, sama prinsipnya dengan
  `00001_schema.sql`).

## Di luar cakupan (ditunda, disepakati eksplisit)

- Master untuk level Desa/Kelurahan (~83.000 baris) — tetap teks bebas milik
  akun (`village_name`).
- Master "Teritori" internal (pembagian wilayah kerja Sales/CSM) — kolom
  `territory` tak disentuh, tetap teks bebas, konsep berbeda dari wilayah
  administratif resmi.
- Auto-fuzzy-match backfill akun/lead lama ke `district_id` — staf assign
  manual.
- Endpoint `@get`/fragment-swap Datastar baru — cascading dropdown 100%
  client-side (§6).
