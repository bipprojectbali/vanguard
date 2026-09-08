-- 00041_crm_quote_term_and_accept_unique.sql — BL-88 (PR1, "quote otoritatif").
--
-- KENAPA: nilai yang diakui (MRR/ARR laporan) selama ini HANYA dari deal.amount yang
-- diketik manual — grand_total quote tak pernah mengalir ke sana (user bisa Accept
-- quote senilai X lalu isi amount = Y ≠ X tanpa peringatan). BL-88 menjadikan quote
-- sumber kebenaran komersial: saat Accept, grand_total disalin ke deal.amount, dan
-- TERMIN langganan pindah ke quote (Won membacanya dari quote, bukan deal).
--
-- Migrasi ini menyiapkan skema untuk pergeseran itu (PR1):
--   1. quotes.subscription_term + contract_term_months — termin milik quote. JANGAN
--      pakai ulang expiration_date (itu masa-berlaku quote, beda makna). CHECK mencermin
--      deals_term_chk (00009) agar himpunan nilai legal identik dgn form deal lama.
--   2. Partial unique index: TEPAT 1 quote Accepted per deal (menghapus semantik lama
--      "accept terkahir menang"). deal_id nullable → index disaring deal_id IS NOT NULL;
--      tenant_id disertakan agar konsisten dgn index quotes lain (RLS per-tenant).
--
-- deals.subscription_term & deals.plan_requested_id SENGAJA DIBIARKAN: jalur Won
-- single-plan PR1 masih memakainya (di-retire di PR2 multi-baris). Inkremental +
-- idempotent (rule: IF NOT EXISTS / guard katalog utk CHECK yang tak punya IF NOT EXISTS).

-- +goose Up

-- ── Kolom termin di quotes (nullable; kosong = quote belum menetapkan termin) ──
-- +goose StatementBegin
ALTER TABLE quotes ADD COLUMN IF NOT EXISTS subscription_term    TEXT;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE quotes ADD COLUMN IF NOT EXISTS contract_term_months INTEGER;
-- +goose StatementEnd

-- CHECK termin (cermin deals_term_chk 00009). Postgres tak punya ADD CONSTRAINT IF
-- NOT EXISTS → guard via katalog (idempotent utk re-run, pola 00012).
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'quotes_term_chk'
    ) THEN
        ALTER TABLE quotes
            ADD CONSTRAINT quotes_term_chk CHECK
            (subscription_term IS NULL OR subscription_term IN
             ('Monthly','Annual','Multi-year'));
    END IF;
END $$;
-- +goose StatementEnd

-- ── Invarian: TEPAT 1 quote Accepted per deal (BL-88 poin 7) ────────────────
-- Menggantikan "accept terakhir menang". Handler memberi pesan ramah (pre-check);
-- index ini = penjaga keras bila balapan. deal_id IS NOT NULL: quote lepas-deal
-- (deal terhapus → SET NULL) tak ikut invarian.
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_quotes_one_accepted
    ON quotes (tenant_id, deal_id)
    WHERE quote_status = 'Accepted' AND deleted_at IS NULL AND deal_id IS NOT NULL;
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_quotes_one_accepted;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE quotes DROP CONSTRAINT IF EXISTS quotes_term_chk;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE quotes DROP COLUMN IF EXISTS contract_term_months;
-- +goose StatementEnd
-- +goose StatementBegin
ALTER TABLE quotes DROP COLUMN IF EXISTS subscription_term;
-- +goose StatementEnd
