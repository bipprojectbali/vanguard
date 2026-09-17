-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- BL-133 follow-up — index functional utk PERINGATAN (non-blocking) duplikat
-- lead_name di pratinjau impor CSV Lead.
--
-- KENAPA: mirror PERSIS 00020_crm_accounts_dupe_index.sql (peringatan
-- duplikat nama desa saat konversi Lead→Account) — tapi di sini utk lead_name
-- SAAT PRATINJAU IMPOR CSV, bukan konversi. lead_name BUKAN kolom unik
-- (banyak lead boleh berbagi nama kontak yang sama, mis. dua perusahaan
-- berbeda kebetulan punya kontak bernama sama) — jadi ini SOFT-WARNING,
-- BUKAN hard block, sama semangatnya dgn 00020. Menopang query
-- ListLeadsByNamesCI (queries/leads.sql) yang dipanggil BATCHED (banyak
-- baris CSV sekaligus, Rule 13) dari LeadImportPreview.
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_leads_name_ci
    ON leads (tenant_id, lower(trim(lead_name)))
    WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_leads_name_ci;
-- +goose StatementEnd
