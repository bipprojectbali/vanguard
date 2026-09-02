-- 00033_crm_cs_training_notes.sql — tambah kolom notes (catatan/kesimpulan
-- hasil training) ke cs_trainings (BL-28 #3). Diisi opsional saat menandai
-- 'completed' (ringkasan hasil) atau 'rescheduled' (alasan penjadwalan ulang).
--
-- KENAPA: aksi status per-baris sebelumnya tak punya tempat menyimpan
-- kesimpulan training. Nullable, tanpa backfill — baris lama = NULL ("belum
-- ada catatan"). Idempotent (IF NOT EXISTS) agar aman diulang di produksi.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE cs_trainings ADD COLUMN IF NOT EXISTS notes TEXT;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE cs_trainings DROP COLUMN IF EXISTS notes;
-- +goose StatementEnd
