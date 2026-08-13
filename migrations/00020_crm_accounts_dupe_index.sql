-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- MODUL 4 SALES (follow-up M4-6) — index functional utk cek duplikat desa
-- SAAT KONVERSI lead → account.
--
-- KENAPA: idx_accounts_code (00005) hanya unique pada village_code, TAPI
-- village_code SELALU NULL saat konversi (lead tak pernah membawanya — kolom
-- itu Kode Kemendagri, diisi user belakangan). Jadi index itu tak pernah
-- menyala utk mencegah nama desa yang sama dibuat dua kali dari alur
-- konversi. Kandidat duplikat sebenarnya harus dicocokkan lewat
-- village_name (case-insensitive, trim), bukan village_code.
--
-- KEPUTUSAN EKSPLISIT USER (2026-08-13): cakupan HANYA saat konversi (bukan
-- create-account manual maupun create-lead), match EXACT case-insensitive
-- pada village_name, perilaku SOFT-WARNING (tetap bisa lanjut, bukan hard
-- block — nama desa sama valid utk beda dusun/kabupaten). Index ini menopang
-- query FindDuplicateAccountsByNameRegion (queries/accounts.sql) yang
-- dipanggil di GET halaman review konversi.
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_accounts_village_name_ci
    ON accounts (tenant_id, lower(trim(village_name)))
    WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_accounts_village_name_ci;
-- +goose StatementEnd
