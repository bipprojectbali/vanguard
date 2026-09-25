-- 00053_crm_deal_last_active_stage.sql — snapshot tahap aktif terakhir sebelum
-- deal ditutup (BL-173, sub-scope "stepper visual", keputusan user 25 Sep 2026).
--
-- KENAPA: BL-173 membuka "Closed Lost" dari SETIAP tahap aktif, bukan cuma
-- setelah march-through penuh ke Negotiation. Stepper "Tahap Pipeline"
-- (dealStepper, sales_deals_detail_stage.go) mewarnai murni berdasar INDEKS
-- current stage di daftar tahap — untuk deal terminal, semua node sebelum
-- terminal otomatis hijau, seolah deal melewati SEMUA tahap. Untuk deal yang
-- kalah langsung dari Prospecting, ini menampilkan riwayat palsu (Qualifikasi/
-- Demo/Proposal/Negotiation seakan-akan terlewati padahal tidak).
--
-- audit_logs SUDAH merekam event "deal.stage" tapi target_type="workspace" &
-- deal_id hanya ada di JSON metadata (tak terindeks) — tak layak dipakai untuk
-- query per-deal yang sering diakses (halaman detail). Snapshot kolom lebih
-- murah & langsung dipakai UpdateDealStage yang sudah menyentuh baris ini.
--
-- POLA: kolom NULLable (tabel deals berisi data; §4) + CHECK IN inline pada
-- ADD COLUMN IF NOT EXISTS, selaras deals_loss_reason_code_chk (00038). Hanya
-- 5 tahap AKTIF valid (bukan "Closed Won"/"Closed Lost" — snapshot HANYA
-- berarti utk deal terminal, dan nilainya SELALU salah satu tahap aktif).
--
-- BACKFILL: sebelum BL-173, aturan sequential-only (BL-159) MEWAJIBKAN march-
-- through penuh ke Negotiation sebelum bisa ditutup — jadi utk deal terminal
-- existing, 'Negotiation' adalah nilai yang historis akurat (bukan tebakan).
-- Hanya baris last_active_stage masih NULL (idempotent).

-- +goose Up
-- +goose StatementBegin
ALTER TABLE deals ADD COLUMN IF NOT EXISTS last_active_stage TEXT
    CONSTRAINT deals_last_active_stage_chk
        CHECK (last_active_stage IS NULL OR last_active_stage IN
            ('Prospecting', 'Qualification', 'Demo', 'Proposal', 'Negotiation'));
-- +goose StatementEnd
-- +goose StatementBegin
UPDATE deals SET last_active_stage = 'Negotiation'
WHERE stage IN ('Closed Won', 'Closed Lost')
  AND last_active_stage IS NULL;
-- +goose StatementEnd

-- +goose Down
-- Buang kolom. Backfill tak dipulihkan (snapshot turunan, bukan sumber data).
-- +goose StatementBegin
ALTER TABLE deals DROP COLUMN IF EXISTS last_active_stage;
-- +goose StatementEnd
