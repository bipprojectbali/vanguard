-- 00042_crm_subscription_items.sql — BL-88 PR2a: baris langganan (mirror quote_items).
-- Satu langganan dapat berisi banyak paket. Anak subscriptions (ON DELETE CASCADE),
-- tenant_id kolom SENDIRI untuk RLS langsung (tak diwariskan). unit_price & subtotal =
-- SNAPSHOT komersial (disalin dari quote_items saat Closed Won); mrr/arr per-item.
--
-- PR2a = TULIS GANDA: subscriptions.plan_id MASIH terisi & NOT NULL (belum nullable) →
-- laporan/JOIN plans lama utuh, make check hijau. plan_id jadi nullable + identitas paket
-- pindah sepenuhnya ke item = PR2b. Backfill di bawah mengisi 1 item per langganan LAMA
-- (idempotent) agar agregasi via item bisa diandalkan penuh di PR2b. ADR 0011.

-- +goose Up
-- +goose StatementBegin

CREATE TABLE IF NOT EXISTS subscription_items (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    subscription_id BIGINT NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    tenant_id       BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    plan_id         BIGINT REFERENCES plans(id) ON DELETE SET NULL,
    quantity        INTEGER NOT NULL DEFAULT 1,
    unit_price      NUMERIC(15,2) NOT NULL,        -- snapshot
    discount_pct    NUMERIC(5,2),
    subtotal        NUMERIC(15,2) NOT NULL,        -- snapshot (dihitung app)
    mrr             NUMERIC(15,2),                 -- per-item
    arr             NUMERIC(15,2),                 -- per-item
    line_no         SMALLINT,
    CONSTRAINT subscription_items_quantity_chk CHECK (quantity > 0),
    CONSTRAINT subscription_items_discount_chk CHECK
        (discount_pct IS NULL OR discount_pct BETWEEN 0 AND 100)
);

ALTER TABLE subscription_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE subscription_items FORCE  ROW LEVEL SECURITY;

-- Pola dua GUC (is_super + tenant_id) sama persis quote_items/tabel CRM lain.
-- is_super memungkinkan WithSuper bypass untuk audit dan maintenance.
DROP POLICY IF EXISTS tenant_isolation ON subscription_items;
CREATE POLICY tenant_isolation ON subscription_items
    USING (
        COALESCE(current_setting('app.is_super', true), 'off') = 'on'
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint
    )
    WITH CHECK (
        COALESCE(current_setting('app.is_super', true), 'off') = 'on'
        OR tenant_id = NULLIF(current_setting('app.tenant_id', true), '')::bigint
    );

-- Baris per langganan, urut tampil (line_no) — juga penopang JOIN subscription→items.
CREATE INDEX IF NOT EXISTS idx_subscription_items_sub ON subscription_items (subscription_id, line_no);

GRANT SELECT, INSERT, UPDATE, DELETE ON subscription_items TO app_rw;

-- Backfill (idempotent): 1 item per langganan LAMA yang belum punya item, diturunkan dari
-- agregat langganan itu sendiri. unit_price/subtotal legacy = mrr×termin (COALESCE menjaga
-- NOT NULL bila mrr/termin kosong) — presisi tak kritis; perannya "≥1 item ada" + agregasi
-- item PR2b. WHERE NOT EXISTS → aman diulang (goose re-run / clone ulang).
INSERT INTO subscription_items (subscription_id, tenant_id, plan_id, quantity,
    unit_price, subtotal, mrr, arr, line_no)
SELECT s.id, s.tenant_id, s.plan_id, COALESCE(s.quantity_seats, 1),
    COALESCE(s.mrr, 0), COALESCE(s.mrr, 0) * COALESCE(s.contract_term_months, 1),
    s.mrr, s.arr, 1
FROM subscriptions s
WHERE s.plan_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM subscription_items si WHERE si.subscription_id = s.id);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS subscription_items;
-- +goose StatementEnd
