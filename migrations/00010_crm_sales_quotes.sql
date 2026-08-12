-- +goose Up

-- ═══════════════════════════════════════════════════════════════════════════
-- MODUL 4 SALES (slice 2) — tabel penawaran: `quotes` lalu `quote_items`.
--
-- Migrasi INKREMENTAL (00010): melanjutkan slice 1 (00009 = leads+deals). Quote
-- hanya merujuk ke belakang — deals (00009), accounts/plans/users (00005) — semua
-- sudah ada, jadi TAK bergantung Modul 5 (subscriptions). Ini menutup acceptance
-- M4 "harga quote beku walau plan berubah": quote_items menyimpan unit_price &
-- subtotal sebagai SNAPSHOT (bukan JOIN ke plans saat baca).
--
-- Quote di-NEST di bawah deal pada layer aplikasi (/deals/{id}/quotes): deal_id
-- populated di alur ini walau kolomnya nullable (skema §4c). account_id NN =
-- jangkar sesungguhnya (deal boleh hilang, quote tetap milik account → deal_id
-- ON DELETE SET NULL). Ownership F3 DIWARISI dari deal (deal_owner via
-- loadOwnedDeal) — TAK ada kolom quote_owner.
--
-- quote_items = anak quote: hard-delete lewat ON DELETE CASCADE (tanpa soft-delete/
-- audit). tenant_id KOLOM SENDIRI (bukan lewat quote) karena FORCE RLS deny-default
-- butuh tenant_id langsung di tiap tabel untuk policy-nya sendiri.
--
-- RLS = isolasi WORKSPACE (tenant_id) saja, pola verbatim dari 00005/00009.
-- GRANT app_rw otomatis via ALTER DEFAULT PRIVILEGES (00004) — tak ada GRANT manual.
-- ═══════════════════════════════════════════════════════════════════════════


-- ── quotes — header penawaran (§4c) ────────────────────────────────────────
-- Dibuat lebih dulu agar quote_items.quote_id bisa FK langsung. quote_number
-- disimpan di entity_code (QUO-001) — konsisten dgn accounts/leads/deals, dialokasi
-- GenerateEntityCode DALAM tx create. grand_total/tax_amount = snapshot yang
-- dihitung ulang app saat item berubah (bukan agregat live saat baca).
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS quotes (
    id                 BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id          BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    entity_code        TEXT,                          -- QUO-001 (= quote_number)
    -- Relasi inti. deal_id nullable (skema §4c) tapi selalu terisi di alur nest;
    -- SET NULL: quote tetap milik account bila deal-nya dihapus. account_id NN =
    -- jangkar sesungguhnya.
    deal_id            BIGINT REFERENCES deals(id) ON DELETE SET NULL,
    account_id         BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    -- Identitas & status
    quote_name         TEXT,
    quote_status       TEXT NOT NULL DEFAULT 'Draft',
    expiration_date    DATE,
    payment_terms      TEXT,
    notes_terms        TEXT,
    prepared_by        BIGINT REFERENCES users(id) ON DELETE SET NULL,
    -- Total (snapshot; dihitung ulang app dari quote_items + pajak)
    grand_total        NUMERIC(15,2),
    tax_amount         NUMERIC(15,2),
    -- Soft-delete + audit
    deleted_at         TIMESTAMPTZ,
    created_by         BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by         BIGINT REFERENCES users(id) ON DELETE SET NULL,
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT quotes_status_chk CHECK
        (quote_status IN ('Draft','Sent','Under Review','Accepted','Rejected','Expired'))
);
-- +goose StatementEnd

-- entity_code unik per tenant (hanya baris hidup & berkode) — mirror deals.
-- +goose StatementBegin
CREATE UNIQUE INDEX IF NOT EXISTS idx_quotes_entity_code ON quotes (tenant_id, entity_code)
    WHERE entity_code IS NOT NULL AND deleted_at IS NULL;
-- +goose StatementEnd
-- Index utama list keyset (created_at DESC, id DESC) — partial baris hidup.
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_quotes_live ON quotes (tenant_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd
-- Quote per deal (detail deal → daftar quote-nya).
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_quotes_deal ON quotes (tenant_id, deal_id)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd
-- Quote per account (jangkar sesungguhnya).
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_quotes_account ON quotes (tenant_id, account_id)
    WHERE deleted_at IS NULL;
-- +goose StatementEnd


-- ── quote_items — baris penawaran (§4d) ────────────────────────────────────
-- Anak quote (ON DELETE CASCADE). tenant_id kolom sendiri untuk RLS langsung.
-- unit_price & subtotal = SNAPSHOT: disalin dari plans.base_price saat item dibuat
-- dan tak pernah ikut berubah bila katalog plan berubah kemudian (acceptance M4).
-- subtotal dihitung app (unit_price * quantity * (1 - discount_pct/100)).
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS quote_items (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    quote_id      BIGINT NOT NULL REFERENCES quotes(id) ON DELETE CASCADE,
    tenant_id     BIGINT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    plan_id       BIGINT REFERENCES plans(id) ON DELETE SET NULL,
    quantity      INTEGER NOT NULL DEFAULT 1,
    unit_price    NUMERIC(15,2) NOT NULL,        -- snapshot
    discount_pct  NUMERIC(5,2),
    subtotal      NUMERIC(15,2) NOT NULL,        -- snapshot (dihitung app)
    line_no       SMALLINT,
    CONSTRAINT quote_items_quantity_chk CHECK (quantity > 0),
    CONSTRAINT quote_items_discount_chk CHECK
        (discount_pct IS NULL OR discount_pct BETWEEN 0 AND 100)
);
-- +goose StatementEnd

-- Baris per quote, urut tampil (line_no) — juga penopang JOIN quote→items.
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS idx_quote_items_quote ON quote_items (quote_id, line_no);
-- +goose StatementEnd


-- ── RLS — per tabel (tak diwariskan) ───────────────────────────────────────
-- ENABLE + FORCE: FORCE = pemilik tabel pun terkena policy. Policy dua-GUC IDENTIK
-- template 00005/00009: app.is_super='on' (jalur platform) ATAU tenant_id cocok;
-- GUC absen → NOL baris (fail-closed). quote_items pakai tenant_id KOLOM SENDIRI.
-- +goose StatementBegin
ALTER TABLE quotes      ENABLE ROW LEVEL SECURITY;
ALTER TABLE quotes      FORCE  ROW LEVEL SECURITY;
ALTER TABLE quote_items ENABLE ROW LEVEL SECURITY;
ALTER TABLE quote_items FORCE  ROW LEVEL SECURITY;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE POLICY tenant_isolation ON quotes
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
CREATE POLICY tenant_isolation ON quote_items
    USING (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint)
    WITH CHECK (COALESCE(current_setting('app.is_super', true),'off')='on'
           OR tenant_id = NULLIF(current_setting('app.tenant_id', true),'')::bigint);
-- +goose StatementEnd


-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS quote_items;
DROP TABLE IF EXISTS quotes;
-- +goose StatementEnd
