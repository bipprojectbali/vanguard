-- 00036_crm_ticket_status_simplify.sql — sederhanakan domain tickets.status
-- dari 4 nilai lama (BL-38).
--
-- KENAPA: implementasi lama boros slot. `ditugaskan` bisa diturunkan dari
-- `assigned_to` (bukan fase), dan `eskalasi` sebenarnya soal PRIORITAS bukan
-- fase — sambil KEHILANGAN dua fase yang penting untuk kejujuran SLA:
-- `diproses` (sedang dikerjakan) & `menunggu` (tertahan pihak lain). Keputusan
-- user (3 Sep 2026): distilasi 7 status spec crm.pdf (skema.md:542
-- New/Open/In Progress/Pending/Resolved/Closed/Reopened) → 4 fase inti:
--   baru (New+Open) → diproses (In Progress) → menunggu (Pending) → selesai (Resolved+Closed)
-- Reopen = TOMBOL (Selesai→Diproses), bukan status. "Siapa pegang" tetap dari
-- field assigned_to.
--
-- BACKFILL (setelah DROP CHECK lama, sebelum ADD CHECK baru — nilai baru
-- 'diproses' tak ada di domain lama, jadi constraint lama harus lepas dulu):
--   ditugaskan → baru     (penugasan diturunkan dari assigned_to, bukan fase)
--   eskalasi   → diproses  (tiket ter-eskalasi = sedang aktif dikerjakan)
-- baru & selesai tetap; menunggu tanpa sumber data lama (fase baru).
--
-- Pola DROP+ADD CHECK mengikuti 00013/00014/00015 & 00035. Default kolom TETAP
-- 'baru'. Constraint lama dari CHECK inline kolom (00021) bernama otomatis
-- `tickets_status_check` — DROP IF EXISTS aman idempotent.

-- +goose Up
-- +goose StatementBegin
ALTER TABLE tickets DROP CONSTRAINT IF EXISTS tickets_status_check;
-- +goose StatementEnd
-- +goose StatementBegin
UPDATE tickets SET status = 'baru', updated_at = now() WHERE status = 'ditugaskan';
-- +goose StatementEnd
-- +goose StatementBegin
UPDATE tickets SET status = 'diproses', updated_at = now() WHERE status = 'eskalasi';
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE tickets ADD CONSTRAINT tickets_status_check
    CHECK (status IN ('baru', 'diproses', 'menunggu', 'selesai'));
-- +goose StatementEnd

-- +goose Down
-- Balik ke domain lama. Backfill best-effort SEBELUM restore CHECK lama agar
-- baris nilai-baru tak melanggar: diproses→eskalasi (inverse Up), menunggu→baru
-- (tak ada padanan lama). LOSSY & tak memulihkan data historis (ditugaskan yang
-- ter-merge ke baru saat Up tak bisa dipisah lagi) — diterima; down memulihkan
-- DOMAIN, bukan data historis (pola sama 00035).
-- +goose StatementBegin
UPDATE tickets SET status = 'eskalasi', updated_at = now() WHERE status = 'diproses';
-- +goose StatementEnd
-- +goose StatementBegin
UPDATE tickets SET status = 'baru', updated_at = now() WHERE status = 'menunggu';
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE tickets DROP CONSTRAINT IF EXISTS tickets_status_check;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE tickets ADD CONSTRAINT tickets_status_check
    CHECK (status IN ('baru', 'ditugaskan', 'eskalasi', 'selesai'));
-- +goose StatementEnd
