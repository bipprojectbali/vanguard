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
-- SEMUA ~7.817 baris (3 level), dipakai SEKALI di handler form utk embed
-- <script type="application/json"> yang dibaca static/regions.js — cascading
-- dropdown Provinsi→Kabupaten/Kota→Kecamatan 100% di browser, tanpa round-trip
-- server per pilihan (lihat keputusan desain #5, plan 0009).
SELECT id, parent_region_id, level, name FROM regions
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
