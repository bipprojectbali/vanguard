-- deals.sql — pipeline Sales (Deal). Isolasi WORKSPACE ditegakkan RLS (GUC
-- app.tenant_id di WithTenant); isolasi ANTAR-DESA (F3) ditegakkan di layer query
-- lewat flag ownership di ListDeals/ListDealsForPipeline — lihat ownership.go.
-- Ownership deal memakai SATU kolom (deal_owner).
--
-- entity_code (DEAL-001) dialokasikan di GenerateEntityCode DALAM tx create.
-- account_id NN: deal selalu menempel ke satu desa. Deal hasil konversi lead dibuat
-- di tx konversi (bersama account+contact) — lihat handler.

-- name: CreateDeal :one
-- Buat deal. tenant_id di-set eksplisit (RLS WITH CHECK memverifikasinya = GUC).
INSERT INTO deals (
    tenant_id, entity_code, deal_name, account_id,
    deal_owner, primary_contact_id, plan_requested_id,
    deal_type, stage, amount, probability,
    expected_close_date, forecast_category, next_step, subscription_term,
    created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(entity_code), sqlc.arg(deal_name), sqlc.arg(account_id),
    sqlc.narg(deal_owner), sqlc.narg(primary_contact_id), sqlc.narg(plan_requested_id),
    sqlc.narg(deal_type), sqlc.arg(stage), sqlc.narg(amount), sqlc.narg(probability),
    sqlc.narg(expected_close_date), sqlc.narg(forecast_category),
    sqlc.narg(next_step), sqlc.narg(subscription_term),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: GetDeal :one
-- Satu deal hidup. RLS menjamin tenant_id; ownership diputuskan handler
-- (DealsListFilter.Allows) atas baris.
SELECT * FROM deals
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListDeals :many
-- Daftar deal (tampilan Tabel), keyset (created_at DESC, id DESC) + filter
-- ownership F3 + filter stage opsional. Dua flag ownership (sumber SATU dengan
-- DealsListFilter): scope_all → semua; is_own → deal_owner = uid; keduanya false
-- → NOL baris (fail-closed). stage_filter '' → semua stage.
--
-- Filter tambahan (ortogonal dari ownership, BL-10):
--   mine_only → paksa deal_owner = uid (toggle "Deal Saya", walau aktor scope_all).
-- Cermin mine_only di ListLeads; menyempitkan, tak pernah melebarkan.
--
-- search '' → tak menyaring; selain itu MEMPERSEMPIT (ILIKE substring, case-
-- insensitive) di ATAS ownership+stage — tak pernah melebarkan baris. Hanya kolom
-- tak-tersamar yang tampil di tabel (deal_name + entity_code); nilai ARR tersamar
-- tak dijadikan kunci cari.
SELECT * FROM deals
WHERE deleted_at IS NULL
  AND (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND deal_owner = sqlc.arg(uid))
  )
  AND (NOT sqlc.arg(mine_only)::boolean OR deal_owner = sqlc.arg(uid))
  AND (sqlc.arg(stage_filter)::text = '' OR stage = sqlc.arg(stage_filter)::text)
  AND (
      sqlc.arg(search)::text = ''
      OR deal_name ILIKE '%' || sqlc.arg(search) || '%'
      OR entity_code ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: ListDealsForPipeline :many
-- Papan Kanban: seluruh deal hidup dalam cakupan ownership, diurutkan agar kartu
-- rapi per-stage lalu terbaru dulu. Di-bucket per-stage di handler (bukan N query
-- per kolom). LIMIT membatasi papan agar tak memuat seluruh tabel (guardrail
-- pagination); deal di luar batas tetap terlihat lewat tampilan Tabel berkeyset.
-- mine_only (BL-10) menyaring papan ke deal_owner = uid saat toggle "Deal Saya"
-- aktif — KPI (DealPipelineStats) ikut tersaring agar papan & ringkasan seiring.
SELECT * FROM deals
WHERE deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND deal_owner = sqlc.arg(uid))
  )
  AND (NOT sqlc.arg(mine_only)::boolean OR deal_owner = sqlc.arg(uid))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: DealPipelineStats :one
-- KPI ringkas pipeline dalam cakupan ownership (satu round-trip, bukan hitung di
-- Go atas seluruh baris). COALESCE(...)::bigint/::numeric membungkus agregat agar
-- sqlc tak meng-emit interface{} (gotcha #14). Win rate dihitung di Go dari
-- won_count/(won_count+lost_count) — pembagian nol ditangani di sana.
-- BL-49: filter opsional Periode (created_at) + Owner (deal_owner) untuk Sales
-- Report — guard NULL = tak menyaring (perilaku papan Kanban tak berubah, memang
-- oper nil). owner_filter di-AND DI ATAS predikat scope: hanya bisa MENYEMPIT
-- dalam cakupan yang diizinkan (sales own-scope pilih owner lain → nol baris).
SELECT
    COALESCE(COUNT(*) FILTER (
        WHERE stage NOT IN ('Closed Won','Closed Lost')), 0)::bigint AS open_count,
    COALESCE(SUM(amount) FILTER (
        WHERE stage NOT IN ('Closed Won','Closed Lost')), 0)::numeric AS pipeline_value,
    COALESCE(COUNT(*) FILTER (WHERE stage = 'Closed Won'), 0)::bigint  AS won_count,
    COALESCE(COUNT(*) FILTER (WHERE stage = 'Closed Lost'), 0)::bigint AS lost_count
FROM deals
WHERE deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND deal_owner = sqlc.arg(uid))
  )
  AND (NOT sqlc.arg(mine_only)::boolean OR deal_owner = sqlc.arg(uid))
  AND (sqlc.narg(period_start)::timestamptz IS NULL OR created_at >= sqlc.narg(period_start))
  AND (sqlc.narg(period_end)::timestamptz IS NULL OR created_at < sqlc.narg(period_end))
  AND (sqlc.narg(owner_filter)::bigint IS NULL OR deal_owner = sqlc.narg(owner_filter));

-- name: UpdateDeal :one
-- Sunting profil deal. entity_code, stage, dan hasil (win_loss_reason/closed_date)
-- TAK di sini: stage punya jalur khusus (UpdateDealStage) agar perpindahan pipeline
-- terlihat sebagai aksi tersendiri, bukan efek samping edit.
UPDATE deals SET
    deal_name           = sqlc.arg(deal_name),
    account_id          = sqlc.arg(account_id),
    deal_owner          = sqlc.narg(deal_owner),
    primary_contact_id  = sqlc.narg(primary_contact_id),
    plan_requested_id   = sqlc.narg(plan_requested_id),
    deal_type           = sqlc.narg(deal_type),
    amount              = sqlc.narg(amount),
    probability         = sqlc.narg(probability),
    expected_close_date = sqlc.narg(expected_close_date),
    forecast_category   = sqlc.narg(forecast_category),
    next_step           = sqlc.narg(next_step),
    subscription_term   = sqlc.narg(subscription_term),
    competitor          = sqlc.narg(competitor),
    updated_by          = sqlc.narg(updated_by),
    updated_at          = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: UpdateDealStage :exec
-- Pindah stage pipeline (aksi tersendiri). closed_date/win_loss_reason/loss_notes
-- diisi saat Closed Won/Lost — validasi "Closed Lost wajib win_loss_reason" di
-- handler (bukan constraint DB agar pesan bisa diperbaiki user). closed_date =
-- CURRENT_DATE bila stage terminal, NULL bila dibuka kembali ke stage aktif.
UPDATE deals SET
    stage           = sqlc.arg(stage),
    win_loss_reason = sqlc.narg(win_loss_reason),
    loss_reason_code = sqlc.narg(loss_reason_code),
    loss_notes      = sqlc.narg(loss_notes),
    closed_date     = CASE
        WHEN sqlc.arg(stage) IN ('Closed Won','Closed Lost') THEN CURRENT_DATE
        ELSE NULL END,
    updated_by      = sqlc.narg(updated_by),
    updated_at      = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: SoftDeleteDeal :exec
-- Soft-delete: baris disembunyikan dari list/get tapi tetap ada (jejak & FK dari
-- leads.converted_deal_id tak putus). Idempotent: hanya baris hidup.
UPDATE deals SET deleted_at = now(), updated_by = sqlc.narg(updated_by)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: SetDealCreatedSubscription :exec
-- Tautkan deal ke langganan hasil create-from-deal (deals.created_subscription_id;
-- FK ditutup di migrasi 00012). Dipanggil dalam tx yang SAMA dgn CreateSubscription
-- agar deal Closed Won selalu menunjuk langganan yang lahir darinya (atomik).
UPDATE deals SET
    created_subscription_id = sqlc.arg(created_subscription_id),
    updated_by              = sqlc.narg(updated_by),
    updated_at              = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: GetLatestDealForAccount :one
-- Deal TERBARU satu desa (chip "Terkait" di detail Account). Tanpa filter
-- ownership: gerbangnya desa induk (handler via loadOwnedAccount), pola sama
-- ListContactsByAccount. pgx.ErrNoRows = desa belum punya deal sama sekali.
SELECT * FROM deals
WHERE account_id = sqlc.arg(account_id) AND deleted_at IS NULL
ORDER BY created_at DESC, id DESC
LIMIT 1;

-- name: CountDealsByAccount :one
-- Jumlah deal hidup satu desa — label chip "Terkait" di detail Account. Mirror
-- CountContactsByAccount.
SELECT COUNT(*) FROM deals
WHERE account_id = sqlc.arg(account_id) AND deleted_at IS NULL;
