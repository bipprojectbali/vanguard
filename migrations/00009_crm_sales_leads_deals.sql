-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- MODUL 4 SALES (slice 1) — tabel funnel penjualan: `deals` lalu `leads`.
--
-- Migrasi INKREMENTAL (00009): fondasi CRM (accounts/contacts/plans) sudah ada di
-- 00005, entity_code di 00006, sumbu role bisnis + objek Casbin crm:leads/crm:deals
-- di 00007. Di sini hanya dua tabel transaksi Sales.
--
-- URUTAN dibalik dari §11 skema (leads dulu, deals kemudian) menjadi deals→leads:
-- FK melingkar leads↔deals (skema §11) diputus dengan membuat `deals` LEBIH DULU
-- (deals tak merujuk leads), sehingga leads.converted_deal_id → deals bisa FK
-- langsung tanpa ALTER ADD CONSTRAINT terpisah di akhir. deals sendiri hanya
-- merujuk ke belakang (accounts/contacts/plans — semuanya sudah ada di 00005).
--
-- CAKUPAN slice ini SENGAJA leads+deals saja; quotes/quote_items ditunda ke dekat
-- Modul 5 (bergantung katalog plans lebih dalam). Karena itu deals.plan_requested_id
-- → plans AMAN (plans ada), tapi deals.created_subscription_id = BIGINT NULL TANPA
-- FK (tabel subscriptions belum ada) — FK menyusul Modul 5.
--
-- Ownership antar-desa (F3: lead_owner/deal_owner) ditegakkan di layer aplikasi
-- (internal/db/ownership.go), BUKAN RLS lapis kedua — sama keputusan dgn accounts.
-- RLS di sini hanya isolasi WORKSPACE (tenant_id), pola verbatim dari 00005.
-- GRANT app_rw otomatis via ALTER DEFAULT PRIVILEGES (00004) — tak ada GRANT manual.
-- ═══════════════════════════════════════════════════════════════════════════


-- ── deals — pipeline penjualan (§4b) ───────────────────────────────────────
-- Dibuat lebih dulu agar leads.converted_deal_id bisa FK langsung. account_id NN
-- (deal selalu menempel ke satu desa; lead yang belum jadi account tak punya deal).
-- Ownership deal ada di KOLOM SENDIRI (deal_owner), bukan warisan account — sales
-- yang menutup deal boleh berbeda dari owner desa (skema §4).
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS deals (
    id                      BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id               BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    entity_code             TEXT,                          -- DEAL-001 (lookup internal)
    -- Ownership (otoritatif di deal). SET NULL: deal tetap ada bila staf keluar.
    deal_owner              BIGINT REFERENCES users(id) ON DELETE SET NULL,
    -- Relasi inti
    account_id              BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    primary_contact_id      BIGINT REFERENCES contacts(id) ON DELETE SET NULL,
    plan_requested_id       BIGINT REFERENCES plans(id) ON DELETE SET NULL,
    -- Identitas & tipe
    deal_name               TEXT NOT NULL,
    deal_type               TEXT,                          -- New Business/Renewal/Upsell/Cross-sell
    -- Pipeline
    stage                   TEXT NOT NULL DEFAULT 'Prospecting',
    amount                  NUMERIC(15,2),
    probability             SMALLINT,                      -- 0–100
    expected_close_date     DATE,
    forecast_category       TEXT,
    next_step               TEXT,
    -- Hasil (Closed Won/Lost)
    closed_date             DATE,
    win_loss_reason         TEXT,
    competitor              TEXT,
    loss_notes              TEXT,
    -- Produk & langganan
    subscription_term       TEXT,                          -- Monthly/Annual/Multi-year
    -- Hasil Closed Won: FK ke subscriptions DITUNDA (tabel belum ada; TODO Modul 5).
    -- Kolom BIGINT NULL tanpa constraint agar konversi/closed-won bisa mengisinya
    -- kelak tanpa migrasi ubah tipe — hanya ADD CONSTRAINT saat subscriptions lahir.
    created_subscription_id BIGINT,
    -- Soft-delete + audit
    deleted_at              TIMESTAMPTZ,
    created_by              BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by              BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT deals_type_chk CHECK
        (deal_type IS NULL OR deal_type IN
         ('New Business','Renewal','Upsell','Cross-sell')),
    CONSTRAINT deals_stage_chk CHECK
        (stage IN ('Prospecting','Qualification','Demo','Proposal',
                   'Negotiation','Closed Won','Closed Lost')),
    CONSTRAINT deals_probability_chk CHECK
        (probability IS NULL OR probability BETWEEN 0 AND 100),
    CONSTRAINT deals_term_chk CHECK
        (subscription_term IS NULL OR subscription_term IN
         ('Monthly','Annual','Multi-year'))
);
-- +goose StatementEnd

-- entity_code unik per tenant (hanya baris hidup & berkode) — mirror accounts.
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_deals_entity_code ON deals (tenant_id, entity_code)
    WHERE entity_code IS NOT NULL AND deleted_at IS NULL;
-- +goose StatementEnd
-- Index utama list keyset (created_at DESC, id DESC) — partial baris hidup.
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_deals_live ON deals (tenant_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd
-- Pipeline per-stage (kanban + agregasi forecast §8.1).
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_deals_stage ON deals (tenant_id, stage, created_at DESC)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd
-- Ownership-filter F3 (deal_owner = uid).
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_deals_owner ON deals (tenant_id, deal_owner)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd
-- Deal per desa (detail account → daftar deal-nya).
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_deals_account ON deals (tenant_id, account_id)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd


-- ── leads — entity flat, dikonversi (§4a) ──────────────────────────────────
-- Lead = entity terpisah dengan region/kontak MENTAH (text), BUKAN account.
-- Konversi (Salesforce-style) membuat account+contact+deal atomik lalu menautkan
-- kembali lewat converted_* (nullable → accounts/contacts/deals; SET NULL agar
-- hapus hasil konversi tak menghancurkan jejak lead-nya).
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS leads (
    id                   BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id            BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    entity_code          TEXT,                             -- LEAD-001 (lookup internal)
    lead_owner           BIGINT REFERENCES users(id) ON DELETE SET NULL,
    -- Identitas & kontak mentah
    lead_name            TEXT NOT NULL,                    -- nama desa/lead
    contact_person       TEXT,
    job_title            TEXT,
    lead_source          TEXT,                             -- picklist bebas
    -- Kualifikasi
    lead_status          TEXT NOT NULL DEFAULT 'New',
    rating               TEXT,                             -- Hot/Warm/Cold
    unqualified_reason   TEXT,                             -- termasuk Duplicate
    estimated_value      NUMERIC(15,2),
    -- Region & kontak mentah (belum jadi account)
    province             TEXT,
    regency              TEXT,
    district             TEXT,
    mobile_phone         TEXT,
    whatsapp             TEXT,
    email                TEXT,
    -- Hasil konversi
    converted            BOOLEAN NOT NULL DEFAULT false,
    converted_account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
    converted_contact_id BIGINT REFERENCES contacts(id) ON DELETE SET NULL,
    converted_deal_id    BIGINT REFERENCES deals(id) ON DELETE SET NULL,
    converted_at         TIMESTAMPTZ,
    -- Soft-delete + audit
    deleted_at           TIMESTAMPTZ,
    created_by           BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by           BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT leads_status_chk CHECK
        (lead_status IN ('New','Contacted','Qualified','Unqualified','Converted')),
    CONSTRAINT leads_rating_chk CHECK
        (rating IS NULL OR rating IN ('Hot','Warm','Cold'))
);
-- +goose StatementEnd

-- entity_code unik per tenant (hanya baris hidup & berkode) — mirror accounts.
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_leads_entity_code ON leads (tenant_id, entity_code)
    WHERE entity_code IS NOT NULL AND deleted_at IS NULL;
-- +goose StatementEnd
-- Index utama list keyset (created_at DESC, id DESC) — partial baris hidup.
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_leads_live ON leads (tenant_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd
-- Tab per-status (All/My/Unqualified) — filter berindeks.
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_leads_status ON leads (tenant_id, lead_status, created_at DESC)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd
-- Ownership-filter F3 (lead_owner = uid).
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_leads_owner ON leads (tenant_id, lead_owner)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd


-- ── RLS — per tabel (tak diwariskan) ───────────────────────────────────────
-- ENABLE + FORCE: FORCE = pemilik tabel pun terkena policy. Policy dua-GUC IDENTIK
-- template 00005: app.is_super='on' (jalur platform) ATAU tenant_id cocok; GUC
-- absen → NOL baris (fail-closed). GRANT app_rw otomatis (ALTER DEFAULT PRIVILEGES).
-- +goose StatementBegin
ALTER TABLE deals ENABLE ROW LEVEL SECURITY;
ALTER TABLE deals FORCE  ROW LEVEL SECURITY;
ALTER TABLE leads ENABLE ROW LEVEL SECURITY;
ALTER TABLE leads FORCE  ROW LEVEL SECURITY;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE POLICY tenant_isolation ON deals
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
CREATE POLICY tenant_isolation ON leads
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS leads;
DROP TABLE IF EXISTS deals;
-- +goose StatementEnd
