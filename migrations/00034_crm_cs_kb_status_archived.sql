-- 00034_crm_cs_kb_status_archived.sql — redesain DOMAIN status kb_articles
-- dari Draft/Review/Published → Draft/Published/Archived (BL-34).
--
-- KENAPA: membalik keputusan 2026-08-13 (00018 memilih Draft/Review/Published
-- mengikuti wireframe 6.10 + role doc). Keputusan user (3 Sep 2026): buang
-- gerbang mutu `Review` (artikel terbit langsung Draft→Published) & tambah
-- `Archived` = pensiun-tanpa-hapus (arsip, semantik skema.md §6h). Ini
-- PENYIMPANGAN SADAR dari wireframe Penpot 6.10 (aturan wireframe=sumber
-- kebenaran → deviasi dicatat di docs/crm/sistem-dan-role.md §6.10). Pola
-- migrasi DROP+ADD CHECK meniru 00013/00014/00015 — 00018 sudah menulis
-- "jangan sunting CHECK ini in-place setelah production, tambah via migrasi
-- baru": inilah migrasi itu.
--
-- BACKFILL: baris `status='Review'` lama WAJIB dipetakan SEBELUM CHECK baru
-- terpasang, kalau tidak baris lama menolak constraint. Dipetakan → 'Draft'
-- (keputusan user: konservatif — jangan auto-terbitkan konten yang belum
-- tuntas ditinjau).

-- +goose Up
-- +goose StatementBegin
UPDATE kb_articles SET status = 'Draft', updated_at = now() WHERE status = 'Review';
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE kb_articles DROP CONSTRAINT IF EXISTS kb_articles_status_chk;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE kb_articles ADD CONSTRAINT kb_articles_status_chk
    CHECK (status IN ('Draft', 'Published', 'Archived'));
-- +goose StatementEnd

-- +goose Down
-- Balik: baris 'Archived' → 'Draft' SEBELUM kembalikan CHECK lama (tanpa
-- Archived), agar tak menolak constraint. Baris yang tadinya 'Review' tak
-- bisa dipulihkan (informasi hilang saat Up) — diterima, arah down bersifat
-- pemulihan domain, bukan data historis.
-- +goose StatementBegin
UPDATE kb_articles SET status = 'Draft', updated_at = now() WHERE status = 'Archived';
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE kb_articles DROP CONSTRAINT IF EXISTS kb_articles_status_chk;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE kb_articles ADD CONSTRAINT kb_articles_status_chk
    CHECK (status IN ('Draft', 'Review', 'Published'));
-- +goose StatementEnd
