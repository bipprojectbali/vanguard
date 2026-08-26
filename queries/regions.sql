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
