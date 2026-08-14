-- 00020_crm_cs_tickets.sql — Tiket / Kasus layanan desa (CRM Modul 6 slice B2,
-- wireframe 6.9 "Tickets / Cases"). Satu tiket mewakili satu permintaan dukungan
-- dari desa ke tim Support. SLA dilacak per prioritas: deadline dihitung saat
-- create (created_at + resolution_target_minutes) dan DISIMPAN — tidak mereferensi
-- kebijakan SLA secara langsung karena target dapat berubah setelah tiket dibuat
-- (prinsip snapshot, sama dengan quotes).
--
-- F3 ownership: Support melihat SEMUA tiket (data_scope='none' dikecualikan di
-- layer aplikasi lewat TicketsListFilterFor, bukan di sini). CSM/Sales melihat
-- hanya tiket desa binaan mereka (JOIN accounts → assigned_csm/account_owner).
-- RLS di sini hanya mengisolasi antar-WORKSPACE (tenant_id), sama dengan tabel CRM
-- lainnya.

-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS tickets (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id       BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    account_id      BIGINT NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,

    subject         TEXT NOT NULL,
    description     TEXT,

    priority        TEXT NOT NULL DEFAULT 'sedang'
        CHECK (priority IN ('rendah', 'sedang', 'tinggi')),
    status          TEXT NOT NULL DEFAULT 'baru'
        CHECK (status IN ('baru', 'ditugaskan', 'eskalasi', 'selesai')),

    -- Agen yang menangani tiket ini (nullable = belum ditugaskan).
    assigned_to     BIGINT REFERENCES users(id) ON DELETE SET NULL,

    -- Snapshot SLA: policy + deadline dihitung dan disimpan saat create, BUKAN
    -- referensi hidup ke sla_policies. Target yang berlaku saat tiket dibuat
    -- adalah yang dipertanggungjawabkan — kebijakan boleh diubah sesudahnya.
    sla_policy_id   BIGINT REFERENCES sla_policies(id) ON DELETE SET NULL,
    sla_deadline_at TIMESTAMPTZ, -- = created_at + resolution_target_minutes * '1 minute'

    -- Ditentukan saat status bergerak ke 'selesai'.
    resolved_at     TIMESTAMPTZ,

    created_by      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by      BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

ALTER TABLE tickets ENABLE ROW LEVEL SECURITY;
ALTER TABLE tickets FORCE  ROW LEVEL SECURITY;

-- Pola dua GUC (is_super + tenant_id) sama persis dengan tabel CRM lain (accounts,
-- customer_success, dll). is_super memungkinkan WithSuper bypass untuk audit dan
-- maintenance. DROP IF EXISTS: migration ini di-rename dari 00020 ke 00021
-- (duplikat nomor dengan 00020_crm_accounts_dupe_index); policy mungkin sudah
-- ada di DB dev yang sebelumnya menjalankannya sebagai v20.
DROP POLICY IF EXISTS tenant_isolation ON tickets;
CREATE POLICY tenant_isolation ON tickets
    USING (
        COALESCE(current_setting('app.is_super', true), 'off') = 'on'
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint
    )
    WITH CHECK (
        COALESCE(current_setting('app.is_super', true), 'off') = 'on'
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint
    );

-- Indeks: list daftar (created_at DESC keyset), SLA scan aktif, dan FK lookup.
CREATE INDEX IF NOT EXISTS idx_tickets_tenant_created ON tickets (tenant_id, created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_tickets_tenant_sla     ON tickets (tenant_id, sla_deadline_at)
    WHERE sla_deadline_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tickets_account        ON tickets (account_id);
CREATE INDEX IF NOT EXISTS idx_tickets_assigned       ON tickets (assigned_to)
    WHERE assigned_to IS NOT NULL;

GRANT SELECT, INSERT, UPDATE, DELETE ON tickets TO app_rw;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS tickets;
-- +goose StatementEnd
