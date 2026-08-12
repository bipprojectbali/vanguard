-- activities.sql — aktivitas polimorfik (task/call/note …). Isolasi WORKSPACE
-- ditegakkan RLS (GUC app.tenant_id di WithTenant); isolasi ANTAR-DESA (F3)
-- ditegakkan di layer query lewat flag ownership di ListActivities — lihat
-- ownership.go (ActivitiesListFilter, sumbu tunggal owner_id).
--
-- Nama query JAMAK (Activities) — sengaja beda dari activity.sql (presence,
-- 00001) agar tak bentrok di querier generated. Tabel TAK punya entity_code
-- (spec §7a) → tak ada alokasi kode di create.
--
-- activity_context di-set handler ('sales' untuk 4.4); target_id BUKAN FK
-- (integritas target diverifikasi handler via loadOwned* sebelum insert).

-- name: CreateActivity :one
-- Buat aktivitas. tenant_id di-set eksplisit (RLS WITH CHECK memverifikasinya =
-- GUC). Kolom per-kind opsional (narg) — hanya yang relevan untuk kind terisi.
INSERT INTO activities (
    tenant_id, kind, subject, target_type, target_id,
    owner_id, activity_context, status, notes,
    due_date, priority, reminder_at,
    contact_id, direction, activity_at, duration_min, call_result,
    body,
    created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(kind), sqlc.arg(subject),
    sqlc.arg(target_type), sqlc.arg(target_id),
    sqlc.narg(owner_id), sqlc.arg(activity_context), sqlc.narg(status), sqlc.narg(notes),
    sqlc.narg(due_date), sqlc.narg(priority), sqlc.narg(reminder_at),
    sqlc.narg(contact_id), sqlc.narg(direction), sqlc.narg(activity_at),
    sqlc.narg(duration_min), sqlc.narg(call_result),
    sqlc.narg(body),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: GetActivity :one
-- Satu aktivitas hidup. RLS menjamin tenant_id; ownership diputuskan handler
-- (ActivitiesListFilter.Allows) atas baris.
SELECT * FROM activities
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListActivities :many
-- Daftar aktivitas (tampilan Tabel), keyset (created_at DESC, id DESC) + filter
-- ownership F3 + filter context. Dua flag ownership (sumber SATU dengan
-- ActivitiesListFilter): scope_all → semua; is_own → owner_id = uid; keduanya
-- false → NOL baris (fail-closed). context_filter menyaring view modul
-- ('sales' untuk 4.4) — pemisah dari CS 6.5 / general M7.
SELECT * FROM activities
WHERE deleted_at IS NULL
  AND activity_context = sqlc.arg(context_filter)::text
  AND (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND owner_id = sqlc.arg(uid))
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: UpdateActivity :one
-- Sunting aktivitas. kind & target TAK di sini: kind menentukan bentuk form
-- (immutable saat edit), target ditetapkan saat create. status punya jalur
-- khusus (UpdateActivityStatus) agar perpindahan terlihat sebagai aksi tersendiri.
UPDATE activities SET
    subject      = sqlc.arg(subject),
    owner_id     = sqlc.narg(owner_id),
    notes        = sqlc.narg(notes),
    due_date     = sqlc.narg(due_date),
    priority     = sqlc.narg(priority),
    reminder_at  = sqlc.narg(reminder_at),
    contact_id   = sqlc.narg(contact_id),
    direction    = sqlc.narg(direction),
    activity_at  = sqlc.narg(activity_at),
    duration_min = sqlc.narg(duration_min),
    call_result  = sqlc.narg(call_result),
    body         = sqlc.narg(body),
    updated_by   = sqlc.narg(updated_by),
    updated_at   = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: UpdateActivityStatus :exec
-- Ubah status (aksi tersendiri, cermin UpdateDealStage). Validasi enum di handler
-- (allowlist) + CHECK DB sebagai jaring terakhir.
UPDATE activities SET
    status     = sqlc.arg(status),
    updated_by = sqlc.narg(updated_by),
    updated_at = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: SoftDeleteActivity :exec
-- Soft-delete: baris disembunyikan dari list/get tapi tetap ada (jejak aktivitas
-- bertahan). Idempotent: hanya baris hidup.
UPDATE activities SET deleted_at = now(), updated_by = sqlc.narg(updated_by)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;
