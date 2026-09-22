-- regions.sql — master wilayah administratif GLOBAL (Provinsi → Kabupaten/Kota →
-- Kecamatan), lihat migrations/00026_crm_regions.sql & docs/decisions/0009. Tabel ini
-- TANPA tenant_id/RLS (data sama utk semua tenant, pola sama dgn platform_staff) —
-- semua query di sini aman dipanggil lewat h.q(ctx) (tx ber-tenant) MAUPUN db.WithSuper,
-- keduanya baca baris yang sama.

-- name: ListProvinces :many
-- Level 1 (Provinsi), dipakai isi opsi pertama dropdown wilayah server-side (fallback
-- non-JS / SSR awal) sebelum static/regions.js mengambil alih interaksi client-side.
SELECT * FROM regions
WHERE level = 1
ORDER BY name;

-- name: ListRegenciesByProvince :many
-- Level 2 (Kabupaten/Kota) di bawah satu provinsi. Tak dipakai cascading di JS
-- (dataset penuh sudah di-embed via ListAllRegions), tapi berguna utk validasi
-- server-side / API lain di masa depan.
SELECT * FROM regions
WHERE level = 2 AND parent_region_id = sqlc.arg(parent_region_id)
ORDER BY name;

-- name: ListDistrictsByRegency :many
-- Level 3 (Kecamatan) di bawah satu kabupaten/kota. Sama alasannya dgn
-- ListRegenciesByProvince — bukan jalur utama (JS-side), cadangan validasi.
SELECT * FROM regions
WHERE level = 3 AND parent_region_id = sqlc.arg(parent_region_id)
ORDER BY name;

-- name: ListAllRegions :many
-- SEMUA ~7.837 baris (3 level: Provinsi/Kabupaten-Kota/Kecamatan), dipakai
-- SEKALI di handler form utk embed <script type="application/json"> yang
-- dibaca static/regions.js — cascading dropdown Provinsi→Kabupaten/Kota→
-- Kecamatan 100% di browser, tanpa round-trip server per pilihan (lihat
-- keputusan desain #5, plan 0009). WHERE level <= 3 WAJIB: tanpa ini query
-- ikut menyeret ~83.762 baris Desa/Kelurahan (level 4, migrasi 00039) yang
-- TAK dikonsumsi static/regions.js (level 4 sengaja lazy-fetch terpisah via
-- ListVillagesByDistrict/data-villages-url, ADR 0009 — "terlalu besar utk
-- diembed") → payload ~11x lebih besar dari perlu, penyebab loading lambat
-- form Lead/Account baru saat diakses lewat jalur jaringan lambat (tunnel).
SELECT id, parent_region_id, level, name FROM regions
WHERE level <= 3
ORDER BY level, name;

-- name: GetRegionAncestry :one
-- Nama Kecamatan + Kabupaten/Kota + Provinsi sekaligus utk SATU district_id — dipakai
-- READ view (list/detail account & lead, banner dupe konversi) yang tampil tanpa JS.
-- Self-join 3x murni (bukan recursive CTE) karena kedalaman selalu tepat 3, lebih
-- mudah dibaca sqlc & planner Postgres.
SELECT
    d.id           AS district_id,
    d.name         AS district_name,
    rgc.id         AS regency_id,
    rgc.name       AS regency_name,
    prov.id        AS province_id,
    prov.name      AS province_name
FROM regions d
JOIN regions rgc  ON rgc.id = d.parent_region_id
JOIN regions prov ON prov.id = rgc.parent_region_id
WHERE d.id = sqlc.arg(district_id) AND d.level = 3;

-- name: GetDistrictCode :one
-- Kode Kemendagri (mis. "32.01.01") satu Kecamatan (level 3) — dipakai sbg PREFIX
-- village_code otomatis saat create desa (generateVillageCode). Filter level = 3
-- eksplisit: id level 1/2 (atau id tak ada) → pgx.ErrNoRows, dipetakan pemanggil
-- ke galat "district_id" (payload district_id bukan Kecamatan sah).
SELECT code FROM regions
WHERE id = sqlc.arg(id) AND level = 3;

-- name: ListVillagesByDistrict :many
-- Daftar Desa/Kelurahan (level 4) di bawah SATU Kecamatan (level 3) — dipakai
-- endpoint server GET /accounts/villages (BL-66). TAK di-embed ke payload
-- dropdown seperti 3 level di atas karena volume desa se-Indonesia (~83rb baris,
-- ADR 0009): meng-embed semua akan membengkakkan HTML form → di-fetch same-origin
-- per Kecamatan saat dipilih (lolos CSP default-src 'self'). `code` = Kode
-- Kemendagri desa 4-segmen (mis. "32.01.01.2001") yang jadi village_code akun.
SELECT id, code, name FROM regions
WHERE level = 4 AND parent_region_id = sqlc.arg(parent_region_id)
ORDER BY name;

-- name: GetVillageRegion :one
-- Resolusi SATU Desa/Kelurahan (level 4) dari id pilihan form → dipakai
-- AccountCreate/AccountUpdate (BL-66) menurunkan village_code (=code, 4 segmen
-- Kemendagri asli), village_name (=name), district_id (=parent_region_id
-- Kecamatan induk). Filter level = 4 eksplisit: id level lain / tak ada →
-- pgx.ErrNoRows → galat "village_id" (payload bukan Desa sah).
SELECT id, code, name, parent_region_id FROM regions
WHERE id = sqlc.arg(id) AND level = 4;

-- name: GetVillageByCode :one
-- Cari Desa/Kelurahan (level 4) via Kode Kemendagri — dipakai prefill form
-- Sunting akun (BL-66): akun menyimpan village_code (bukan region id), jadi untuk
-- pra-pilih dropdown Desa perlu memetakan balik code → id region. Tak ketemu
-- (kode akun lama non-Kemendagri) → pgx.ErrNoRows, pemanggil biarkan dropdown kosong.
SELECT id, code, name, parent_region_id FROM regions
WHERE code = sqlc.arg(code) AND level = 4;

-- name: ListVillagesByCodes :many
-- Resolusi BANYAK kode Desa sekaligus (impor CSV) — hindari N+1 SELECT per
-- baris. Baris yang kodenya tak ketemu tak muncul di hasil; pemanggil
-- mencocokkan balik by code utk tahu yang hilang.
SELECT id, code, name, parent_region_id FROM regions
WHERE code = ANY(sqlc.arg(codes)::text[]) AND level = 4;

-- name: ListDistrictsByCodes :many
-- Resolusi BANYAK kode Kecamatan (level 3) sekaligus — impor CSV Lead
-- (BL-133), hindari N+1 (Rule 13). Pola SAMA ListVillagesByCodes di atas
-- tapi level = 3. Nama BEDA dari "GetDistrictByCode" yang disebut draf
-- tasks.md BL-133 (yang menulis "pola sama GetVillageByCode" — versi :one)
-- — dipilih versi BATCH krn satu file CSV bisa berisi ratusan baris; query
-- :one per baris akan jadi N+1. Baris yang kodenya tak ketemu tak muncul di
-- hasil; pemanggil mencocokkan balik by code utk tahu yang hilang.
SELECT id, code, name FROM regions
WHERE code = ANY(sqlc.arg(codes)::text[]) AND level = 3;

-- name: SearchRegionsByCode :many
-- BL-163: modal pencarian global "Cari Kode Desa/Kecamatan" — pencocokan
-- PERSIS Kode Kemendagri, Kecamatan (level 3) ATAU Desa (level 4). `code`
-- UNIQUE (migrations/00026) jadi hasil praktis maks 1 baris, tapi kode
-- sendiri tak menyimpan levelnya (format teks sama bentuknya utk level lain),
-- jadi tetap dua cabang UNION ALL bukan cari level dulu. Kolom output SAMA
-- persis dgn SearchRegionsByName (kontrak dibaca handler yang sama):
-- village_name string KOSONG (bukan NULL — sqlc/pgx tak infer nullability
-- lintas cabang UNION dgn benar, lihat commit ini) utk baris Kecamatan,
-- handler render "-" saat kosong.
SELECT r.code, r.name AS village_name, d.name AS district_name,
       rgc.name AS regency_name, prov.name AS province_name
FROM regions r
JOIN regions d    ON d.id = r.parent_region_id
JOIN regions rgc  ON rgc.id = d.parent_region_id
JOIN regions prov ON prov.id = rgc.parent_region_id
WHERE r.level = 4 AND r.code = sqlc.arg(code)
UNION ALL
SELECT r.code, ''::text AS village_name, r.name AS district_name,
       rgc.name AS regency_name, prov.name AS province_name
FROM regions r
JOIN regions rgc  ON rgc.id = r.parent_region_id
JOIN regions prov ON prov.id = rgc.parent_region_id
WHERE r.level = 3 AND r.code = sqlc.arg(code)
ORDER BY district_name
LIMIT sqlc.arg(page_size);

-- name: SearchRegionsByDistrict :many
-- BL-163 lanjutan: hasil tab "Wilayah" (cascading Provinsi→Kabupaten/Kota→
-- Kecamatan) — Kecamatan yang dipilih user itu SENDIRI (baris pertama,
-- village_name kosong, sama pola dgn SearchRegionsByCode) + SEMUA Desa
-- anaknya. Bentuk kolom SAMA PERSIS dgn SearchRegionsByCode/ByName (kontrak
-- dibaca ui.RegionSearchResults yang sama, tanpa perubahan UI).
SELECT r.code, ''::text AS village_name, r.name AS district_name,
       rgc.name AS regency_name, prov.name AS province_name
FROM regions r
JOIN regions rgc  ON rgc.id = r.parent_region_id
JOIN regions prov ON prov.id = rgc.parent_region_id
WHERE r.level = 3 AND r.id = sqlc.arg(district_id)
UNION ALL
SELECT r.code, r.name AS village_name, d.name AS district_name,
       rgc.name AS regency_name, prov.name AS province_name
FROM regions r
JOIN regions d    ON d.id = r.parent_region_id
JOIN regions rgc  ON rgc.id = d.parent_region_id
JOIN regions prov ON prov.id = rgc.parent_region_id
WHERE r.level = 4 AND r.parent_region_id = sqlc.arg(district_id)
ORDER BY village_name
LIMIT sqlc.arg(page_size);

-- name: SearchRegionsByName :many
-- BL-163: cabang pencarian nama (bukan kode) modal yang sama — ILIKE
-- Kecamatan (level 3) ATAU Desa (level 4), hasil DICAMPUR satu daftar
-- (bukan dua seksi terpisah) sesuai keputusan desain BL-163. `name` TANPA
-- indeks (ADR 0009: volume desa besar, tapi pencarian debounced/
-- submit-triggered dianggap cukup) — seq scan ~91rb baris per panggilan,
-- diterima sadar sbg trade-off keputusan BL-163 (bukan lupa index).
-- Handler bertanggung jawab memangkas `pattern` ke >= 3 karakter sebelum
-- panggil (guard server-side, lihat regions_search.go). village_name string
-- kosong (bukan NULL, sama alasannya dgn SearchRegionsByCode) utk baris
-- Kecamatan.
SELECT * FROM (
    SELECT r.code, r.name AS village_name, d.name AS district_name,
           rgc.name AS regency_name, prov.name AS province_name,
           r.name AS match_name
    FROM regions r
    JOIN regions d    ON d.id = r.parent_region_id
    JOIN regions rgc  ON rgc.id = d.parent_region_id
    JOIN regions prov ON prov.id = rgc.parent_region_id
    WHERE r.level = 4 AND r.name ILIKE sqlc.arg(pattern)
    UNION ALL
    SELECT r.code, ''::text AS village_name, r.name AS district_name,
           rgc.name AS regency_name, prov.name AS province_name,
           r.name AS match_name
    FROM regions r
    JOIN regions rgc  ON rgc.id = r.parent_region_id
    JOIN regions prov ON prov.id = rgc.parent_region_id
    WHERE r.level = 3 AND r.name ILIKE sqlc.arg(pattern)
) matches
ORDER BY match_name
LIMIT sqlc.arg(page_size);
