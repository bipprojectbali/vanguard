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
    start_at, end_at, location, meeting_type, channel,
    body,
    created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(kind), sqlc.arg(subject),
    sqlc.arg(target_type), sqlc.arg(target_id),
    sqlc.narg(owner_id), sqlc.arg(activity_context), sqlc.narg(status), sqlc.narg(notes),
    sqlc.narg(due_date), sqlc.narg(priority), sqlc.narg(reminder_at),
    sqlc.narg(contact_id), sqlc.narg(direction), sqlc.narg(activity_at),
    sqlc.narg(duration_min), sqlc.narg(call_result),
    sqlc.narg(start_at), sqlc.narg(end_at), sqlc.narg(location),
    sqlc.narg(meeting_type), sqlc.narg(channel),
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
-- search '' → tak menyaring; selain itu MEMPERSEMPIT (ILIKE substring, case-
-- insensitive) di ATAS ownership+context — tak pernah melebarkan baris (BL-6).
-- Hanya subject (satu-satunya kolom teks bebas yang TAMPIL) jadi kunci cari;
-- kind/status/target = enum/id (bukan teks bebas), owner = nama diresolusi handler.
SELECT * FROM activities
WHERE deleted_at IS NULL
  AND activity_context = sqlc.arg(context_filter)::text
  AND (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND owner_id = sqlc.arg(uid))
  )
  AND (sqlc.arg(search)::text = '' OR subject ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: ListActivitiesSortByKind :many
-- BL-157j (sort per kolom Sales Activities): sort by Jenis (kind), RAW enum
-- alfabetis (task/meeting/call/chat/note) — mirror keputusan "Status" Leads/
-- "Tipe" Accounts: tak menduplikasi urutan tampil ke SQL. NOT NULL → kloning
-- pola sederhana ListContactsSortByName (tanpa kerumitan NULL). SAMA PERSIS
-- filter ListActivities (context+F3+search) — hanya ORDER BY/keyset beda.
SELECT * FROM activities
WHERE deleted_at IS NULL
  AND activity_context = sqlc.arg(context_filter)::text
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc'
          AND (kind, id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      OR (sqlc.arg(dir)::text = 'desc'
          AND (kind, id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
  )
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND owner_id = sqlc.arg(uid))
  )
  AND (sqlc.arg(search)::text = '' OR subject ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN kind END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN kind END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListActivitiesSortBySubject :many
-- BL-157j: sort by Subjek (subject), NOT NULL → kloning sederhana sama pola
-- ListActivitiesSortByKind, kolom beda.
SELECT * FROM activities
WHERE deleted_at IS NULL
  AND activity_context = sqlc.arg(context_filter)::text
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc'
          AND (subject, id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      OR (sqlc.arg(dir)::text = 'desc'
          AND (subject, id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
  )
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND owner_id = sqlc.arg(uid))
  )
  AND (sqlc.arg(search)::text = '' OR subject ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN subject END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN subject END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListActivitiesSortByOwner :many
-- BL-157j: sort by Pemilik (owner_id). Kunci sort HARUS
-- COALESCE(NULLIF(u.name,''), u.email) — PERSIS logika tampil ownerName/
-- memberNameMap (nama bila terisi, else email) — agar urutan tak menyimpang
-- dari yang ditampilkan. NULLABLE (owner_id ON DELETE SET NULL). LEFT JOIN
-- users: baris tanpa owner ATAU owner terhapus → owner_key NULL, masuk
-- kelompok NULL (default Postgres). Mirror PERSIS ListDealsSortByOwner.
SELECT activities.* FROM activities
LEFT JOIN users u ON u.id = owner_id
WHERE activities.deleted_at IS NULL
  AND activities.activity_context = sqlc.arg(context_filter)::text
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc' AND (
          (NOT sqlc.arg(cursor_is_null)::boolean
           AND (COALESCE(NULLIF(u.name, ''), u.email) IS NULL
                OR (COALESCE(NULLIF(u.name, ''), u.email), activities.id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint)))
          OR (sqlc.arg(cursor_is_null)::boolean
              AND COALESCE(NULLIF(u.name, ''), u.email) IS NULL AND activities.id > sqlc.arg(cursor_id)::bigint)
      ))
      OR (sqlc.arg(dir)::text = 'desc' AND (
          (sqlc.arg(cursor_is_null)::boolean
           AND (COALESCE(NULLIF(u.name, ''), u.email) IS NOT NULL OR activities.id < sqlc.arg(cursor_id)::bigint))
          OR (NOT sqlc.arg(cursor_is_null)::boolean AND COALESCE(NULLIF(u.name, ''), u.email) IS NOT NULL
              AND (COALESCE(NULLIF(u.name, ''), u.email), activities.id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      ))
  )
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND owner_id = sqlc.arg(uid))
  )
  AND (sqlc.arg(search)::text = '' OR subject ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN COALESCE(NULLIF(u.name, ''), u.email) END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN COALESCE(NULLIF(u.name, ''), u.email) END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN activities.id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN activities.id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListActivitiesSortByStatus :many
-- BL-157j: sort by Status, RAW enum alfabetis — kolom mentah aktivitas, BUKAN
-- derivasi seperti Renewals; mirror keputusan Tickets (Status sortable karena
-- raw, bukan derivasi penuh). NULLABLE (call/note tanpa status) → pola
-- null-aware SAMA dgn ListContactsSortByRole.
SELECT * FROM activities
WHERE deleted_at IS NULL
  AND activity_context = sqlc.arg(context_filter)::text
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc' AND (
          (NOT sqlc.arg(cursor_is_null)::boolean
           AND (status IS NULL OR (status, id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint)))
          OR (sqlc.arg(cursor_is_null)::boolean AND status IS NULL AND id > sqlc.arg(cursor_id)::bigint)
      ))
      OR (sqlc.arg(dir)::text = 'desc' AND (
          (sqlc.arg(cursor_is_null)::boolean
           AND (status IS NOT NULL OR id < sqlc.arg(cursor_id)::bigint))
          OR (NOT sqlc.arg(cursor_is_null)::boolean AND status IS NOT NULL
              AND (status, id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      ))
  )
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND owner_id = sqlc.arg(uid))
  )
  AND (sqlc.arg(search)::text = '' OR subject ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN status END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN status END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListActivitiesSortByDate :many
-- BL-157j (lanjutan): sort by Tanggal (created_at) — NOT NULL, tapi BEDA dari
-- ListActivities: arah DINAMIS (bisa asc, bukan desc tetap), jadi tak bisa
-- pakai pageCursor/splitPage biasa (sentinel ±Infinity, arah tetap). Kolom ini
-- SUDAH jadi sumbu default (created_at DESC via ListActivities) — varian ini
-- HANYA menambah kemampuan flip ke ASC & jadi target klik header eksplisit;
-- SAMA PERSIS filter ListActivities (context+F3+search), cuma keyset+ORDER BY
-- beda arah.
SELECT * FROM activities
WHERE deleted_at IS NULL
  AND activity_context = sqlc.arg(context_filter)::text
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc'
          AND (created_at, id) > (sqlc.arg(cursor_val)::timestamptz, sqlc.arg(cursor_id)::bigint))
      OR (sqlc.arg(dir)::text = 'desc'
          AND (created_at, id) < (sqlc.arg(cursor_val)::timestamptz, sqlc.arg(cursor_id)::bigint))
  )
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND owner_id = sqlc.arg(uid))
  )
  AND (sqlc.arg(search)::text = '' OR subject ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN created_at END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN created_at END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListActivitiesByTarget :many
-- Timeline satu entitas: semua aktivitas yang terkait ke target_type+target_id ini,
-- keyset (created_at DESC, id DESC). Tanpa filter context (lintas sales/cs/general)
-- dan tanpa F3 ownership — siapa pun yang boleh lihat entitasnya boleh lihat
-- timelinenya (gate ada di handler detail entitas masing-masing). search '' →
-- tak menyaring; selain itu MEMPERSEMPIT subject (BL-6, ILIKE case-insensitive)
-- di ATAS filter target — tak menembus ke entitas lain.
SELECT * FROM activities
WHERE deleted_at IS NULL
  AND target_type = sqlc.arg(target_type)::text
  AND target_id   = sqlc.arg(target_id)::bigint
  AND (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (sqlc.arg(search)::text = '' OR subject ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: ListAllActivities :many
-- Daftar SEMUA aktivitas lintas-context (sales+cs+general) — untuk halaman
-- "Activities" top-level (M7). Ownership F3 sama dengan ListActivities (scope_all
-- atau is_own); tanpa context_filter agar semua modul terwakili. Keyset identik.
-- search '' → tak menyaring; selain itu MEMPERSEMPIT subject (BL-6, ILIKE case-
-- insensitive) di ATAS F3 — tak pernah melebarkan baris di luar cakupan.
SELECT * FROM activities
WHERE deleted_at IS NULL
  AND (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND owner_id = sqlc.arg(uid))
  )
  AND (sqlc.arg(search)::text = '' OR subject ILIKE '%' || sqlc.arg(search) || '%')
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
    start_at     = sqlc.narg(start_at),
    end_at       = sqlc.narg(end_at),
    location     = sqlc.narg(location),
    meeting_type = sqlc.narg(meeting_type),
    channel      = sqlc.narg(channel),
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
