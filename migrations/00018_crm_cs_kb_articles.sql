-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- MODUL 6 CUSTOMER SUCCESS (slice A3) — tabel `kb_articles` (6.10). Katalog
-- artikel bantuan mandiri (Knowledge Base) — Support menulis & memelihara,
-- tujuan deflect tiket berulang. Dibaca lintas peran CS (sales/csm = read).
--
-- Pola SAMA PERSIS dgn playbooks (00017)/sla_policies (00005/00016): master
-- milik WORKSPACE, TANPA F3 ownership (RLS satu-satunya pengurung), TANPA
-- soft-delete. Fresh CREATE TABLE di migrasi inkremental. GRANT app_rw
-- otomatis via ALTER DEFAULT PRIVILEGES (00004) — tak ada GRANT manual.
--
-- `status` (Draft/Review/Published): KEPUTUSAN EKSPLISIT USER (2026-08-13) —
-- docs/crm/skema.md §6h semula menulis Draft/Published/Archived, TAPI
-- docs/crm/sistem-dan-role.md §6.10 DAN wireframe Penpot (board "CS — 6.10
-- Knowledge Base") keduanya independen menunjukkan Terbit/Review/Draf (badge
-- warna beda utk 3 status, tanpa Archived). User memilih ikut wireframe+role
-- doc: Draft/Review/Published. "Archived" TIDAK diimplementasi slice ini —
-- kalau dibutuhkan nanti, tambah via migrasi baru (DROP+ADD CHECK, pola
-- 00013/00014/00015), jangan sunting CHECK ini in-place setelah production.
--
-- `visibility` (Public/Internal/Portal Only) ADA di skema.md tapi TAK muncul
-- di wireframe (kolom tabel maupun filter tab) — field form-only, disiapkan
-- utk Portal v2 (portal-facing view SENGAJA ditunda v1, lihat plan A3).
-- Default 'Internal' (aman: artikel baru tak otomatis publik ke portal).
--
-- `author_id` = metadata "siapa penulis" (BEDA dari audit created_by/
-- updated_by yang menjawab "siapa yang menyunting baris DB"; author_id bisa
-- diperlukan kalau suatu saat artikel diimpor/ditulis atas nama orang lain).
-- Slice ini set author_id = created_by otomatis saat create (tak ada picker
-- di form — wireframe tak menyediakan UI reassign penulis). `view_count`
-- auto-increment & total-votes utk persentase "Membantu %" SENGAJA di luar
-- scope (lihat queries/kb_articles.sql & docs/crm/tasks.md catatan A3).
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS kb_articles (
    id             BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id      BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    article_title  TEXT NOT NULL,
    article_body   TEXT,
    category       TEXT,
    keywords       TEXT,
    status         TEXT NOT NULL DEFAULT 'Draft',
    visibility     TEXT NOT NULL DEFAULT 'Internal',
    author_id      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    view_count     INTEGER NOT NULL DEFAULT 0,
    helpful_votes  INTEGER NOT NULL DEFAULT 0,
    created_by     BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by     BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT kb_articles_status_chk CHECK
        (status IN ('Draft','Review','Published')),
    CONSTRAINT kb_articles_visibility_chk CHECK
        (visibility IN ('Public','Internal','Portal Only'))
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_kb_articles_status   ON kb_articles (tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_kb_articles_category ON kb_articles (tenant_id, category);
-- +goose StatementEnd


-- ── RLS — isolasi WORKSPACE (pola verbatim 00005/00012/00017) ──────────────
-- ENABLE + FORCE (pemilik tabel pun terkena policy). Policy dua-GUC: GUC
-- absen → NOL baris (fail-closed), sama dgn seluruh tabel Modul 6.
-- +goose StatementBegin
ALTER TABLE kb_articles ENABLE ROW LEVEL SECURITY;
ALTER TABLE kb_articles FORCE  ROW LEVEL SECURITY;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE POLICY tenant_isolation ON kb_articles
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS kb_articles;
-- +goose StatementEnd
