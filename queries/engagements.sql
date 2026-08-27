-- engagements.sql — Query Engagements / Check-ins (CRM Modul 6 slice 6.5).
-- RLS mengisolasi workspace; F3 ownership ditegakkan lewat flag boolean
-- (scope_all, is_own) — satu sumber kebenaran dengan EngagementsListFilterFor
-- (ownership.go).
--
-- Keyset: (scheduled_at DESC, id DESC) — beda dari tickets (created_at DESC)
-- karena daftar engagement lebih bermakna diurutkan kapan dijadwalkan, bukan
-- kapan dibuat.

-- name: CreateEngagement :one
-- Buat engagement baru. next_due_date dan outcome opsional.
INSERT INTO engagements (
    tenant_id, account_id,
    subject, engagement_type, frequency,
    scheduled_at, status, channel,
    outcome, next_due_date, owner_id,
    created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(account_id),
    sqlc.arg(subject), sqlc.arg(engagement_type), sqlc.narg(frequency),
    sqlc.arg(scheduled_at), sqlc.arg(status), sqlc.narg(channel),
    sqlc.narg(outcome), sqlc.narg(next_due_date), sqlc.narg(owner_id),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: GetEngagement :one
-- Satu engagement + nama desa + nama owner. Dipakai handler UpdateEngagementStatus
-- sebelum update untuk validasi keberadaan + F3 (handler memanggil
-- EngagementsListFilter.Allows).
SELECT e.*, a.village_name AS account_name,
       u.name AS owner_name
FROM engagements e
JOIN accounts a ON e.account_id = a.id
LEFT JOIN users u ON e.owner_id = u.id
WHERE e.id = sqlc.arg(id);

-- name: UpdateEngagementStatus :one
-- Ubah status engagement. outcome diperbarui bersamaan (CSM mengisi ringkasan
-- setelah engagement selesai). next_due_date dapat diperbarui (khususnya saat
-- status = rescheduled).
UPDATE engagements
SET
    status        = sqlc.arg(status),
    outcome       = sqlc.narg(outcome),
    next_due_date = sqlc.narg(next_due_date),
    updated_by    = sqlc.narg(updated_by),
    updated_at    = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ListEngagements :many
-- Daftar engagement, keyset (scheduled_at DESC, id DESC) + F3 ownership + filter tab.
--
-- Ownership dikodekan sebagai dua flag boolean (scope_all / is_own):
--   scope_all → lihat semua engagement workspace (Admin, Manager)
--   is_own    → hanya engagement desa yang ditugaskan (CSM)
--              = union kepemilikan: account_owner ATAU assigned_csm ATAU backup_csm
-- Keduanya false → OR selalu false → NOL baris (fail-closed).
--
-- filter_status '' → semua status; non-'' → cocokkan persis.
-- filter_type '' → semua tipe engagement; non-'' → cocokkan persis.
SELECT
    e.id, e.account_id, e.subject,
    e.engagement_type, e.frequency,
    e.scheduled_at, e.status, e.channel,
    e.outcome, e.next_due_date, e.owner_id,
    e.created_at, e.updated_at,
    a.village_name AS account_name,
    u.name AS owner_name
FROM engagements e
JOIN accounts a ON e.account_id = a.id AND a.deleted_at IS NULL
LEFT JOIN users u ON e.owner_id = u.id
WHERE (e.scheduled_at, e.id) < (sqlc.arg(cursor_scheduled_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND (
          a.account_owner = sqlc.arg(uid)
          OR a.assigned_csm = sqlc.arg(uid)
          OR a.backup_csm = sqlc.arg(uid)
      ))
  )
  AND (sqlc.arg(filter_status) = '' OR e.status = sqlc.arg(filter_status))
  AND (sqlc.arg(filter_type)   = '' OR e.engagement_type = sqlc.arg(filter_type))
ORDER BY e.scheduled_at DESC, e.id DESC
LIMIT sqlc.arg(page_size);

-- name: CountEngagementKPIs :one
-- Agregat KPI header halaman /engagements. Cakupan scope sama persis ListEngagements.
-- uid dioper walau scope_all=true (diabaikan dalam kasus itu).
SELECT
    COUNT(*)                                                  AS total_count,
    COUNT(*) FILTER (WHERE e.status = 'planned')              AS planned_count,
    COUNT(*) FILTER (WHERE e.status = 'done')                 AS done_count,
    COUNT(*) FILTER (WHERE e.status IN ('skipped', 'rescheduled'))
                                                              AS missed_count,
    COUNT(*) FILTER (WHERE e.next_due_date IS NOT NULL
                       AND e.next_due_date <= (now() AT TIME ZONE 'UTC')::date + 7
                       AND e.status = 'planned')              AS due_soon_count
FROM engagements e
JOIN accounts a ON e.account_id = a.id AND a.deleted_at IS NULL
WHERE (
    sqlc.arg(scope_all)::boolean
    OR (sqlc.arg(is_own)::boolean AND (
        a.account_owner = sqlc.arg(uid)
        OR a.assigned_csm = sqlc.arg(uid)
        OR a.backup_csm = sqlc.arg(uid)
    ))
);

-- name: GetLatestEngagementForAccount :one
-- Engagement TERPILIH TERBARU satu desa (kartu "Ringkasan Customer Success" di
-- detail Account, baris "Terakhir Engagement") — diurut scheduled_at (bukan
-- created_at), konsisten dgn ListEngagements (lihat rationale di atas). Tanpa
-- filter ownership: gerbangnya desa induk (handler via loadOwnedAccount), sama
-- seperti ListContactsByAccount. pgx.ErrNoRows = desa belum punya engagement.
SELECT e.*, u.name AS owner_name
FROM engagements e
LEFT JOIN users u ON e.owner_id = u.id
WHERE e.account_id = sqlc.arg(account_id)
ORDER BY e.scheduled_at DESC, e.id DESC
LIMIT 1;
