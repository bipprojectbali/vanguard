-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- MODUL 5 SUBSCRIPTIONS (slice 1) — tabel `subscriptions` (inti + renewal +
-- churn + renewal-action). `plans` (katalog master) sudah ada sejak 00005 dengan
-- index unik (tenant,plan_code) + (tenant,is_active) — tak diubah di sini.
--
-- KEPUTUSAN skema (dikunci di docs/crm/skema.md §5):
--   • Renewal = INSERT baris BARU (previous_subscription_id self-FK + previous_value
--     snapshot), BUKAN update — riwayat periode tak ditimpa.
--   • ARR DISIMPAN terpisah dari MRR (billing Annual/Multi-year bisa diskon → ARR≠MRR×12).
--   • Banyak langganan aktif paralel per desa, tapi SATU active per (account,plan) —
--     dijaga idx_subs_one_active (partial unique).
--   • subscription_number memakai kolom `entity_code` (SUB-xxx via GenerateEntityCode),
--     SERAGAM dgn deals/quotes (00009/00010); nullable + unik partial (kode diisi handler,
--     nullable menutup celah bila codegen belum jalan — pola sama dgn tabel Sales).
--
-- deals.created_subscription_id (00009, BIGINT tanpa FK) DITUTUP di sini: FK menyusul
-- setelah subscriptions lahir (persis rencana di komentar 00009).
--
-- Ownership antar-desa (subscription_owner) ditegakkan di layer aplikasi (F3), BUKAN
-- RLS lapis kedua — sama keputusan dgn accounts/deals. RLS di sini hanya isolasi
-- WORKSPACE (tenant_id), pola verbatim 00005/00009/00010. GRANT app_rw otomatis via
-- ALTER DEFAULT PRIVILEGES (00004) — tak ada GRANT manual.
-- ═══════════════════════════════════════════════════════════════════════════


-- ── subscriptions — inti langganan + renewal(5.2) + churn(5.4) + action(6.6) ──
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS subscriptions (
    id                        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id                 BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    entity_code               TEXT,                        -- SUB-001 (= subscription_number)
    -- Ownership (otoritatif di langganan). SET NULL: langganan tetap ada bila staf keluar.
    subscription_owner        BIGINT REFERENCES users(id) ON DELETE SET NULL,
    -- Relasi inti
    account_id                BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    plan_id                   BIGINT NOT NULL REFERENCES plans(id) ON DELETE RESTRICT,
    source_deal_id            BIGINT REFERENCES deals(id) ON DELETE SET NULL,
    -- Rantai renewal: baris ini adalah renewal dari previous_subscription_id.
    -- SET NULL: putus tautan bila periode lama terhapus, baris ini tetap sah.
    previous_subscription_id  BIGINT REFERENCES subscriptions(id) ON DELETE SET NULL,
    -- Status & periode (5.B)
    status                    TEXT NOT NULL DEFAULT 'Trial',
    start_date                DATE,
    end_date                  DATE,
    billing_cycle             TEXT,                        -- Monthly/Quarterly/Annual/Multi-year
    auto_renew                BOOLEAN NOT NULL DEFAULT false,
    contract_term_months      INTEGER,
    -- Nilai (5.C). ARR disimpan (bukan MRR×12) — diskon annual.
    mrr                       NUMERIC(15,2),
    arr                       NUMERIC(15,2),
    quantity_seats            INTEGER,
    discount_pct              NUMERIC(5,2),
    payment_status            TEXT,                        -- Paid/Pending/Overdue/Partial
    -- Renewal (5.2) — field menempel, bukan objek baru
    renewal_status            TEXT,                        -- Upcoming/In Progress/Renewed/Not Renewed/At Risk
    renewal_type              TEXT,                        -- Auto/Manual/Upsell/Downgrade
    renewal_owner             BIGINT REFERENCES users(id) ON DELETE SET NULL,
    renewal_quote_id          BIGINT REFERENCES quotes(id) ON DELETE SET NULL,
    previous_value            NUMERIC(15,2),               -- snapshot nilai periode lalu (deteksi upsell/downgrade)
    -- Renewal action (6.6) — DATA milik 5.2, AKSI milik CS
    renewal_stage             TEXT,                        -- Not Started/Outreach/Negotiation/Won/Lost
    renewal_risk              TEXT,                        -- Low/Medium/High
    renewal_action_plan       TEXT,
    renewal_next_action_date  DATE,
    -- Churn (5.4) — diisi saat status Cancelled/Churned
    cancellation_date         DATE,
    churn_reason              TEXT,                        -- Budget/No Adoption/Change of Leadership/Competitor/Dissatisfaction/Feature Gap
    churn_type                TEXT,                        -- Voluntary/Involuntary
    churn_notes               TEXT,
    lost_value_mrr            NUMERIC(15,2),
    win_back_eligible         BOOLEAN,
    -- Soft-delete + audit
    deleted_at                TIMESTAMPTZ,
    created_by                BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by                BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- CHECK domain nilai (nullable → IS NULL OR IN). status NN → IN langsung.
    CONSTRAINT subs_status_chk CHECK (
        status IN ('Trial','Active','Suspended','Expired','Cancelled','Churned')),
    CONSTRAINT subs_billing_cycle_chk CHECK (
        billing_cycle IS NULL OR billing_cycle IN ('Monthly','Quarterly','Annual','Multi-year')),
    CONSTRAINT subs_payment_status_chk CHECK (
        payment_status IS NULL OR payment_status IN ('Paid','Pending','Overdue','Partial')),
    CONSTRAINT subs_discount_pct_chk CHECK (
        discount_pct IS NULL OR discount_pct BETWEEN 0 AND 100),
    CONSTRAINT subs_renewal_status_chk CHECK (
        renewal_status IS NULL OR renewal_status IN
            ('Upcoming','In Progress','Renewed','Not Renewed','At Risk')),
    CONSTRAINT subs_renewal_type_chk CHECK (
        renewal_type IS NULL OR renewal_type IN ('Auto','Manual','Upsell','Downgrade')),
    CONSTRAINT subs_renewal_stage_chk CHECK (
        renewal_stage IS NULL OR renewal_stage IN
            ('Not Started','Outreach','Negotiation','Won','Lost')),
    CONSTRAINT subs_renewal_risk_chk CHECK (
        renewal_risk IS NULL OR renewal_risk IN ('Low','Medium','High')),
    CONSTRAINT subs_churn_reason_chk CHECK (
        churn_reason IS NULL OR churn_reason IN
            ('Budget','No Adoption','Change of Leadership','Competitor','Dissatisfaction','Feature Gap')),
    CONSTRAINT subs_churn_type_chk CHECK (
        churn_type IS NULL OR churn_type IN ('Voluntary','Involuntary'))
);
-- +goose StatementEnd

-- Nomor langganan unik per tenant (nullable → partial; kode diisi handler).
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_subs_number
    ON subscriptions (tenant_id, entity_code) WHERE entity_code IS NOT NULL;
-- +goose StatementEnd

-- Daftar langganan per desa (list-by-account), sembunyikan yang terhapus.
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_subs_account
    ON subscriptions (tenant_id, account_id) WHERE deleted_at IS NULL;
-- +goose StatementEnd

-- Jatuh tempo renewal (Menu 6.6): hanya Active yang punya end_date relevan.
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_subs_renewal
    ON subscriptions (tenant_id, end_date) WHERE status = 'Active' AND deleted_at IS NULL;
-- +goose StatementEnd

-- Telusuri rantai renewal balik ke periode lama.
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_subs_prev
    ON subscriptions (previous_subscription_id) WHERE previous_subscription_id IS NOT NULL;
-- +goose StatementEnd

-- Invarian bisnis: SATU langganan Active per (tenant,account,plan). Renewal harus
-- meng-Expired baris lama SEBELUM meng-Active baris baru, atau index ini menolaknya.
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_subs_one_active
    ON subscriptions (tenant_id, account_id, plan_id)
    WHERE status = 'Active' AND deleted_at IS NULL;
-- +goose StatementEnd


-- ── Tutup FK deals.created_subscription_id (ditunda dari 00009) ─────────────
-- Postgres tak punya ADD CONSTRAINT IF NOT EXISTS → guard via katalog (idempotent
-- untuk re-run produksi, rule 4). ON DELETE SET NULL: hapus langganan tak menghapus
-- deal historis, hanya melepas tautan.
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'deals_created_subscription_fk'
    ) THEN
        ALTER TABLE deals
            ADD CONSTRAINT deals_created_subscription_fk
            FOREIGN KEY (created_subscription_id) REFERENCES subscriptions(id)
            ON DELETE SET NULL;
    END IF;
END $$;
-- +goose StatementEnd


-- ── RLS — isolasi WORKSPACE (pola verbatim 00005/00009/00010) ───────────────
-- ENABLE + FORCE (pemilik tabel pun terkena policy). Policy dua-GUC: app.is_super='on'
-- (jalur platform) ATAU tenant_id cocok; GUC absen → NOL baris (fail-closed).
-- +goose StatementBegin
ALTER TABLE subscriptions ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscriptions FORCE  ROW LEVEL SECURITY;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE POLICY tenant_isolation ON subscriptions
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
ALTER TABLE deals DROP CONSTRAINT IF EXISTS deals_created_subscription_fk;
-- +goose StatementEnd
-- +goose StatementBegin
DROP TABLE IF EXISTS subscriptions;
-- +goose StatementEnd
