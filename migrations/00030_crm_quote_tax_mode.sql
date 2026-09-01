-- +goose Up
-- +goose StatementBegin

-- ═══════════════════════════════════════════════════════════════════════════
-- BL-14 — Pajak Quote pindah dari input rupiah manual di HEADER → kontrol di
-- BUILDER item dengan dua mode:
--   • 'percent' — tarif (mis. PPN 11%) dihitung dari subtotal; MENGIKUTI subtotal
--     otomatis saat item ditambah/diubah/dihapus.
--   • 'amount'  — nilai rupiah tetap (mis. materai Rp10.000); tak ikut subtotal.
--
-- tax_mode  = cara hitung (menentukan apakah tax_rate atau nilai tetap dipakai).
-- tax_rate  = tarif persen, dipakai HANYA saat mode 'percent' (NULL di mode lain).
-- tax_amount (kolom lama, 00010) TETAP = SNAPSHOT hasil rupiah → grand_total &
--   laporan tak berubah bentuk; hanya CARA mengisinya yang berpindah.
--
-- DEFAULT 'amount' DISENGAJA: baris lama menyimpan pajak sebagai rupiah manual di
-- tax_amount. Menaikkan default ke 'percent' akan salah menafsirkan nilai lama
-- (5000 rupiah → 5000%). Dengan default 'amount' + tax_rate NULL, baris lama utuh:
-- recomputeTotals mode 'amount' memakai tax_amount tersimpan apa adanya.
-- Idempotent (IF NOT EXISTS) — aman diulang.
-- ═══════════════════════════════════════════════════════════════════════════

ALTER TABLE quotes
    ADD COLUMN IF NOT EXISTS tax_mode TEXT NOT NULL DEFAULT 'amount'
        CHECK (tax_mode IN ('percent', 'amount'));

ALTER TABLE quotes
    ADD COLUMN IF NOT EXISTS tax_rate NUMERIC(5,2);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE quotes DROP COLUMN IF EXISTS tax_rate;
ALTER TABLE quotes DROP COLUMN IF EXISTS tax_mode;

-- +goose StatementEnd
