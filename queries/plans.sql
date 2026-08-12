-- plans.sql — katalog master (plans), dibaca untuk Quote Builder (Modul 4 Sales).
-- Isolasi WORKSPACE ditegakkan RLS (GUC app.tenant_id di WithTenant); tak ada
-- filter tenant_id manual. plans TANPA soft-delete: is_active=false = pensiun
-- (harga historis di quote memakai SNAPSHOT quote_items.unit_price, bukan referensi
-- hidup ke sini). Query ini menopang picker plan + baca base_price saat add item.

-- name: ListPlans :many
-- Plan aktif untuk picker item quote, urut nama. Hanya is_active=true: plan pensiun
-- tak boleh dijual baru (tapi quote lama tetap sah lewat snapshot). Bounded katalog
-- master per-workspace → tanpa keyset.
SELECT * FROM plans
WHERE is_active = true
ORDER BY plan_name ASC, id ASC;

-- name: GetPlan :one
-- Satu plan (untuk baca base_price yang di-SNAPSHOT ke quote_items.unit_price saat
-- add item). RLS menjamin tenant_id. Tak filter is_active: handler memutuskan (add
-- item baru gate ke ListPlans; get by id juga menutup balapan pensiun-saat-submit).
SELECT * FROM plans
WHERE id = sqlc.arg(id);


-- ── Katalog master CRUD (M5-2) — kelola plan di /plans ───────────────────────
-- plans TANPA soft-delete: is_active=false = pensiun (harga historis aman via
-- snapshot quote_items). SetPlanActive = pensiunkan/aktifkan; UpdatePlan menyunting
-- profil (is_active punya jalur sendiri agar pensiun terlihat sebagai aksi khusus).

-- name: ListPlansAll :many
-- Seluruh katalog untuk tampilan kelola (TERMASUK yang pensiun) — beda dari
-- ListPlans (hanya aktif, untuk picker quote). Aktif dulu lalu urut nama. Bounded
-- katalog master per-workspace → tanpa keyset.
SELECT * FROM plans
ORDER BY is_active DESC, plan_name ASC, id ASC;

-- name: CreatePlan :one
-- Buat plan katalog. tenant_id eksplisit (RLS WITH CHECK memverifikasinya = GUC).
-- plan_code unik per tenant (idx_plans_code) → kode kembar ditolak DB. is_active
-- default true di skema tapi di-set eksplisit agar handler bisa membuat draft pensiun.
INSERT INTO plans (
    tenant_id, plan_name, plan_code, description, plan_category,
    is_active, base_price, billing_frequency, setup_fee, currency,
    included_features, created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(plan_name), sqlc.arg(plan_code), sqlc.narg(description),
    sqlc.arg(plan_category), sqlc.arg(is_active), sqlc.narg(base_price),
    sqlc.narg(billing_frequency), sqlc.narg(setup_fee), sqlc.arg(currency),
    sqlc.narg(included_features), sqlc.narg(created_by)
)
RETURNING *;

-- name: UpdatePlan :one
-- Sunting profil plan. is_active TAK di sini (SetPlanActive) — pensiun/aktifkan
-- adalah aksi tersendiri, bukan efek samping edit. plan_code boleh diubah (tetap
-- tunduk idx_plans_code unik).
UPDATE plans SET
    plan_name         = sqlc.arg(plan_name),
    plan_code         = sqlc.arg(plan_code),
    description       = sqlc.narg(description),
    plan_category     = sqlc.arg(plan_category),
    base_price        = sqlc.narg(base_price),
    billing_frequency = sqlc.narg(billing_frequency),
    setup_fee         = sqlc.narg(setup_fee),
    currency          = sqlc.arg(currency),
    included_features = sqlc.narg(included_features),
    updated_by        = sqlc.narg(updated_by),
    updated_at        = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: SetPlanActive :exec
-- Pensiunkan (false) atau aktifkan kembali (true) plan. Plan pensiun hilang dari
-- ListPlans (picker) tapi quote/langganan lama tetap sah (snapshot harga).
UPDATE plans SET
    is_active  = sqlc.arg(is_active),
    updated_by = sqlc.narg(updated_by),
    updated_at = now()
WHERE id = sqlc.arg(id);
