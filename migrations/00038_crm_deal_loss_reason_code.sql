-- 00038_crm_deal_loss_reason_code.sql — picklist alasan kalah bersih untuk deals
-- (BL-44 butir 3a, keputusan user 4 Sep 2026).
--
-- KENAPA: `win_loss_reason` (00009) = TEKS BEBAS → panel "Alasan Kalah" di Sales
-- Report (BL-43) GROUP BY teks apa adanya, rawan variasi ejaan ("Harga terlalu
-- tinggi" vs "harga mahal" jadi dua baris). Untuk akurasi laporan, tambah kolom
-- TERSTRUKTUR `loss_reason_code` (picklist terkunci). `win_loss_reason` DIPERTA-
-- HANKAN sebagai detail bebas (mis. "kalah tender vs Kompetitor X"); kode dipakai
-- untuk grouping, teks untuk konteks. Aditif — nol data-loss.
--
-- POLA: kolom baru NULLable (tabel deals berisi data; §4) + CHECK IN inline pada
-- ADD COLUMN IF NOT EXISTS (kolom sudah ada → statement no-op, idempotent). Enum
-- via TEXT+CHECK selaras deals_stage_chk (bukan tipe ENUM Postgres — konsisten
-- konvensi repo). Index parsial (tenant_id, loss_reason_code) WHERE Closed Lost
-- mendukung GROUP BY laporan (§13).
--
-- BACKFILL best-effort dari teks lama untuk deal Closed Lost yang punya
-- win_loss_reason: petakan kata kunci → kode via CASE prioritas; sisanya
-- 'Lainnya'. Hanya baris loss_reason_code masih NULL (idempotent).

-- +goose Up
-- +goose StatementBegin
ALTER TABLE deals ADD COLUMN IF NOT EXISTS loss_reason_code TEXT
    CONSTRAINT deals_loss_reason_code_chk
        CHECK (loss_reason_code IS NULL OR loss_reason_code IN
            ('Harga', 'Fitur', 'Kompetitor', 'Anggaran', 'Lainnya'));
-- +goose StatementEnd
-- +goose StatementBegin
UPDATE deals SET loss_reason_code = CASE
        WHEN win_loss_reason ILIKE '%harga%'
          OR win_loss_reason ILIKE '%mahal%'
          OR win_loss_reason ILIKE '%price%'         THEN 'Harga'
        WHEN win_loss_reason ILIKE '%fitur%'
          OR win_loss_reason ILIKE '%feature%'
          OR win_loss_reason ILIKE '%fungsi%'        THEN 'Fitur'
        WHEN win_loss_reason ILIKE '%kompetitor%'
          OR win_loss_reason ILIKE '%pesaing%'
          OR win_loss_reason ILIKE '%saingan%'
          OR win_loss_reason ILIKE '%competitor%'    THEN 'Kompetitor'
        WHEN win_loss_reason ILIKE '%anggaran%'
          OR win_loss_reason ILIKE '%budget%'
          OR win_loss_reason ILIKE '%dana%'          THEN 'Anggaran'
        ELSE 'Lainnya'
    END
WHERE stage = 'Closed Lost'
  AND win_loss_reason IS NOT NULL
  AND loss_reason_code IS NULL;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_deals_loss_reason_code
    ON deals (tenant_id, loss_reason_code)
    WHERE deleted_at IS NULL AND stage = 'Closed Lost';
-- +goose StatementEnd

-- +goose Down
-- Buang index & kolom. Backfill tak dipulihkan (kode turunan dari teks yang tetap
-- ada di win_loss_reason) — down memulihkan SKEMA, bukan menghapus data sumber.
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_deals_loss_reason_code;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE deals DROP COLUMN IF EXISTS loss_reason_code;
-- +goose StatementEnd
