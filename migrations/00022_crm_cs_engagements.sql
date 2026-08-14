-- 00022_crm_cs_engagements.sql — Engagements / Check-ins (CRM Modul 6 slice 6.5).
-- Satu engagement mewakili satu interaksi / sesi touch point antara CSM dan desa.
-- Tipe interaksi: touch_point (rutin), qbr (quarterly business review),
-- onboarding_call, escalation, check_in. Status: planned/done/skipped/rescheduled.
--
-- F3 ownership: Admin/Manager (data_scope='all') lihat semua; CSM (data_scope='own')
-- lihat engagement untuk desa yang ia bina (JOIN accounts → account_owner/assigned_csm/
-- backup_csm). Sales tidak punya crm:engagements di policy → fail-closed di Casbin F2.
-- Support tidak punya crm:engagements → fail-closed juga.
-- RLS di sini hanya mengisolasi antar-WORKSPACE (tenant_id).

-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS engagements (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id       BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    account_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,

    subject         TEXT NOT NULL,

    -- Tipe interaksi (picklist); nilai cermin label dari spec 6.5.
    engagement_type TEXT NOT NULL DEFAULT 'check_in'
        CHECK (engagement_type IN ('touch_point', 'qbr', 'onboarding_call', 'escalation', 'check_in')),

    -- Ritme / frekuensi (label konteks v1, belum dikaitkan ke logic otomasi).
    frequency       TEXT
        CHECK (frequency IS NULL OR frequency IN ('weekly', 'monthly', 'quarterly', 'ad_hoc')),

    -- Jadwal interaksi (TIMESTAMPTZ → simpan UTC, tampilkan pakai appTZ).
    scheduled_at    TIMESTAMPTZ NOT NULL,

    -- Status siklus hidup engagement.
    status          TEXT NOT NULL DEFAULT 'planned'
        CHECK (status IN ('planned', 'done', 'skipped', 'rescheduled')),

    -- Channel komunikasi.
    channel         TEXT
        CHECK (channel IS NULL OR channel IN ('whatsapp', 'call', 'video', 'site_visit')),

    -- Ringkasan hasil interaksi; diisi setelah status = 'done'.
    outcome         TEXT,

    -- Jadwal touch point berikutnya (DATE).
    next_due_date   DATE,

    -- CSM pelaksana (nullable = belum ditugaskan eksplisit).
    owner_id        BIGINT REFERENCES users(id) ON DELETE SET NULL,

    created_by      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE engagements ENABLE ROW LEVEL SECURITY;
ALTER TABLE engagements FORCE  ROW LEVEL SECURITY;

-- Pola dua GUC (is_super + tenant_id) sama persis dengan tabel CRM lain.
-- is_super memungkinkan WithSuper bypass untuk audit dan maintenance.
DROP POLICY IF EXISTS tenant_isolation ON engagements;
CREATE POLICY tenant_isolation ON engagements
    USING (
        COALESCE(current_setting('app.is_super', true), 'off') = 'on'
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint
    )
    WITH CHECK (
        COALESCE(current_setting('app.is_super', true), 'off') = 'on'
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint
    );

-- Indeks utama: keyset pagination (scheduled_at DESC, id DESC) + FK lookup.
CREATE INDEX IF NOT EXISTS idx_engagements_tenant_scheduled ON engagements (tenant_id, scheduled_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_engagements_account          ON engagements (account_id);
CREATE INDEX IF NOT EXISTS idx_engagements_owner            ON engagements (owner_id)
    WHERE owner_id IS NOT NULL;
-- Indeks bantu: scan status planned (tampilan kalender / reminder).
CREATE INDEX IF NOT EXISTS idx_engagements_tenant_status    ON engagements (tenant_id, status)
    WHERE status = 'planned';

GRANT SELECT, INSERT, UPDATE, DELETE ON engagements TO app_rw;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS engagements;
-- +goose StatementEnd
