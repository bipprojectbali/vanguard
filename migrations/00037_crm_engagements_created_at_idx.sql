-- 00037 — indeks (tenant_id, created_at DESC, id DESC) untuk engagements.
--
-- KENAPA: BL-41 menaikkan linimasa terpadu BL-31 ke halaman Activities global
-- (/activity-log). Lengan CS di feed itu (ListEngagementsFeed) diurut & di-keyset
-- pada (created_at DESC, id DESC) — sumbu "kapan DICATAT" yang SAMA dengan
-- activities, agar merge-sort lintas-tabel di handler konsisten (rasional sumbu
-- di ADR 0010 & tasks BL-41). Indeks lama idx_engagements_tenant_scheduled hanya
-- melayani sumbu scheduled_at (ListEngagements di /engagements); tanpa indeks ini
-- feed global memaksa sort penuh partisi tenant (langgar rule 13: index kolom
-- ORDER BY/keyset). Aditif — tak menyentuh DDL/behavior yang ada.
--
-- +goose Up
CREATE INDEX IF NOT EXISTS idx_engagements_tenant_created
    ON engagements (tenant_id, created_at DESC, id DESC);

-- +goose Down
DROP INDEX IF EXISTS idx_engagements_tenant_created;
