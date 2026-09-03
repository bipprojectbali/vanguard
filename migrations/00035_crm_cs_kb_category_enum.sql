-- 00035_crm_cs_kb_category_enum.sql — kunci DOMAIN kb_articles.category dari
-- TEKS BEBAS → enum terkunci (BL-35).
--
-- KENAPA: membalik desain 00018 (komentar kb_articles_form.go menyatakan
-- category SENGAJA teks bebas — "kategori KB dinamis per workspace"). Keputusan
-- user (3 Sep 2026, opsi a): kategori jadi enum GLOBAL tetap (daftar tetap +
-- dropdown + CHECK), pola CERMIN visibility (kb_articles_visibility_chk 00018)
-- & Playbooks trigger_scenario. TRADEOFF DISAMPAIKAN & DITERIMA: hilang
-- fleksibilitas taksonomi per-workspace (bukan lagi per-desa). Bila kelak butuh
-- taksonomi per-workspace = tabel kb_categories ber-FK (opsi c, DI LUAR BL ini).
--
-- Enum final (keputusan user): Panduan Awal, Pembayaran, Kependudukan, Teknis,
-- Umum. Kolom TETAP nullable — kategori opsional (kosong = tak berkategori).
--
-- BACKFILL (keputusan user: seragam → 'Umum'): baris dgn category NON-NULL di
-- luar enum WAJIB dipetakan SEBELUM CHECK terpasang, kalau tidak constraint
-- menolak baris lama. Dipetakan → 'Umum' (bukan di-NULL-kan) agar konten lama
-- tetap berkategori. NULL dibiarkan NULL (sah: kategori opsional).

-- +goose Up
-- +goose StatementBegin
UPDATE kb_articles SET category = 'Umum', updated_at = now()
WHERE category IS NOT NULL
  AND category NOT IN ('Panduan Awal', 'Pembayaran', 'Kependudukan', 'Teknis', 'Umum');
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE kb_articles DROP CONSTRAINT IF EXISTS kb_articles_category_chk;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE kb_articles ADD CONSTRAINT kb_articles_category_chk
    CHECK (category IS NULL
           OR category IN ('Panduan Awal', 'Pembayaran', 'Kependudukan', 'Teknis', 'Umum'));
-- +goose StatementEnd

-- +goose Down
-- Balik: cukup lepas CHECK — category kembali teks bebas. Nilai lama yang
-- ter-backfill ke 'Umum' TAK bisa dipulihkan ke teks aslinya (informasi hilang
-- saat Up) — diterima, arah down memulihkan domain (teks bebas), bukan data
-- historis.
-- +goose StatementBegin
ALTER TABLE kb_articles DROP CONSTRAINT IF EXISTS kb_articles_category_chk;
-- +goose StatementEnd
