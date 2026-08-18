-- 00024_crm_cs_impl_tasks.sql — Implementation Tracker (CRM Modul 6, sub-item
-- Onboarding 6.2.1.1). Satu baris = satu task checklist implementasi/onboarding
-- untuk satu desa (mis. "Setup akun admin", "Migrasi data awal").
--
-- F2: gerbang REUSE objek Casbin "crm:journey" (Journey/Onboarding) — bukan
-- objek baru, task ini sub-fitur onboarding yang sama dgn kolom onboarding_*
-- di customer_success. F3 ownership: Admin/Manager (data_scope='all') lihat
-- semua; CSM (data_scope='own') lihat task desa yang ia bina (JOIN accounts →
-- account_owner/assigned_csm/backup_csm). Sales/Support tak punya crm:journey
-- di policy → fail-closed di Casbin F2. RLS di sini hanya mengisolasi
-- antar-WORKSPACE (tenant_id).

-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS cs_impl_tasks (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id       BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    account_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,

    task_name       TEXT NOT NULL,

    -- Status checklist (picklist); nilai snake_case, label Title Case di layer helper.
    task_status     TEXT NOT NULL DEFAULT 'to_do'
        CHECK (task_status IN ('to_do', 'in_progress', 'done', 'blocked')),

    -- Penanggung jawab task (nullable = belum ditugaskan eksplisit).
    owner_id        BIGINT REFERENCES users(id) ON DELETE SET NULL,

    due_date        DATE,

    created_by      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE cs_impl_tasks ENABLE ROW LEVEL SECURITY;
ALTER TABLE cs_impl_tasks FORCE  ROW LEVEL SECURITY;

-- Pola dua GUC (is_super + tenant_id) sama persis dengan tabel CRM lain.
-- is_super memungkinkan WithSuper bypass untuk audit dan maintenance.
DROP POLICY IF EXISTS tenant_isolation ON cs_impl_tasks;
CREATE POLICY tenant_isolation ON cs_impl_tasks
    USING (
        COALESCE(current_setting('app.is_super', true), 'off') = 'on'
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint
    )
    WITH CHECK (
        COALESCE(current_setting('app.is_super', true), 'off') = 'on'
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint
    );

-- Indeks utama: keyset pagination (due_date DESC, id DESC) + FK lookup.
CREATE INDEX IF NOT EXISTS idx_cs_impl_tasks_tenant_account ON cs_impl_tasks (tenant_id, account_id);
-- Indeks bantu: scan per status (mis. daftar "blocked" untuk eskalasi).
CREATE INDEX IF NOT EXISTS idx_cs_impl_tasks_tenant_status  ON cs_impl_tasks (tenant_id, task_status);

GRANT SELECT, INSERT, UPDATE, DELETE ON cs_impl_tasks TO app_rw;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS cs_impl_tasks;
-- +goose StatementEnd
