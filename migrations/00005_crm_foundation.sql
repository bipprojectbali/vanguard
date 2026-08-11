-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- FONDASI CRM Desa+ — sumbu role bisnis + tabel inti (accounts/contacts) +
-- master (plans/sla_policies).
--
-- Ini migrasi INKREMENTAL (00005), bukan penyatuan 00001: repo sudah punya
-- 00001–00004 dan mungkin sudah ter-clone, jadi tabel CRM lahir di file baru.
-- Dokumen skema menyebut "00002_crm_schema.sql" karena ditulis saat baru ada
-- 00001; nomornya menyesuaikan keadaan repo, bukan sebaliknya.
--
-- Cakupan sengaja DIBATASI ke fondasi (keputusan F0): tabel yang dipakai lintas
-- modul (accounts=HUB, contacts) + master tanpa FK maju (plans, sla_policies).
-- leads/deals/quotes/subscriptions/CS/activities menyusul per modul di migrasi
-- berikutnya (§11 skema) — supaya satu file tak jadi god-migration.
--
-- Urutan mengikuti ketergantungan FK: memberships (ALTER) → plans → accounts
-- (self-FK nullable) → contacts (→ accounts) → sla_policies. RLS dipasang di
-- akhir, per tabel (RLS tak diwariskan lewat ALTER DEFAULT PRIVILEGES).
-- ═══════════════════════════════════════════════════════════════════════════


-- ── Sumbu role bisnis (F1) ─────────────────────────────────────────────────
-- Role CRM (admin/manager/sales/csm/support) disimpan PER-KEANGGOTAAN, bukan
-- per-user: orang yang sama bisa Manager di satu workspace, CSM di workspace
-- lain. Ortogonal dari memberships.role (owner/admin/member) dan role platform —
-- keputusan bypass RLS TETAP dari role platform, tak pernah dari sumbu ini.
--
-- NULL = belum diberi peran CRM (mis. owner platform yang bukan tim penjualan).
-- Nullable, sama pola dengan users.workspace_quota. Perangkapan (satu orang dua
-- role) ditolak di handler, bukan constraint DB.
-- +goose StatementBegin
ALTER TABLE memberships
    ADD COLUMN IF NOT EXISTS business_role TEXT;
-- +goose StatementEnd

-- +goose StatementBegin
-- CHECK dipisah + guard IF NOT EXISTS: ADD CONSTRAINT tak punya IF NOT EXISTS
-- di Postgres, jadi dibungkus DO agar migrasi aman diulang.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'memberships_business_role_chk'
    ) THEN
        ALTER TABLE memberships
            ADD CONSTRAINT memberships_business_role_chk
            CHECK (business_role IS NULL OR business_role IN
                   ('admin','manager','sales','csm','support'));
    END IF;
END $$;
-- +goose StatementEnd


-- ── plans — katalog master (tenant-scoped) ─────────────────────────────────
-- Master milik workspace Desa+. Tanpa soft-delete: is_active=false = pensiun
-- (harga historis di quote/subscription memakai snapshot, bukan referensi hidup).
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS plans (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id         BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    plan_name         TEXT NOT NULL,
    plan_code         TEXT NOT NULL,                    -- SKU
    description       TEXT,
    plan_category     TEXT NOT NULL,                    -- Core/Add-on/Module
    is_active         BOOLEAN NOT NULL DEFAULT true,
    base_price        NUMERIC(15,2),
    billing_frequency TEXT,                             -- Monthly/Annual
    setup_fee         NUMERIC(15,2),
    currency          TEXT NOT NULL DEFAULT 'IDR',
    included_features TEXT,
    created_by        BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by        BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT plans_category_chk CHECK (plan_category IN ('Core','Add-on','Module'))
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_plans_code   ON plans (tenant_id, plan_code);
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS        idx_plans_active ON plans (tenant_id, is_active);
-- +goose StatementEnd


-- ── accounts — Desa (HUB) ──────────────────────────────────────────────────
-- Pusat sistem: semua modul lain menyambung ke sini. "Desa" = DATA (baris),
-- BUKAN tenant — tenant tetap workspace Desa+. RLS mengisolasi antar-workspace;
-- isolasi antar-desa (ownership Sales/CSM) ditegakkan di layer aplikasi (F3),
-- bukan RLS lapis kedua.
--
-- account_owner/assigned_csm/backup_csm OTORITATIF di hub (bukan rollup): filter
-- `WHERE account_owner=$uid OR assigned_csm=$uid OR backup_csm=$uid` dipakai
-- lintas modul, jadi kolom di sini menghindari JOIN berulang.
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS accounts (
    id                     BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id              BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    -- Ownership (otoritatif di hub). SET NULL: desa tetap ada bila stafnya keluar.
    account_owner          BIGINT REFERENCES users(id) ON DELETE SET NULL,
    assigned_csm           BIGINT REFERENCES users(id) ON DELETE SET NULL,
    backup_csm             BIGINT REFERENCES users(id) ON DELETE SET NULL,
    -- Identitas
    village_name           TEXT NOT NULL,
    village_code           TEXT,                         -- Kode Kemendagri; nullable (prospek awal)
    account_type           TEXT NOT NULL,                -- prospect/customer/former_customer
    parent_account_id      BIGINT REFERENCES accounts(id) ON DELETE SET NULL,  -- self-FK, opsional v1
    website                TEXT,
    description            TEXT,
    -- Alamat & geo (region = kolom teks, bukan master table — lihat keputusan skema)
    province               TEXT,
    regency                TEXT,
    district               TEXT,
    village_address        TEXT,
    postal_code            TEXT,
    latitude               NUMERIC(10,7),
    longitude              NUMERIC(10,7),
    territory              TEXT,
    -- Profil desa
    village_status         TEXT,                         -- Desa/Kelurahan/Nagari/Gampong
    village_classification TEXT,                         -- IDM: Mandiri/Maju/...
    population             INTEGER,
    hamlets_count          INTEGER,
    village_budget         NUMERIC(15,2),                -- APBDes
    contact_phone          TEXT,
    office_phone           TEXT,
    office_email           TEXT,
    -- Soft-delete + audit
    deleted_at             TIMESTAMPTZ,
    created_by             BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by             BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT accounts_type_chk CHECK
        (account_type IN ('prospect','customer','former_customer')),
    CONSTRAINT accounts_village_status_chk CHECK
        (village_status IS NULL OR village_status IN
         ('Desa','Kelurahan','Nagari','Gampong')),
    CONSTRAINT accounts_classification_chk CHECK
        (village_classification IS NULL OR village_classification IN
         ('Mandiri','Maju','Berkembang','Tertinggal','Sangat Tertinggal'))
);
-- +goose StatementEnd

-- Guard duplikat desa: village_code unik per tenant, hanya baris hidup.
-- Tabrakan ketahuan di DB saat konversi Lead, bukan jadi desa ganda.
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_accounts_code ON accounts (tenant_id, village_code)
    WHERE village_code IS NOT NULL AND deleted_at IS NULL;
-- +goose StatementEnd
-- Index utama list (keyset created_at DESC, id DESC) — partial pada baris hidup.
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_accounts_live ON accounts (tenant_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_accounts_owner ON accounts (tenant_id, account_owner)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_accounts_csm ON accounts (tenant_id, assigned_csm)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_accounts_region ON accounts (tenant_id, province, regency, district)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd


-- ── contacts — Kontak (perangkat desa) ─────────────────────────────────────
-- 1:N ke accounts. Mewarisi kepemilikan desa induk (F3 tak memfilter contacts
-- sendiri — ikut account_id-nya). email_opt_out/do_not_contact = flag kontrol
-- WRITABLE (koreksi label spec 3.E: bukan rollup) — kontak bisa minta berhenti
-- dihubungi.
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS contacts (
    id                   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id            BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    account_id           BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    contact_owner        BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reports_to_id        BIGINT REFERENCES contacts(id) ON DELETE SET NULL,  -- self-FK
    first_name           TEXT NOT NULL,
    last_name            TEXT,
    salutation           TEXT,                          -- Bapak/Ibu/Sdr
    job_title            TEXT,                          -- bebas ("Kepala Desa")
    position_category    TEXT,                          -- Kepala Desa/Sekdes/Kaur/...
    contact_role         TEXT,                          -- Decision Maker/Influencer/...
    is_primary_contact   BOOLEAN NOT NULL DEFAULT false,
    is_technical_contact BOOLEAN NOT NULL DEFAULT false,
    term_period          TEXT,                          -- masa jabatan ("2021–2027")
    mobile_phone         TEXT,
    whatsapp_number      TEXT,                          -- kanal utama
    office_phone         TEXT,
    email                TEXT,
    preferred_channel    TEXT,                          -- WhatsApp/Telepon/Email/Kunjungan
    mailing_address      TEXT,
    city                 TEXT,
    postal_code          TEXT,
    email_opt_out        BOOLEAN NOT NULL DEFAULT false, -- writable (kontrol manual)
    do_not_contact       BOOLEAN NOT NULL DEFAULT false, -- writable
    deleted_at           TIMESTAMPTZ,
    created_by           BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by           BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT contacts_position_chk CHECK
        (position_category IS NULL OR position_category IN
         ('Kepala Desa','Sekdes','Kaur','Kasi','Operator','Bendahara','BPD','Lainnya')),
    CONSTRAINT contacts_role_chk CHECK
        (contact_role IS NULL OR contact_role IN
         ('Decision Maker','Influencer','User','Finance','Gatekeeper')),
    CONSTRAINT contacts_channel_chk CHECK
        (preferred_channel IS NULL OR preferred_channel IN
         ('WhatsApp','Telepon','Email','Kunjungan'))
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_contacts_account ON contacts (tenant_id, account_id)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_contacts_live ON contacts (tenant_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd
-- Maks 1 kontak utama per desa (partial unique).
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_contacts_primary ON contacts (tenant_id, account_id)
    WHERE is_primary_contact AND deleted_at IS NULL;
-- +goose StatementEnd


-- ── sla_policies — master (M6) ─────────────────────────────────────────────
-- Master SLA dipasang lebih awal (tanpa FK maju) supaya tickets (modul M6) bisa
-- merujuknya tanpa migrasi tambahan. Tanpa soft-delete: is_active=false = pensiun.
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS sla_policies (
    id                          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id                   BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    sla_name                    TEXT NOT NULL,
    applies_to_priority         TEXT,
    first_response_target_hours INTEGER,
    resolution_target_hours     INTEGER,
    business_hours              TEXT,
    escalation_rule             TEXT,
    is_active                   BOOLEAN NOT NULL DEFAULT true,
    created_by                  BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by                  BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_sla_active ON sla_policies (tenant_id, is_active);
-- +goose StatementEnd


-- ── RLS — per tabel (tak diwariskan) ───────────────────────────────────────
-- ENABLE + FORCE: FORCE = pemilik tabel pun terkena policy; tanpanya owner
-- bypass diam-diam dan seluruh jaring hanya dekorasi. GRANT app_rw sudah
-- otomatis lewat ALTER DEFAULT PRIVILEGES (00004) — tak perlu GRANT manual.
-- +goose StatementBegin
ALTER TABLE plans        ENABLE ROW LEVEL SECURITY;
ALTER TABLE plans        FORCE  ROW LEVEL SECURITY;
ALTER TABLE accounts     ENABLE ROW LEVEL SECURITY;
ALTER TABLE accounts     FORCE  ROW LEVEL SECURITY;
ALTER TABLE contacts     ENABLE ROW LEVEL SECURITY;
ALTER TABLE contacts     FORCE  ROW LEVEL SECURITY;
ALTER TABLE sla_policies ENABLE ROW LEVEL SECURITY;
ALTER TABLE sla_policies FORCE  ROW LEVEL SECURITY;
-- +goose StatementEnd

-- +goose StatementBegin
-- Policy dua-GUC IDENTIK template: app.is_super='on' (jalur platform) ATAU
-- tenant_id cocok. current_setting(...,true) → NULL bila GUC absen; NULLIF/
-- COALESCE membuat "GUC absen" berarti TAK ADA BARIS, bukan semua baris.
CREATE POLICY tenant_isolation ON plans
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
CREATE POLICY tenant_isolation ON accounts
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
CREATE POLICY tenant_isolation ON contacts
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
CREATE POLICY tenant_isolation ON sla_policies
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS sla_policies;
DROP TABLE IF EXISTS contacts;
DROP TABLE IF EXISTS accounts;
DROP TABLE IF EXISTS plans;
ALTER TABLE memberships DROP CONSTRAINT IF EXISTS memberships_business_role_chk;
ALTER TABLE memberships DROP COLUMN IF EXISTS business_role;
-- +goose StatementEnd
