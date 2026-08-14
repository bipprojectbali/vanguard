-- 00023_crm_cs_success_plans.sql — Success Plans (CRM Modul 6 slice 6.3).
-- Satu success plan mewakili rencana tertulis yang dibuat CSM untuk desa:
-- tujuan, metrik sukses, target tanggal, persentase progress, dan status siklus
-- hidup (Draft/Active/Achieved/At-Risk/Cancelled).
--
-- F3 ownership: Admin/Manager (data_scope='all') lihat semua; CSM (data_scope='own')
-- lihat plan untuk desa yang ia bina (account_id → accounts:
-- account_owner/assigned_csm/backup_csm). Sales tidak punya crm:success_plans di
-- policy → fail-closed di Casbin F2. Support sama.
-- RLS di sini hanya mengisolasi antar-WORKSPACE (tenant_id).

-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS success_plans (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id       BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    account_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,

    -- Judul plan (wajib).
    plan_name       TEXT NOT NULL,

    -- Tujuan utama dan cara mengukur keberhasilan.
    objective       TEXT,
    success_metric  TEXT,

    -- Tanggal target selesai (DATE → simpan tanpa timezone).
    target_date     DATE,

    -- Status siklus hidup plan; cermin label dari spec 6.3.
    plan_status     TEXT NOT NULL DEFAULT 'Draft'
        CHECK (plan_status IN ('Draft', 'Active', 'Achieved', 'At-Risk', 'Cancelled')),

    -- Progress persentase 0–100 (diisi manual oleh CSM).
    progress        SMALLINT NOT NULL DEFAULT 0
        CHECK (progress >= 0 AND progress <= 100),

    -- CSM pemilik plan (nullable = belum ditugaskan eksplisit).
    owner_csm       BIGINT REFERENCES users(id) ON DELETE SET NULL,

    created_by      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Soft-delete: plan dibatalkan dapat disembunyikan tanpa hapus data historis.
    deleted_at      TIMESTAMPTZ
);

ALTER TABLE success_plans ENABLE ROW LEVEL SECURITY;
ALTER TABLE success_plans FORCE  ROW LEVEL SECURITY;

-- Pola dua GUC (is_super + tenant_id) sama persis dengan tabel CRM lain.
DROP POLICY IF EXISTS tenant_isolation ON success_plans;
CREATE POLICY tenant_isolation ON success_plans
    USING (
        COALESCE(current_setting('app.is_super', true), 'off') = 'on'
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint
    )
    WITH CHECK (
        COALESCE(current_setting('app.is_super', true), 'off') = 'on'
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint
    );

-- Indeks: keyset pagination (created_at DESC, id DESC) + FK lookup + scan status.
CREATE INDEX IF NOT EXISTS idx_success_plans_tenant_created
    ON success_plans (tenant_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_success_plans_account
    ON success_plans (account_id)
    WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_success_plans_tenant_status
    ON success_plans (tenant_id, plan_status)
    WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_success_plans_owner
    ON success_plans (owner_csm)
    WHERE owner_csm IS NOT NULL AND deleted_at IS NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON success_plans TO app_rw;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS success_plans;
-- +goose StatementEnd
