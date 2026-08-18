-- 00025_crm_cs_trainings.sql — Training Schedule (CRM Modul 6, sub-item
-- Onboarding 6.2.1.2). Satu baris = satu sesi training/pelatihan (mis.
-- "Training Admin Desa", "Pelatihan Modul Keuangan") untuk satu desa.
--
-- F2: gerbang REUSE objek Casbin "crm:journey" (Journey/Onboarding) — sama
-- dgn cs_impl_tasks, sub-fitur onboarding yang sama. F3 ownership: Admin/
-- Manager (data_scope='all') lihat semua; CSM (data_scope='own') lihat
-- training desa yang ia bina (JOIN accounts → account_owner/assigned_csm/
-- backup_csm). Sales/Support tak punya crm:journey di policy → fail-closed
-- di Casbin F2. RLS di sini hanya mengisolasi antar-WORKSPACE (tenant_id).

-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS cs_trainings (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id       BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    account_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,

    training_topic  TEXT NOT NULL,

    -- Jadwal training (TIMESTAMPTZ → simpan UTC, tampilkan pakai appTZ).
    training_date   TIMESTAMPTZ NOT NULL,

    -- Pelatih pelaksana (nullable = belum ditugaskan eksplisit).
    trainer_id      BIGINT REFERENCES users(id) ON DELETE SET NULL,

    participants    INTEGER,

    -- Status siklus hidup training (picklist); nilai snake_case, label Title
    -- Case di layer helper.
    training_status TEXT NOT NULL DEFAULT 'scheduled'
        CHECK (training_status IN ('scheduled', 'completed', 'rescheduled', 'cancelled')),

    -- Persentase kehadiran; diisi setelah status = 'completed'.
    attendance      NUMERIC(5, 2),

    created_by      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE cs_trainings ENABLE ROW LEVEL SECURITY;
ALTER TABLE cs_trainings FORCE  ROW LEVEL SECURITY;

-- Pola dua GUC (is_super + tenant_id) sama persis dengan tabel CRM lain.
-- is_super memungkinkan WithSuper bypass untuk audit dan maintenance.
DROP POLICY IF EXISTS tenant_isolation ON cs_trainings;
CREATE POLICY tenant_isolation ON cs_trainings
    USING (
        COALESCE(current_setting('app.is_super', true), 'off') = 'on'
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint
    )
    WITH CHECK (
        COALESCE(current_setting('app.is_super', true), 'off') = 'on'
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint
    );

-- Indeks utama: keyset pagination (training_date DESC, id DESC) + FK lookup.
CREATE INDEX IF NOT EXISTS idx_cs_trainings_tenant_account ON cs_trainings (tenant_id, account_id);
-- Indeks bantu: keyset by training_date (jadwal kalender).
CREATE INDEX IF NOT EXISTS idx_cs_trainings_tenant_date    ON cs_trainings (tenant_id, training_date);

GRANT SELECT, INSERT, UPDATE, DELETE ON cs_trainings TO app_rw;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS cs_trainings;
-- +goose StatementEnd
