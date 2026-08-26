-- +goose Up
-- +goose StatementBegin

-- ═══════════════════════════════════════════════════════════════════════════
-- accounts/leads.district_id — pindah region dari teks bebas ke FK regions
-- (00026), lihat docs/decisions/0009-master-wilayah-administratif.md.
--
-- Kolom teks lama DI-RENAME (bukan di-drop) jadi *_legacy: masih ada baris
-- lama yang isinya teks manusia ("Jawa Barat", "Kab. Bandung", ...) dan
-- menghapusnya permanen sebelum staff sempat memilih ulang district_id yang
-- benar berarti kehilangan satu-satunya petunjuk utk backfill manual. Kolom
-- ini TIDAK lagi ditulis kode baru — murni jejak baca, boleh di-drop di
-- migration terpisah setelah semua akun/lead lama sudah di-assign ulang.
--
-- KEPUTUSAN EKSPLISIT USER: data lama TIDAK di-fuzzy-match ke district_id —
-- semua baris lama biarkan district_id NULL, staff pilih manual satu-satu.
-- ═══════════════════════════════════════════════════════════════════════════

ALTER TABLE accounts RENAME COLUMN province TO province_legacy;
ALTER TABLE accounts RENAME COLUMN regency  TO regency_legacy;
ALTER TABLE accounts RENAME COLUMN district TO district_legacy;
ALTER TABLE accounts ADD COLUMN district_id BIGINT REFERENCES regions(id) ON DELETE SET NULL;

ALTER TABLE leads RENAME COLUMN province TO province_legacy;
ALTER TABLE leads RENAME COLUMN regency  TO regency_legacy;
ALTER TABLE leads RENAME COLUMN district TO district_legacy;
ALTER TABLE leads ADD COLUMN district_id BIGINT REFERENCES regions(id) ON DELETE SET NULL;

DROP INDEX IF EXISTS idx_accounts_region;
CREATE INDEX idx_accounts_district ON accounts (tenant_id, district_id)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_leads_district ON leads (tenant_id, district_id)
    WHERE deleted_at IS NULL;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_leads_district;
DROP INDEX IF EXISTS idx_accounts_district;
CREATE INDEX idx_accounts_region ON accounts (tenant_id, province_legacy, regency_legacy, district_legacy)
    WHERE deleted_at IS NULL;

ALTER TABLE leads DROP COLUMN district_id;
ALTER TABLE leads RENAME COLUMN district_legacy TO district;
ALTER TABLE leads RENAME COLUMN regency_legacy  TO regency;
ALTER TABLE leads RENAME COLUMN province_legacy TO province;

ALTER TABLE accounts DROP COLUMN district_id;
ALTER TABLE accounts RENAME COLUMN district_legacy TO district;
ALTER TABLE accounts RENAME COLUMN regency_legacy  TO regency;
ALTER TABLE accounts RENAME COLUMN province_legacy TO province;

-- +goose StatementEnd
