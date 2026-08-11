-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- DESKRIPSI PERAN BISNIS — kalimat singkat "peran ini untuk apa".
--
-- Wireframe Roles & Permissions (9.2) menampilkan kolom Deskripsi di sebelah
-- nama peran: keterangan satu baris yang membantu operator memilih peran tanpa
-- membuka matriks izinnya. Sebelumnya business_roles hanya punya display_name
-- (label layar) — tak ada tempat menaruh maksud peran.
--
-- NOT NULL DEFAULT '' (bukan NULL): kolom keterangan opsional lebih baik "kosong"
-- daripada tiga-nilai (ada/kosong/NULL). Peran lama & peran baru tanpa deskripsi
-- sama-sama '' — view cukup cek string kosong, tak perlu bedakan NULL.
--
-- Backfill 5 peran bawaan dengan kalimat dari wireframe. Dipilih by-name (bukan
-- is_system) agar hanya menyentuh peran seed asli; peran kustom operator (nama
-- lain) tak tertimpa. Idempoten: hanya menulis bila deskripsi masih '' (migrasi
-- diulang atau operator sudah menyunting → tak ditimpa).
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
ALTER TABLE business_roles
    ADD COLUMN IF NOT EXISTS description TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose StatementBegin
UPDATE business_roles br
SET description = v.description
FROM (VALUES
    ('admin',   'Akses penuh seluruh modul & konfigurasi sistem'),
    ('manager', 'Menyetujui diskon & renewal, lihat seluruh tim'),
    ('sales',   'Leads, deals, quotes — hanya desa yang ditugaskan padanya'),
    ('csm',     'Health score, renewal, onboarding desa binaan'),
    ('support', 'Tiket & SLA — tanpa akses data komersial')
) AS v(name, description)
WHERE br.name = v.name AND br.description = '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE business_roles DROP COLUMN IF EXISTS description;
-- +goose StatementEnd
