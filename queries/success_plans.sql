-- success_plans.sql — Query Success Plans (CRM Modul 6 slice 6.3).
-- RLS mengisolasi workspace; F3 ownership ditegakkan lewat flag boolean
-- (scope_all, is_own) — satu sumber kebenaran dengan SuccessPlansListFilterFor
-- (ownership.go).
--
-- Keyset: (created_at DESC, id DESC) — baris terbaru di atas; cursor dari
-- splitPage. Filter plan_status opsional ('' = semua).

-- name: CreateSuccessPlan :one
-- Buat success plan baru. objective, success_metric, target_date, owner_csm opsional.
INSERT INTO success_plans (
    tenant_id, account_id,
    plan_name, objective, success_metric,
    target_date, plan_status, progress,
    owner_csm, created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(account_id),
    sqlc.arg(plan_name), sqlc.narg(objective), sqlc.narg(success_metric),
    sqlc.narg(target_date), sqlc.arg(plan_status), sqlc.arg(progress),
    sqlc.narg(owner_csm), sqlc.narg(created_by)
)
RETURNING *;

-- name: GetSuccessPlan :one
-- Satu success plan + nama desa + nama owner. Dipakai handler sebelum Edit/Update
-- untuk validasi keberadaan + F3 (handler memanggil SuccessPlansListFilter.Allows).
SELECT sp.*, a.village_name AS account_name,
       u.name AS owner_name
FROM success_plans sp
JOIN accounts a ON sp.account_id = a.id
LEFT JOIN users u ON sp.owner_csm = u.id
WHERE sp.id = sqlc.arg(id)
  AND sp.deleted_at IS NULL;

-- name: UpdateSuccessPlan :one
-- Perbarui semua field editable success plan.
UPDATE success_plans
SET
    plan_name      = sqlc.arg(plan_name),
    objective      = sqlc.narg(objective),
    success_metric = sqlc.narg(success_metric),
    target_date    = sqlc.narg(target_date),
    plan_status    = sqlc.arg(plan_status),
    progress       = sqlc.arg(progress),
    owner_csm      = sqlc.narg(owner_csm),
    updated_by     = sqlc.narg(updated_by),
    updated_at     = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL
RETURNING *;

-- name: ListSuccessPlans :many
-- Daftar success plan, keyset (created_at DESC, id DESC) + F3 ownership + filter tab.
--
-- Ownership dikodekan sebagai dua flag boolean (scope_all / is_own):
--   scope_all → lihat semua plan workspace (Admin, Manager)
--   is_own    → hanya plan untuk desa yang ditugaskan (CSM)
--              = union kepemilikan akun: account_owner ATAU assigned_csm ATAU backup_csm
--              ATAU owner_csm plan = uid (CSM pemilik plan, walau desa berpindah)
-- Keduanya false → OR selalu false → NOL baris (fail-closed).
--
-- filter_status '' → semua status; non-'' → cocokkan persis.
SELECT
    sp.id, sp.account_id,
    sp.plan_name, sp.objective, sp.success_metric,
    sp.target_date, sp.plan_status, sp.progress,
    sp.owner_csm, sp.created_at,
    a.village_name AS account_name,
    u.name AS owner_name
FROM success_plans sp
JOIN accounts a ON sp.account_id = a.id AND a.deleted_at IS NULL
LEFT JOIN users u ON sp.owner_csm = u.id
WHERE (sp.created_at, sp.id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND sp.deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND (
          a.account_owner  = sqlc.arg(uid)
          OR a.assigned_csm = sqlc.arg(uid)
          OR a.backup_csm   = sqlc.arg(uid)
          OR sp.owner_csm   = sqlc.arg(uid)
      ))
  )
  AND (sqlc.arg(filter_status) = '' OR sp.plan_status = sqlc.arg(filter_status))
  AND (sqlc.arg(search)::text = ''
       OR sp.plan_name ILIKE '%' || sqlc.arg(search) || '%'
       OR a.village_name ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY sp.created_at DESC, sp.id DESC
LIMIT sqlc.arg(page_size);

-- name: SoftDeleteSuccessPlan :exec
-- Hapus lunak success plan (tandai deleted_at). Fail jika sudah dihapus.
UPDATE success_plans
SET deleted_at = now(),
    updated_by = sqlc.narg(updated_by),
    updated_at = now()
WHERE id = sqlc.arg(id)
  AND deleted_at IS NULL;
