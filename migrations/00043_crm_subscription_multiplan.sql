-- 00043_crm_subscription_multiplan.sql — BL-88 PR2b: multi-plan langganan AKTIF.
-- Menutup poin 6 ADR 0011: identitas paket PINDAH sepenuhnya ke subscription_items.
--
-- Dua perubahan menyatu:
--   1. subscriptions.plan_id → NULLABLE. Quote multi-plan → parent plan_id NULL (identitas
--      di item). Single-plan tetap boleh mengisi plan_id sbg kenyamanan, TAPI laporan &
--      invarian tak lagi bergantung padanya (laporan agregasi via item, invarian di item).
--   2. Invarian "1 langganan Active per (tenant,account,plan)" PINDAH dari
--      subscriptions (idx_subs_one_active, di-DROP) ke subscription_items.
--
-- KENAPA bukan sekadar unique index di subscription_items: invarian menjangkau DUA tabel
-- (item.plan_id + status/account_id PARENT). Unique index tak bisa JOIN. Solusi: denormalisasi
-- account_id + flag parent_active ke item (diturunkan server via trigger, aplikasi tak bisa
-- salah isi), lalu partial unique index nyata di atasnya — SAMA race-proof-nya dgn index lama
-- (index yang menyerialkan, bukan cek prosedural yang bocor di READ COMMITTED).

-- +goose Up

-- 1. Parent plan_id boleh NULL (multi-plan → identitas di item). Idempotent alami.
ALTER TABLE subscriptions ALTER COLUMN plan_id DROP NOT NULL;

-- 2. Kolom turunan di item untuk menopang partial unique index lintas-tabel.
--    account_id: dedup per-desa (beda desa boleh punya paket sama). parent_active: status
--    parent yg RELEVAN utk invarian (Active & belum terhapus). Keduanya DITURUNKAN dari
--    parent oleh trigger — bukan diisi aplikasi (mustahil salah/lupa).
ALTER TABLE subscription_items ADD COLUMN IF NOT EXISTS account_id BIGINT REFERENCES accounts(id) ON DELETE CASCADE;
ALTER TABLE subscription_items ADD COLUMN IF NOT EXISTS parent_active BOOLEAN NOT NULL DEFAULT false;

-- Backfill kolom turunan utk baris yang sudah ada (item hasil backfill 00042).
UPDATE subscription_items si
   SET account_id   = s.account_id,
       parent_active = (s.status = 'Active' AND s.deleted_at IS NULL)
  FROM subscriptions s
 WHERE s.id = si.subscription_id
   AND (si.account_id IS DISTINCT FROM s.account_id
        OR si.parent_active IS DISTINCT FROM (s.status = 'Active' AND s.deleted_at IS NULL));

ALTER TABLE subscription_items ALTER COLUMN account_id SET NOT NULL;

-- Trigger 1: turunkan account_id + parent_active dari parent SAAT item lahir. Won membuat
-- parent Active LALU menyisipkan item → parent_active harus benar seketika di INSERT (bukan
-- menunggu UPDATE parent yang tak akan datang).
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION sync_subscription_item_parent() RETURNS trigger AS $$
BEGIN
    SELECT s.account_id, (s.status = 'Active' AND s.deleted_at IS NULL)
      INTO NEW.account_id, NEW.parent_active
      FROM subscriptions s WHERE s.id = NEW.subscription_id;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- Trigger 2: saat status/deleted_at PARENT berubah (Trial→Active, renew Expired, approve,
-- soft-delete), sinkronkan parent_active semua item-nya. Perubahan menjadi Active memicu
-- pengecekan partial unique index → melanggar = tx abort (penjaga keras balapan).
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION resync_subscription_items_active() RETURNS trigger AS $$
BEGIN
    UPDATE subscription_items
       SET parent_active = (NEW.status = 'Active' AND NEW.deleted_at IS NULL)
     WHERE subscription_id = NEW.id
       AND parent_active IS DISTINCT FROM (NEW.status = 'Active' AND NEW.deleted_at IS NULL);
    RETURN NULL;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trg_sync_subscription_item_parent ON subscription_items;
CREATE TRIGGER trg_sync_subscription_item_parent
    BEFORE INSERT ON subscription_items
    FOR EACH ROW EXECUTE FUNCTION sync_subscription_item_parent();

DROP TRIGGER IF EXISTS trg_resync_subscription_items_active ON subscriptions;
CREATE TRIGGER trg_resync_subscription_items_active
    AFTER UPDATE OF status, deleted_at ON subscriptions
    FOR EACH ROW
    WHEN (OLD.status IS DISTINCT FROM NEW.status OR OLD.deleted_at IS DISTINCT FROM NEW.deleted_at)
    EXECUTE FUNCTION resync_subscription_items_active();

-- 3. Invarian pindah ke item: SATU item Active per (tenant,account,plan). NULL plan_id
--    (baris tak berpaket) & item parent non-Active tak dihitung. Menggantikan idx_subs_one_active.
CREATE UNIQUE INDEX IF NOT EXISTS idx_subscription_items_one_active
    ON subscription_items (tenant_id, account_id, plan_id)
    WHERE parent_active AND plan_id IS NOT NULL;

DROP INDEX IF EXISTS idx_subs_one_active;

-- +goose Down

DROP INDEX IF EXISTS idx_subscription_items_one_active;

CREATE UNIQUE INDEX IF NOT EXISTS idx_subs_one_active
    ON subscriptions (tenant_id, account_id, plan_id)
    WHERE status = 'Active' AND deleted_at IS NULL;

DROP TRIGGER IF EXISTS trg_resync_subscription_items_active ON subscriptions;
DROP TRIGGER IF EXISTS trg_sync_subscription_item_parent ON subscription_items;
DROP FUNCTION IF EXISTS resync_subscription_items_active();
DROP FUNCTION IF EXISTS sync_subscription_item_parent();

ALTER TABLE subscription_items DROP COLUMN IF EXISTS parent_active;
ALTER TABLE subscription_items DROP COLUMN IF EXISTS account_id;

ALTER TABLE subscriptions ALTER COLUMN plan_id SET NOT NULL;
