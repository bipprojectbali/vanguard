-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- MODUL 6 CUSTOMER SUCCESS (slice B1) — tabel `customer_success` (6.1 Health +
-- 6.2 Lifecycle/Journey + 6.2.1 Onboarding + 6.4 Adoption/Usage). SATU baris
-- per desa, digabung dari 4 sub-bagian wireframe karena semuanya snapshot
-- kondisi TERKINI satu account — bukan riwayat (§docs/crm/skema.md §6a).
--
-- TABEL 1:1-DENGAN-`accounts` PERTAMA di codebase ini (beda dari `contacts`
-- yang 1:N-dengan-"paling banyak satu primary"): `account_id` UNIQUE
-- non-partial (bukan partial-unique seperti contacts_one_primary_per_account).
-- Baris dibuat SAAT user pertama kali menyimpan form (get-then-branch di
-- handler, tak ada `ON CONFLICT` — tak ada precedent di codebase, dan Update
-- toh butuh baca baris existing dulu untuk masking F2 per-section).
--
-- `overall_health_score` DIHITUNG APLIKASI (rata-rata 4 komponen non-NULL:
-- adoption/engagement/support/sentiment, dibulatkan) — keputusan v1 sederhana
-- karena skema.md tak beri rumus pasti; boleh direvisi tanpa migrasi (kolom
-- tetap SMALLINT biasa, bukan generated column, supaya rumus bisa berubah di
-- Go tanpa DDL).
--
-- `days_in_stage` BUKAN kolom — dihitung Go saat baca dari `stage_entry_date`
-- (gotcha #14 CLAUDE.md: ekspresi tanggal `CURRENT_DATE - col` di SELECT list
-- bikin sqlc emit `interface{}` yang tak bisa dipakai). `assigned_csm` BUKAN
-- kolom — sudah ada di `accounts.assigned_csm` (authoritative), join, jangan
-- duplikasi.
--
-- TANPA soft-delete: baris ikut hidup account induk (`ON DELETE CASCADE`),
-- sejalan RLS satu-satunya pengurung (pola sama kb_articles/playbooks/
-- sla_policies — TANPA F3 ownership sendiri, kepemilikan diwariskan account).
-- `usage_data_source` v1 SELALU 'Manual' (tak ada field form) — kolom
-- disiapkan utk v2 integrasi product telemetry (lihat docs/crm/tasks.md).
-- ═══════════════════════════════════════════════════════════════════════════

-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS customer_success (
    id                      BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id               BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    account_id              BIGINT NOT NULL UNIQUE REFERENCES accounts(id) ON DELETE CASCADE,

    -- Health (6.1) — 4 komponen input manual/semi + overall dihitung app.
    overall_health_score    SMALLINT,
    health_status           TEXT,
    adoption_score          SMALLINT,
    engagement_score        SMALLINT,
    support_score           SMALLINT,
    sentiment_score         SMALLINT,
    score_trend             TEXT,
    health_last_calculated  TIMESTAMPTZ,

    -- Lifecycle (6.2)
    lifecycle_stage         TEXT,
    stage_entry_date        DATE,

    -- Onboarding (6.2.1)
    onboarding_status       TEXT,
    kickoff_date            DATE,
    target_go_live_date     DATE,
    actual_go_live_date     DATE,
    onboarding_progress     SMALLINT,

    -- Adoption / Usage (6.4)
    last_login_date         DATE,
    active_users            INTEGER,
    login_frequency         TEXT,
    feature_adoption_rate   NUMERIC(5,2),
    key_features_used       TEXT,
    usage_trend             TEXT,
    usage_data_source       TEXT NOT NULL DEFAULT 'Manual',

    created_by              BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by              BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT cs_overall_health_score_chk   CHECK (overall_health_score   IS NULL OR overall_health_score   BETWEEN 0 AND 100),
    CONSTRAINT cs_adoption_score_chk         CHECK (adoption_score         IS NULL OR adoption_score         BETWEEN 0 AND 100),
    CONSTRAINT cs_engagement_score_chk       CHECK (engagement_score       IS NULL OR engagement_score       BETWEEN 0 AND 100),
    CONSTRAINT cs_support_score_chk          CHECK (support_score          IS NULL OR support_score          BETWEEN 0 AND 100),
    CONSTRAINT cs_sentiment_score_chk        CHECK (sentiment_score        IS NULL OR sentiment_score        BETWEEN 0 AND 100),
    CONSTRAINT cs_onboarding_progress_chk    CHECK (onboarding_progress    IS NULL OR onboarding_progress    BETWEEN 0 AND 100),
    CONSTRAINT cs_feature_adoption_rate_chk  CHECK (feature_adoption_rate  IS NULL OR feature_adoption_rate  BETWEEN 0 AND 100),
    CONSTRAINT cs_health_status_chk CHECK
        (health_status IS NULL OR health_status IN ('Healthy','At-Risk','Critical')),
    CONSTRAINT cs_score_trend_chk CHECK
        (score_trend IS NULL OR score_trend IN ('Improving','Stable','Declining')),
    CONSTRAINT cs_lifecycle_stage_chk CHECK
        (lifecycle_stage IS NULL OR lifecycle_stage IN ('Onboarding','Adoption','Retention','Renewal','Advocacy')),
    CONSTRAINT cs_onboarding_status_chk CHECK
        (onboarding_status IS NULL OR onboarding_status IN ('Not Started','In Progress','Completed','Stalled')),
    CONSTRAINT cs_login_frequency_chk CHECK
        (login_frequency IS NULL OR login_frequency IN ('Daily','Weekly','Monthly','Rarely','Inactive')),
    CONSTRAINT cs_usage_trend_chk CHECK
        (usage_trend IS NULL OR usage_trend IN ('Increasing','Stable','Decreasing')),
    CONSTRAINT cs_usage_data_source_chk CHECK
        (usage_data_source IN ('Manual','Product Telemetry'))
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_cs_tenant_account   ON customer_success (tenant_id, account_id);
CREATE INDEX        IF NOT EXISTS idx_cs_health_status    ON customer_success (tenant_id, health_status);
CREATE INDEX        IF NOT EXISTS idx_cs_lifecycle_stage  ON customer_success (tenant_id, lifecycle_stage);
-- +goose StatementEnd


-- ── RLS — isolasi WORKSPACE (pola verbatim 00005/00012/00017/00018) ────────
-- ENABLE + FORCE (pemilik tabel pun terkena policy). Policy dua-GUC: GUC
-- absen → NOL baris (fail-closed), sama dgn seluruh tabel Modul 6.
-- +goose StatementBegin
ALTER TABLE customer_success ENABLE ROW LEVEL SECURITY;
ALTER TABLE customer_success FORCE  ROW LEVEL SECURITY;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE POLICY tenant_isolation ON customer_success
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS customer_success;
-- +goose StatementEnd
