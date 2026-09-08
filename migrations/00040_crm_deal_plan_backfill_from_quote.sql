-- 00040_crm_deal_plan_backfill_from_quote.sql — BL-100 (Fix B), pemulihan DATA LAMA.
--
-- KENAPA: subscriptionFromWonDeal (BL-21) membaca deals.plan_requested_id, tetapi
-- kolom itu TAK PERNAH diisi jalur UI mana pun (buat/convert/edit deal tak menyetel),
-- sehingga Closed Won selalu ditolak `plan_required` walau quote sudah Accepted &
-- berisi paket. Fix B mulai mengisi kolom saat quote di-Accept (handler QuoteStatus);
-- migrasi ini memulihkan deal dgn quote yang sudah Accepted SEBELUM fix (mis. DEAL-156).
--
-- Aturan SAMA dgn handler: hanya deal dengan TEPAT SATU paket distinct dari quote
-- Accepted hidupnya yang diisi; 0 atau >1 paket berbeda → dilewati (jangan menebak).
-- Guard `plan_requested_id IS NULL` → IDEMPOTEN (aman diulang; tak menimpa nilai yang
-- sudah ada). Migrasi dijalankan role privileged (bypass RLS) → menjangkau semua tenant.

-- +goose Up
-- +goose StatementBegin
UPDATE deals d
SET plan_requested_id = p.plan_id, updated_at = now()
FROM (
    SELECT q.deal_id, MIN(qi.plan_id) AS plan_id
    FROM quotes q
    JOIN quote_items qi ON qi.quote_id = q.id AND qi.plan_id IS NOT NULL
    WHERE q.quote_status = 'Accepted' AND q.deleted_at IS NULL AND q.deal_id IS NOT NULL
    GROUP BY q.deal_id
    HAVING COUNT(DISTINCT qi.plan_id) = 1
) p
WHERE d.id = p.deal_id AND d.plan_requested_id IS NULL AND d.deleted_at IS NULL;
-- +goose StatementEnd

-- +goose Down
-- Tak bisa dibalik bermakna: setelah backfill kita tak bisa membedakan deal yang
-- NULL "karena belum punya paket" dari yang "diisi migrasi ini". No-op (data-only,
-- non-destruktif) — arah down tak menghapus data yang sah.
-- +goose StatementBegin
SELECT 1;
-- +goose StatementEnd
