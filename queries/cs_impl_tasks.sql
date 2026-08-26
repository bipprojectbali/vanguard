-- cs_impl_tasks.sql — Query Implementation Tracker (CRM Modul 6, sub-item
-- Onboarding 6.2.1.1). RLS mengisolasi workspace; F3 ownership ditegakkan
-- lewat flag boolean (scope_all, is_own) — satu sumber kebenaran dengan
-- CSImplTasksListFilterFor (ownership.go).
--
-- Keyset: (created_at DESC, id DESC) — BUKAN due_date (nullable, task belum
-- ditugaskan tanggal boleh ada), pola sama Tickets (bukan Engagements yang
-- scheduled_at-nya NOT NULL).

-- name: CreateCSImplTask :one
-- Buat task baru. owner_id dan due_date opsional (ditugaskan belakangan).
INSERT INTO cs_impl_tasks (
    tenant_id, account_id,
    task_name, task_status,
    owner_id, due_date,
    created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(account_id),
    sqlc.arg(task_name), sqlc.arg(task_status),
    sqlc.narg(owner_id), sqlc.narg(due_date),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: GetCSImplTask :one
-- Satu task + nama desa + nama owner. Dipakai handler CSImplTaskUpdateStatus
-- sebelum update untuk validasi keberadaan + F3 (handler memanggil
-- CSImplTasksListFilter.Allows).
SELECT t.*, a.village_name AS account_name,
       u.name AS owner_name
FROM cs_impl_tasks t
JOIN accounts a ON t.account_id = a.id
LEFT JOIN users u ON t.owner_id = u.id
WHERE t.id = sqlc.arg(id);

-- name: UpdateCSImplTaskStatus :one
-- Ubah status task + due_date (dapat digeser saat status = in_progress) +
-- owner_id (reassign penanggung jawab).
UPDATE cs_impl_tasks
SET
    task_status = sqlc.arg(task_status),
    due_date    = sqlc.narg(due_date),
    owner_id    = sqlc.narg(owner_id),
    updated_by  = sqlc.narg(updated_by),
    updated_at  = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ListCSImplTasks :many
-- Daftar task, keyset (created_at DESC, id DESC) + F3 ownership + filter tab.
--
-- Ownership dikodekan sebagai dua flag boolean (scope_all / is_own):
--   scope_all → lihat semua task workspace (Admin, Manager)
--   is_own    → hanya task desa yang ditugaskan (CSM)
--              = union kepemilikan: account_owner ATAU assigned_csm ATAU backup_csm
-- Keduanya false → OR selalu false → NOL baris (fail-closed).
--
-- filter_status '' → semua status; non-'' → cocokkan persis.
SELECT
    t.id, t.account_id, t.task_name,
    t.task_status, t.owner_id, t.due_date,
    t.created_at, t.updated_at,
    a.village_name AS account_name,
    u.name AS owner_name
FROM cs_impl_tasks t
JOIN accounts a ON t.account_id = a.id AND a.deleted_at IS NULL
LEFT JOIN users u ON t.owner_id = u.id
WHERE (t.created_at, t.id) < (sqlc.arg(cursor_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND (
          a.account_owner = sqlc.arg(uid)
          OR a.assigned_csm = sqlc.arg(uid)
          OR a.backup_csm = sqlc.arg(uid)
      ))
  )
  AND (sqlc.arg(filter_status) = '' OR t.task_status = sqlc.arg(filter_status))
ORDER BY t.created_at DESC, t.id DESC
LIMIT sqlc.arg(page_size);

-- name: CountCSImplTaskKPIs :one
-- Agregat KPI header halaman /impl-tasks. Cakupan scope sama persis ListCSImplTasks.
-- uid dioper walau scope_all=true (diabaikan dalam kasus itu).
SELECT
    COUNT(*)                                                    AS total_count,
    COUNT(*) FILTER (WHERE t.task_status = 'to_do')             AS to_do_count,
    COUNT(*) FILTER (WHERE t.task_status = 'in_progress')       AS in_progress_count,
    COUNT(*) FILTER (WHERE t.task_status = 'done')              AS done_count,
    COUNT(*) FILTER (WHERE t.task_status = 'blocked')           AS blocked_count
FROM cs_impl_tasks t
JOIN accounts a ON t.account_id = a.id AND a.deleted_at IS NULL
WHERE (
    sqlc.arg(scope_all)::boolean
    OR (sqlc.arg(is_own)::boolean AND (
        a.account_owner = sqlc.arg(uid)
        OR a.assigned_csm = sqlc.arg(uid)
        OR a.backup_csm = sqlc.arg(uid)
    ))
);
