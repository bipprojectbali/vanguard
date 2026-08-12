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
