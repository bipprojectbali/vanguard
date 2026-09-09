-- cs_trainings.sql — Query Training Schedule (CRM Modul 6, sub-item Onboarding
-- 6.2.1.2). RLS mengisolasi workspace; F3 ownership ditegakkan lewat flag
-- boolean (scope_all, is_own) — satu sumber kebenaran dengan
-- CSTrainingsListFilterFor (ownership.go).
--
-- Keyset: (training_date DESC, id DESC) — training_date NOT NULL (jadwal wajib
-- diisi saat dibuat), pola sama Engagements (scheduled_at).

-- name: CreateCSTraining :one
-- Buat jadwal training baru. trainer_id dan participants opsional.
INSERT INTO cs_trainings (
    tenant_id, account_id,
    training_topic, training_date,
    trainer_id, participants, training_status,
    created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(account_id),
    sqlc.arg(training_topic), sqlc.arg(training_date),
    sqlc.narg(trainer_id), sqlc.narg(participants), sqlc.arg(training_status),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: GetCSTraining :one
-- Satu training + nama desa + nama trainer. Dipakai handler
-- CSTrainingUpdateStatus sebelum update untuk validasi keberadaan + F3
-- (handler memanggil CSTrainingsListFilter.Allows).
SELECT tr.*, a.village_name AS account_name,
       u.name AS trainer_name
FROM cs_trainings tr
JOIN accounts a ON tr.account_id = a.id
LEFT JOIN users u ON tr.trainer_id = u.id
WHERE tr.id = sqlc.arg(id);

-- name: UpdateCSTrainingStatus :one
-- Ubah status training. Field hasil (attendance/participants/notes) &
-- training_date (jadwal ulang) OPSIONAL: COALESCE(narg, kolom) menjaga nilai
-- lama saat form tak mengirim (BL-28 #1 — tombol status polos, mis. "Batal"/
-- "Buka Ulang", TAK boleh menimpa peserta/attendance jadi NULL). Kirim
-- non-NULL hanya bila operator memang mengisi (panel "Selesai"/"Jadwal Ulang").
UPDATE cs_trainings
SET
    training_status = sqlc.arg(training_status),
    attendance      = COALESCE(sqlc.narg(attendance), attendance),
    participants    = COALESCE(sqlc.narg(participants), participants),
    training_date   = COALESCE(sqlc.narg(training_date), training_date),
    notes           = COALESCE(sqlc.narg(notes), notes),
    updated_by      = sqlc.narg(updated_by),
    updated_at      = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ListCSTrainings :many
-- Daftar training, keyset (training_date DESC, id DESC) + F3 ownership + filter tab.
--
-- Ownership dikodekan sebagai dua flag boolean (scope_all / is_own):
--   scope_all → lihat semua training workspace (Admin, Manager)
--   is_own    → hanya training desa yang ditugaskan (CSM)
--              = union kepemilikan: account_owner ATAU assigned_csm ATAU backup_csm
-- Keduanya false → OR selalu false → NOL baris (fail-closed).
--
-- filter_status '' → semua status; non-'' → cocokkan persis.
SELECT
    tr.id, tr.account_id, tr.training_topic,
    tr.training_date, tr.trainer_id, tr.participants,
    tr.training_status, tr.attendance, tr.notes,
    tr.created_at, tr.updated_at,
    a.village_name AS account_name,
    u.name AS trainer_name
FROM cs_trainings tr
JOIN accounts a ON tr.account_id = a.id AND a.deleted_at IS NULL
LEFT JOIN users u ON tr.trainer_id = u.id
WHERE (tr.training_date, tr.id) < (sqlc.arg(cursor_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND (
          a.account_owner = sqlc.arg(uid)
          OR a.assigned_csm = sqlc.arg(uid)
          OR a.backup_csm = sqlc.arg(uid)
      ))
  )
  AND (sqlc.arg(filter_status) = '' OR tr.training_status = sqlc.arg(filter_status))
  AND (sqlc.arg(search)::text = ''
       OR tr.training_topic ILIKE '%' || sqlc.arg(search) || '%'
       OR a.village_name ILIKE '%' || sqlc.arg(search) || '%')
  AND (NOT sqlc.arg(use_account)::boolean OR tr.account_id = sqlc.arg(account_id)::bigint)
ORDER BY tr.training_date DESC, tr.id DESC
LIMIT sqlc.arg(page_size);

-- name: CountCSTrainingKPIs :one
-- Agregat KPI header halaman /trainings. Cakupan scope sama persis ListCSTrainings.
-- uid dioper walau scope_all=true (diabaikan dalam kasus itu).
SELECT
    COUNT(*)                                                          AS total_count,
    COUNT(*) FILTER (WHERE tr.training_status = 'scheduled')          AS scheduled_count,
    COUNT(*) FILTER (WHERE tr.training_status = 'completed')          AS completed_count,
    COUNT(*) FILTER (WHERE tr.training_status = 'rescheduled')        AS rescheduled_count,
    COUNT(*) FILTER (WHERE tr.training_status = 'cancelled')          AS cancelled_count
FROM cs_trainings tr
JOIN accounts a ON tr.account_id = a.id AND a.deleted_at IS NULL
WHERE (
    sqlc.arg(scope_all)::boolean
    OR (sqlc.arg(is_own)::boolean AND (
        a.account_owner = sqlc.arg(uid)
        OR a.assigned_csm = sqlc.arg(uid)
        OR a.backup_csm = sqlc.arg(uid)
    ))
  )
  AND (NOT sqlc.arg(use_account)::boolean OR tr.account_id = sqlc.arg(account_id)::bigint);
