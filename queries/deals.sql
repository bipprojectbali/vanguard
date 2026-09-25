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

-- name: ListDealsSortByCode :many
-- BL-157e (fondasi sort per kolom Deals): SAMA PERSIS filter ListDeals
-- (ownership F3 + mine_only + stage_filter + search) — hanya ORDER BY/keyset
-- yang beda, diurut entity_code (bukan created_at). entity_code NULLABLE
-- (diisi GenerateEntityCode saat create, tapi kolom tetap nullable di skema)
-- → kloning PERSIS pola ListLeadsSortByCode. NULLS default Postgres
-- (ASC=LAST, DESC=FIRST).
SELECT * FROM deals
WHERE deleted_at IS NULL
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc' AND (
          (NOT sqlc.arg(cursor_is_null)::boolean
           AND (entity_code IS NULL OR (entity_code, id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint)))
          OR (sqlc.arg(cursor_is_null)::boolean AND entity_code IS NULL AND id > sqlc.arg(cursor_id)::bigint)
      ))
      OR (sqlc.arg(dir)::text = 'desc' AND (
          (sqlc.arg(cursor_is_null)::boolean
           AND (entity_code IS NOT NULL OR id < sqlc.arg(cursor_id)::bigint))
          OR (NOT sqlc.arg(cursor_is_null)::boolean AND entity_code IS NOT NULL
              AND (entity_code, id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      ))
  )
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
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN entity_code END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN entity_code END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListDealsSortByName :many
-- BL-157e: sort by deal_name ("Deal"). deal_name TIDAK NULLABLE → kloning pola
-- ListLeadsSortByName (tanpa kerumitan NULL).
SELECT * FROM deals
WHERE deleted_at IS NULL
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc'
          AND (deal_name, id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      OR (sqlc.arg(dir)::text = 'desc'
          AND (deal_name, id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
  )
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
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN deal_name END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN deal_name END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListDealsSortByStage :many
-- BL-157e: sort by stage ("Tahap" — RAW enum Prospecting/Qualification/…,
-- alfabetis; tak menduplikasi urutan pipeline ke SQL, mirror keputusan
-- "Status" Leads BL-157b). stage NOT NULL → kloning PERSIS pola
-- ListDealsSortByName.
SELECT * FROM deals
WHERE deleted_at IS NULL
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc'
          AND (stage, id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      OR (sqlc.arg(dir)::text = 'desc'
          AND (stage, id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
  )
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
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN stage END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN stage END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListDealsSortByAmount :many
-- BL-157e: sort by amount ("Nilai"). NULLABLE numeric. Kunci sort memakai
-- nilai ASLI (tak ter-mask) — F4 (maskARR) hanya menyamarkan TAMPILAN di
-- handler, mirror presedan Estimasi Leads (ListLeadsSortByValue). Pola
-- null-aware SAMA dgn ListDealsSortByCode, tipe kolom numeric bukan text.
SELECT * FROM deals
WHERE deleted_at IS NULL
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc' AND (
          (NOT sqlc.arg(cursor_is_null)::boolean
           AND (amount IS NULL OR (amount, id) > (sqlc.arg(cursor_val)::numeric, sqlc.arg(cursor_id)::bigint)))
          OR (sqlc.arg(cursor_is_null)::boolean AND amount IS NULL AND id > sqlc.arg(cursor_id)::bigint)
      ))
      OR (sqlc.arg(dir)::text = 'desc' AND (
          (sqlc.arg(cursor_is_null)::boolean
           AND (amount IS NOT NULL OR id < sqlc.arg(cursor_id)::bigint))
          OR (NOT sqlc.arg(cursor_is_null)::boolean AND amount IS NOT NULL
              AND (amount, id) < (sqlc.arg(cursor_val)::numeric, sqlc.arg(cursor_id)::bigint))
      ))
  )
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
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN amount END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN amount END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListDealsSortByProbability :many
-- BL-157e: sort by probability ("Peluang", %). NULLABLE smallint. Pola
-- null-aware SAMA dgn ListDealsSortByAmount, tipe kolom smallint.
SELECT * FROM deals
WHERE deleted_at IS NULL
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc' AND (
          (NOT sqlc.arg(cursor_is_null)::boolean
           AND (probability IS NULL OR (probability, id) > (sqlc.arg(cursor_val)::smallint, sqlc.arg(cursor_id)::bigint)))
          OR (sqlc.arg(cursor_is_null)::boolean AND probability IS NULL AND id > sqlc.arg(cursor_id)::bigint)
      ))
      OR (sqlc.arg(dir)::text = 'desc' AND (
          (sqlc.arg(cursor_is_null)::boolean
           AND (probability IS NOT NULL OR id < sqlc.arg(cursor_id)::bigint))
          OR (NOT sqlc.arg(cursor_is_null)::boolean AND probability IS NOT NULL
              AND (probability, id) < (sqlc.arg(cursor_val)::smallint, sqlc.arg(cursor_id)::bigint))
      ))
  )
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
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN probability END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN probability END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListDealsSortByCloseDate :many
-- BL-157e: sort by expected_close_date ("Perkiraan Tutup"). NULLABLE date
-- (belum tentu diisi saat deal dibuat). Pola null-aware SAMA dgn
-- ListDealsSortByCode, tipe kolom date — kloning PERSIS
-- ListSubscriptionsSortByRenewal.
SELECT * FROM deals
WHERE deleted_at IS NULL
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc' AND (
          (NOT sqlc.arg(cursor_is_null)::boolean
           AND (expected_close_date IS NULL OR (expected_close_date, id) > (sqlc.arg(cursor_val)::date, sqlc.arg(cursor_id)::bigint)))
          OR (sqlc.arg(cursor_is_null)::boolean AND expected_close_date IS NULL AND id > sqlc.arg(cursor_id)::bigint)
      ))
      OR (sqlc.arg(dir)::text = 'desc' AND (
          (sqlc.arg(cursor_is_null)::boolean
           AND (expected_close_date IS NOT NULL OR id < sqlc.arg(cursor_id)::bigint))
          OR (NOT sqlc.arg(cursor_is_null)::boolean AND expected_close_date IS NOT NULL
              AND (expected_close_date, id) < (sqlc.arg(cursor_val)::date, sqlc.arg(cursor_id)::bigint))
      ))
  )
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
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN expected_close_date END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN expected_close_date END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListDealsSortByOwner :many
-- BL-157e: sort by Pemilik (deal_owner). Kunci sort HARUS
-- COALESCE(NULLIF(u.name,''), u.email) — PERSIS logika tampil ownerName/
-- memberNameMap (nama bila terisi, else email) — agar urutan tak menyimpang
-- dari yang ditampilkan. NULLABLE (deal_owner ON DELETE SET NULL). LEFT JOIN
-- users: baris tanpa owner ATAU owner terhapus → owner_key NULL, masuk
-- kelompok NULL (default Postgres). Mirror PERSIS ListLeadsSortByOwner.
SELECT deals.* FROM deals
LEFT JOIN users u ON u.id = deal_owner
WHERE deals.deleted_at IS NULL
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc' AND (
          (NOT sqlc.arg(cursor_is_null)::boolean
           AND (COALESCE(NULLIF(u.name, ''), u.email) IS NULL
                OR (COALESCE(NULLIF(u.name, ''), u.email), deals.id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint)))
          OR (sqlc.arg(cursor_is_null)::boolean
              AND COALESCE(NULLIF(u.name, ''), u.email) IS NULL AND deals.id > sqlc.arg(cursor_id)::bigint)
      ))
      OR (sqlc.arg(dir)::text = 'desc' AND (
          (sqlc.arg(cursor_is_null)::boolean
           AND (COALESCE(NULLIF(u.name, ''), u.email) IS NOT NULL OR deals.id < sqlc.arg(cursor_id)::bigint))
          OR (NOT sqlc.arg(cursor_is_null)::boolean AND COALESCE(NULLIF(u.name, ''), u.email) IS NOT NULL
              AND (COALESCE(NULLIF(u.name, ''), u.email), deals.id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      ))
  )
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
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN COALESCE(NULLIF(u.name, ''), u.email) END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN COALESCE(NULLIF(u.name, ''), u.email) END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN deals.id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN deals.id END DESC
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
-- last_active_stage (00053, BL-173): snapshot tahap aktif TERAKHIR sebelum
-- transisi ke terminal — dipakai stepper utk bedakan "march-through penuh" vs
-- "gugur langsung dari tahap awal". prev_stage = stage SEBELUM update ini
-- (dioper handler, dibaca dari h.loadOwnedDeal sebelum UpdateDealStage jalan).
-- Diisi hanya saat aktif→terminal DAN prev_stage salah satu dari 5 tahap aktif
-- (ALLOWLIST, bukan "bukan terminal" — pemanggil lama/test yang belum diaudit
-- & tak mengoper prev_stage jatuh ke zero-value "", yang gagal CHECK
-- deals_last_active_stage_chk bila dipaksa masuk; allowlist bikin nilai tak
-- dikenal jatuh ke ELSE-pertahankan alih-alih coba tulis nilai ilegal).
-- Dikosongkan saat reopen (stage baru aktif); dipertahankan saat
-- terminal→terminal (mis. Closed Lost → Closed Won tanpa reopen dulu).
UPDATE deals SET
    stage           = sqlc.arg(stage),
    win_loss_reason = sqlc.narg(win_loss_reason),
    loss_reason_code = sqlc.narg(loss_reason_code),
    loss_notes      = sqlc.narg(loss_notes),
    closed_date     = CASE
        WHEN sqlc.arg(stage) IN ('Closed Won','Closed Lost') THEN CURRENT_DATE
        ELSE NULL END,
    last_active_stage = CASE
        WHEN sqlc.arg(stage) IN ('Closed Won','Closed Lost')
             AND sqlc.arg(prev_stage)::text IN
                 ('Prospecting','Qualification','Demo','Proposal','Negotiation')
            THEN sqlc.arg(prev_stage)::text
        WHEN sqlc.arg(stage) NOT IN ('Closed Won','Closed Lost') THEN NULL
        ELSE last_active_stage END,
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

-- name: SetDealRequestedPlan :exec
-- BL-100 (Fix B): backfill paket deal dari quote yang BARU di-Accept, agar Closed Won
-- bisa membuat langganan (subscriptionFromWonDeal membaca deals.plan_requested_id yang
-- tak pernah diisi jalur UI). Timpa nilai lama (accept terakhir menang). Tak menyentuh
-- baris terhapus.
UPDATE deals SET
    plan_requested_id = sqlc.arg(plan_requested_id),
    updated_by        = sqlc.narg(updated_by),
    updated_at        = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: SetDealRecognizedValue :exec
-- BL-88 (quote otoritatif): salin grand_total quote yang BARU di-Accept ke deal.amount
-- sebagai NILAI DIAKUI (revenue MRR/ARR laporan). Menggantikan nilai perkiraan manual;
-- jalur tersendiri (bukan UpdateDeal, itu jalur form) agar terlihat sebagai efek Accept.
-- Tak menyentuh baris terhapus.
UPDATE deals SET
    amount     = sqlc.narg(amount),
    updated_by = sqlc.narg(updated_by),
    updated_at = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListDealsForJena :many
-- Jena AI (BL-162 fase 3, tool search_deals — internal/handler/jena_ai_tools_sales.go):
-- cermin PERSIS ListLeadsForJena (lihat leads.sql) utk deal — SAMA filter
-- ownership F3 (scope_all/is_own, sumber SATU dgn DealsListFilterFor) TANPA
-- mine_only, beda dari list_my_deals yang MEMAKSA deal_owner=uid. Tunduk cakupan
-- role penanya: scope 'own' nol baris utk deal orang lain, owner_search tak bisa
-- melebarkannya.
--
-- owner_search '' → tak menyaring; selain itu MEMPERSEMPIT lewat LEFT JOIN users
-- (ILIKE nama ATAU email). search (deal_name/entity_code) independen, keduanya
-- AND (mempersempit saja). owner_label = nama > email (pola SAMA dgn
-- accounts.sql/ListLeadsForJena), string KOSONG (bukan NULL — fallback ''
-- eksplisit, lihat catatan ListLeadsForJena) bila deal belum berpemilik.
SELECT d.*, COALESCE(NULLIF(u.name, ''), u.email, '') AS owner_label
FROM deals d
LEFT JOIN users u ON u.id = d.deal_owner
WHERE d.deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND d.deal_owner = sqlc.arg(uid))
  )
  AND (
      sqlc.arg(search)::text = ''
      OR d.deal_name ILIKE '%' || sqlc.arg(search) || '%'
      OR d.entity_code ILIKE '%' || sqlc.arg(search) || '%'
  )
  AND (
      sqlc.arg(owner_search)::text = ''
      OR u.name ILIKE '%' || sqlc.arg(owner_search) || '%'
      OR u.email ILIKE '%' || sqlc.arg(owner_search) || '%'
  )
ORDER BY d.created_at DESC, d.id DESC
LIMIT sqlc.arg(page_size);
