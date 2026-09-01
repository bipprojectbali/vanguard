-- +goose Up
-- +goose StatementBegin

-- ═══════════════════════════════════════════════════════════════════════════
-- Indeks keyset (created_at DESC, id DESC) untuk 4 katalog master yang kini
-- dipaginate (BL-6): plans, sla_policies, playbooks, kb_articles. Sebelumnya
-- daftar kelola-nya mengambil SELURUH baris tanpa LIMIT — aman selagi katalog
-- kecil, tapi tak berpagar saat tumbuh. Query List*All kini keyset + LIMIT,
-- jadi butuh indeks yang MENYAMAI ORDER BY halaman agar cursor dilayani indeks
-- (bukan sort penuh tiap request). Menyamai pola idx_accounts_live /
-- idx_contacts_live / idx_quotes_live (00005/00010).
--
-- tenant_id di depan: RLS memfilter per-tenant lebih dulu, jadi indeks komposit
-- menopang scan yang sudah ter-scope satu workspace. Tak partial (katalog master
-- TANPA soft-delete: is_active/status=false/'Draft' = pensiun/draf, TETAP tampil
-- di daftar kelola — beda dari tabel ber-deleted_at yang partial WHERE hidup).
-- Idempotent (IF NOT EXISTS) sesuai guard produksi.
-- ═══════════════════════════════════════════════════════════════════════════

CREATE INDEX IF NOT EXISTS idx_plans_keyset       ON plans        (tenant_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_sla_keyset         ON sla_policies (tenant_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_playbooks_keyset   ON playbooks    (tenant_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_kb_articles_keyset ON kb_articles  (tenant_id, created_at DESC, id DESC);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS idx_plans_keyset;
DROP INDEX IF EXISTS idx_sla_keyset;
DROP INDEX IF EXISTS idx_playbooks_keyset;
DROP INDEX IF EXISTS idx_kb_articles_keyset;

-- +goose StatementEnd
