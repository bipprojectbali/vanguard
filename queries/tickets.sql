-- tickets.sql — Query tiket layanan desa (CRM Modul 6 slice B2). RLS
-- mengisolasi workspace; F3 ownership ditegakkan lewat flag boolean (scope_all,
-- is_own) — satu sumber kebenaran dengan TicketsListFilterFor (ownership.go).
--
-- SLA deadline disimpan saat create (snapshot), bukan dihitung JOIN sla_policies
-- — karena target dapat berubah setelah tiket dibuat (prinsip snapshot, catatan
-- same di 00020_crm_cs_tickets.sql).
--
-- Nomor tiket (#TK-{id}) diformat di layer aplikasi (ticketNumber), bukan kolom.

-- name: CreateTicket :one
-- Buat tiket baru. sla_deadline_at dihitung HANDLER (created_at + menit), bukan
-- dihitung SQL, karena memerlukan nilai SLA policy yang di-fetch lebih dulu.
-- assigned_to NULL = belum ditugaskan (status 'baru').
INSERT INTO tickets (
    tenant_id, account_id,
    subject, description,
    priority, status,
    assigned_to,
    sla_policy_id, sla_deadline_at,
    created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(account_id),
    sqlc.arg(subject), sqlc.narg(description),
    sqlc.arg(priority), 'baru',
    sqlc.narg(assigned_to),
    sqlc.narg(sla_policy_id), sqlc.narg(sla_deadline_at),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: GetTicket :one
-- Satu tiket + nama desa. Dipakai handler UpdateTicketStatus sebelum update
-- untuk validasi keberadaan + F3 (handler memanggil TicketsListFilter.Allows).
SELECT t.*, a.village_name AS account_name
FROM tickets t
JOIN accounts a ON t.account_id = a.id
WHERE t.id = sqlc.arg(id);

-- name: UpdateTicketStatus :one
-- Ubah status tiket. resolved_at diisi otomatis saat status = 'selesai', dan
-- di-NULL-kan pada status lain apa pun — termasuk REOPEN (Selesai→Diproses,
-- BL-38): tiket yang dibuka ulang tak boleh menyimpan resolved_at basi. Invarian:
-- resolved_at terisi IFF status = 'selesai'.
-- assigned_to dapat berubah bersamaan (mis. Support menugaskan dirinya saat terima).
UPDATE tickets
SET
    status      = sqlc.arg(status),
    assigned_to = sqlc.narg(assigned_to),
    resolved_at = CASE
        WHEN sqlc.arg(status) = 'selesai' THEN now()
        ELSE NULL
    END,
    updated_by  = sqlc.narg(updated_by),
    updated_at  = now()
WHERE id = sqlc.arg(id)
RETURNING *;

-- name: ListTickets :many
-- Daftar tiket, keyset (created_at DESC, id DESC) + F3 ownership + filter tab.
--
-- Ownership dikodekan sebagai dua flag boolean (scope_all / is_own) agar query
-- tetap sqlc murni. is_own = UNION kepemilikan desa: account_owner ATAU
-- assigned_csm ATAU backup_csm — satu makna "desa yang ditugaskan padaku".
--   scope_all → lihat semua tiket workspace (Admin, Manager, Support)
--   is_own    → hanya tiket desa yang ditugaskan (Sales, CSM)
-- Keduanya false → OR selalu false → NOL baris (fail-closed).
--
-- Filter tab (flag boolean eksklusif, cukup satu true per request):
--   filter_status '' → semua status; non-'' → cocokkan persis.
--   filter_sla_breached → deadline < now() DAN status bukan selesai.
--   filter_sla_at_risk  → deadline dalam 4 jam ke depan DAN status bukan selesai.
SELECT
    t.id, t.account_id, t.subject, t.priority, t.status,
    t.assigned_to, t.sla_deadline_at, t.resolved_at,
    t.created_at, t.updated_at,
    a.village_name AS account_name,
    u.name AS assigned_to_name
FROM tickets t
JOIN accounts a ON t.account_id = a.id AND a.deleted_at IS NULL
LEFT JOIN users u ON t.assigned_to = u.id
WHERE (t.created_at, t.id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND (
          a.account_owner = sqlc.arg(uid)
          OR a.assigned_csm = sqlc.arg(uid)
          OR a.backup_csm = sqlc.arg(uid)
      ))
  )
  AND (sqlc.arg(filter_status) = '' OR t.status = sqlc.arg(filter_status))
  AND (NOT sqlc.arg(filter_sla_breached)::boolean
       OR (t.sla_deadline_at IS NOT NULL AND t.sla_deadline_at < now() AND t.status <> 'selesai'))
  AND (NOT sqlc.arg(filter_sla_at_risk)::boolean
       OR (t.sla_deadline_at IS NOT NULL AND t.sla_deadline_at > now()
           AND t.sla_deadline_at < now() + INTERVAL '4 hours' AND t.status <> 'selesai'))
  AND (sqlc.arg(search)::text = ''
       OR t.subject ILIKE '%' || sqlc.arg(search) || '%'
       OR a.village_name ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY t.created_at DESC, t.id DESC
LIMIT sqlc.arg(page_size);

-- name: ListTicketsSortByVillage :many
-- BL-157h: sort by a.village_name ("Desa"). SAMA PERSIS filter ListTickets
-- (ownership F3 + tab filter + search) — hanya ORDER BY/keyset beda. NOT NULL
-- (INNER JOIN accounts + deleted_at IS NULL), pola non-nullable text sama
-- ListSubscriptionsSortByVillage (BL-157a).
SELECT
    t.id, t.account_id, t.subject, t.priority, t.status,
    t.assigned_to, t.sla_deadline_at, t.resolved_at,
    t.created_at, t.updated_at,
    a.village_name AS account_name,
    u.name AS assigned_to_name
FROM tickets t
JOIN accounts a ON t.account_id = a.id AND a.deleted_at IS NULL
LEFT JOIN users u ON t.assigned_to = u.id
WHERE (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc'
          AND (a.village_name, t.id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      OR (sqlc.arg(dir)::text = 'desc'
          AND (a.village_name, t.id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
  )
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND (
          a.account_owner = sqlc.arg(uid)
          OR a.assigned_csm = sqlc.arg(uid)
          OR a.backup_csm = sqlc.arg(uid)
      ))
  )
  AND (sqlc.arg(filter_status) = '' OR t.status = sqlc.arg(filter_status))
  AND (NOT sqlc.arg(filter_sla_breached)::boolean
       OR (t.sla_deadline_at IS NOT NULL AND t.sla_deadline_at < now() AND t.status <> 'selesai'))
  AND (NOT sqlc.arg(filter_sla_at_risk)::boolean
       OR (t.sla_deadline_at IS NOT NULL AND t.sla_deadline_at > now()
           AND t.sla_deadline_at < now() + INTERVAL '4 hours' AND t.status <> 'selesai'))
  AND (sqlc.arg(search)::text = ''
       OR t.subject ILIKE '%' || sqlc.arg(search) || '%'
       OR a.village_name ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN a.village_name END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN a.village_name END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN t.id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN t.id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListTicketsSortBySubject :many
-- BL-157h: sort by t.subject ("Subjek"). NOT NULL, pola sama SortByVillage.
SELECT
    t.id, t.account_id, t.subject, t.priority, t.status,
    t.assigned_to, t.sla_deadline_at, t.resolved_at,
    t.created_at, t.updated_at,
    a.village_name AS account_name,
    u.name AS assigned_to_name
FROM tickets t
JOIN accounts a ON t.account_id = a.id AND a.deleted_at IS NULL
LEFT JOIN users u ON t.assigned_to = u.id
WHERE (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc'
          AND (t.subject, t.id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      OR (sqlc.arg(dir)::text = 'desc'
          AND (t.subject, t.id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
  )
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND (
          a.account_owner = sqlc.arg(uid)
          OR a.assigned_csm = sqlc.arg(uid)
          OR a.backup_csm = sqlc.arg(uid)
      ))
  )
  AND (sqlc.arg(filter_status) = '' OR t.status = sqlc.arg(filter_status))
  AND (NOT sqlc.arg(filter_sla_breached)::boolean
       OR (t.sla_deadline_at IS NOT NULL AND t.sla_deadline_at < now() AND t.status <> 'selesai'))
  AND (NOT sqlc.arg(filter_sla_at_risk)::boolean
       OR (t.sla_deadline_at IS NOT NULL AND t.sla_deadline_at > now()
           AND t.sla_deadline_at < now() + INTERVAL '4 hours' AND t.status <> 'selesai'))
  AND (sqlc.arg(search)::text = ''
       OR t.subject ILIKE '%' || sqlc.arg(search) || '%'
       OR a.village_name ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN t.subject END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN t.subject END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN t.id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN t.id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListTicketsSortByPriority :many
-- BL-157h: sort by t.priority ("Prioritas" — RAW rendah/sedang/tinggi,
-- alfabetis; bukan bobot urgensi). NOT NULL, pola sama SortByVillage.
SELECT
    t.id, t.account_id, t.subject, t.priority, t.status,
    t.assigned_to, t.sla_deadline_at, t.resolved_at,
    t.created_at, t.updated_at,
    a.village_name AS account_name,
    u.name AS assigned_to_name
FROM tickets t
JOIN accounts a ON t.account_id = a.id AND a.deleted_at IS NULL
LEFT JOIN users u ON t.assigned_to = u.id
WHERE (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc'
          AND (t.priority, t.id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      OR (sqlc.arg(dir)::text = 'desc'
          AND (t.priority, t.id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
  )
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND (
          a.account_owner = sqlc.arg(uid)
          OR a.assigned_csm = sqlc.arg(uid)
          OR a.backup_csm = sqlc.arg(uid)
      ))
  )
  AND (sqlc.arg(filter_status) = '' OR t.status = sqlc.arg(filter_status))
  AND (NOT sqlc.arg(filter_sla_breached)::boolean
       OR (t.sla_deadline_at IS NOT NULL AND t.sla_deadline_at < now() AND t.status <> 'selesai'))
  AND (NOT sqlc.arg(filter_sla_at_risk)::boolean
       OR (t.sla_deadline_at IS NOT NULL AND t.sla_deadline_at > now()
           AND t.sla_deadline_at < now() + INTERVAL '4 hours' AND t.status <> 'selesai'))
  AND (sqlc.arg(search)::text = ''
       OR t.subject ILIKE '%' || sqlc.arg(search) || '%'
       OR a.village_name ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN t.priority END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN t.priority END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN t.id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN t.id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListTicketsSortByStatus :many
-- BL-157h: sort by t.status ("Status" — RAW baru/diproses/menunggu/selesai).
-- Berbeda dari Renewals (BL-157g, Status dikecualikan krn derivasi penuh):
-- di sini status ADALAH kolom mentah, bukan turunan, jadi aman disortir
-- langsung. NOT NULL, pola sama SortByVillage.
SELECT
    t.id, t.account_id, t.subject, t.priority, t.status,
    t.assigned_to, t.sla_deadline_at, t.resolved_at,
    t.created_at, t.updated_at,
    a.village_name AS account_name,
    u.name AS assigned_to_name
FROM tickets t
JOIN accounts a ON t.account_id = a.id AND a.deleted_at IS NULL
LEFT JOIN users u ON t.assigned_to = u.id
WHERE (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc'
          AND (t.status, t.id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      OR (sqlc.arg(dir)::text = 'desc'
          AND (t.status, t.id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
  )
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND (
          a.account_owner = sqlc.arg(uid)
          OR a.assigned_csm = sqlc.arg(uid)
          OR a.backup_csm = sqlc.arg(uid)
      ))
  )
  AND (sqlc.arg(filter_status) = '' OR t.status = sqlc.arg(filter_status))
  AND (NOT sqlc.arg(filter_sla_breached)::boolean
       OR (t.sla_deadline_at IS NOT NULL AND t.sla_deadline_at < now() AND t.status <> 'selesai'))
  AND (NOT sqlc.arg(filter_sla_at_risk)::boolean
       OR (t.sla_deadline_at IS NOT NULL AND t.sla_deadline_at > now()
           AND t.sla_deadline_at < now() + INTERVAL '4 hours' AND t.status <> 'selesai'))
  AND (sqlc.arg(search)::text = ''
       OR t.subject ILIKE '%' || sqlc.arg(search) || '%'
       OR a.village_name ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN t.status END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN t.status END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN t.id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN t.id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListTicketsSortByAgent :many
-- BL-157h: sort by u.name ("Agen" — assigned_to_name). NULLABLE (assigned_to
-- NULL = belum ditugaskan). Kunci sort PLAIN u.name (bukan COALESCE dgn
-- email seperti ListSubscriptionsSortByCsm) — ticketRowView menampilkan
-- AssignedToName apa adanya tanpa fallback email, jadi urutan harus sama
-- dgn yang ditampilkan. Pola null-aware sama ListSubscriptionsSortByPlan.
SELECT
    t.id, t.account_id, t.subject, t.priority, t.status,
    t.assigned_to, t.sla_deadline_at, t.resolved_at,
    t.created_at, t.updated_at,
    a.village_name AS account_name,
    u.name AS assigned_to_name
FROM tickets t
JOIN accounts a ON t.account_id = a.id AND a.deleted_at IS NULL
LEFT JOIN users u ON t.assigned_to = u.id
WHERE (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc' AND (
          (NOT sqlc.arg(cursor_is_null)::boolean
           AND (u.name IS NULL OR (u.name, t.id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint)))
          OR (sqlc.arg(cursor_is_null)::boolean AND u.name IS NULL AND t.id > sqlc.arg(cursor_id)::bigint)
      ))
      OR (sqlc.arg(dir)::text = 'desc' AND (
          (sqlc.arg(cursor_is_null)::boolean
           AND (u.name IS NOT NULL OR t.id < sqlc.arg(cursor_id)::bigint))
          OR (NOT sqlc.arg(cursor_is_null)::boolean AND u.name IS NOT NULL
              AND (u.name, t.id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      ))
  )
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND (
          a.account_owner = sqlc.arg(uid)
          OR a.assigned_csm = sqlc.arg(uid)
          OR a.backup_csm = sqlc.arg(uid)
      ))
  )
  AND (sqlc.arg(filter_status) = '' OR t.status = sqlc.arg(filter_status))
  AND (NOT sqlc.arg(filter_sla_breached)::boolean
       OR (t.sla_deadline_at IS NOT NULL AND t.sla_deadline_at < now() AND t.status <> 'selesai'))
  AND (NOT sqlc.arg(filter_sla_at_risk)::boolean
       OR (t.sla_deadline_at IS NOT NULL AND t.sla_deadline_at > now()
           AND t.sla_deadline_at < now() + INTERVAL '4 hours' AND t.status <> 'selesai'))
  AND (sqlc.arg(search)::text = ''
       OR t.subject ILIKE '%' || sqlc.arg(search) || '%'
       OR a.village_name ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN u.name END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN u.name END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN t.id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN t.id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListTicketsSortBySla :many
-- BL-157h: sort by t.sla_deadline_at ("SLA" — RAW deadline, bukan label
-- turunan Terpenuhi/Terlanggar/"Nj Mm lagi" — sama prinsip dgn SortByStatus
-- Subscriptions: sortir sumbu mentah, jangan duplikasi derivasi ke SQL).
-- NULLABLE (tiket belum bersla_policy). Pola null-aware sama SortByMrr,
-- tipe timestamptz bukan numeric.
SELECT
    t.id, t.account_id, t.subject, t.priority, t.status,
    t.assigned_to, t.sla_deadline_at, t.resolved_at,
    t.created_at, t.updated_at,
    a.village_name AS account_name,
    u.name AS assigned_to_name
FROM tickets t
JOIN accounts a ON t.account_id = a.id AND a.deleted_at IS NULL
LEFT JOIN users u ON t.assigned_to = u.id
WHERE (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc' AND (
          (NOT sqlc.arg(cursor_is_null)::boolean
           AND (t.sla_deadline_at IS NULL OR (t.sla_deadline_at, t.id) > (sqlc.arg(cursor_val)::timestamptz, sqlc.arg(cursor_id)::bigint)))
          OR (sqlc.arg(cursor_is_null)::boolean AND t.sla_deadline_at IS NULL AND t.id > sqlc.arg(cursor_id)::bigint)
      ))
      OR (sqlc.arg(dir)::text = 'desc' AND (
          (sqlc.arg(cursor_is_null)::boolean
           AND (t.sla_deadline_at IS NOT NULL OR t.id < sqlc.arg(cursor_id)::bigint))
          OR (NOT sqlc.arg(cursor_is_null)::boolean AND t.sla_deadline_at IS NOT NULL
              AND (t.sla_deadline_at, t.id) < (sqlc.arg(cursor_val)::timestamptz, sqlc.arg(cursor_id)::bigint))
      ))
  )
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND (
          a.account_owner = sqlc.arg(uid)
          OR a.assigned_csm = sqlc.arg(uid)
          OR a.backup_csm = sqlc.arg(uid)
      ))
  )
  AND (sqlc.arg(filter_status) = '' OR t.status = sqlc.arg(filter_status))
  AND (NOT sqlc.arg(filter_sla_breached)::boolean
       OR (t.sla_deadline_at IS NOT NULL AND t.sla_deadline_at < now() AND t.status <> 'selesai'))
  AND (NOT sqlc.arg(filter_sla_at_risk)::boolean
       OR (t.sla_deadline_at IS NOT NULL AND t.sla_deadline_at > now()
           AND t.sla_deadline_at < now() + INTERVAL '4 hours' AND t.status <> 'selesai'))
  AND (sqlc.arg(search)::text = ''
       OR t.subject ILIKE '%' || sqlc.arg(search) || '%'
       OR a.village_name ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN t.sla_deadline_at END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN t.sla_deadline_at END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN t.id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN t.id END DESC
LIMIT sqlc.arg(page_size);

-- name: CountTicketKPIs :one
-- Agregat KPI header halaman /tickets. Cakupan scope sama persis ListTickets.
-- COUNT ... FILTER = PostgreSQL aggregate filter clause (SQL:2003, native pg).
-- uid dioper walau scope_all=true (diabaikan dalam kasus itu).
--
-- resolved_today menggunakan date_trunc('day', now()) — UTC day boundary; cukup
-- untuk monitoring operasional. Timezone workspace (gotcha #14) tidak diterapkan
-- di v1 untuk menjaga query tetap sederhana.
SELECT
    COUNT(*) FILTER (WHERE t.status <> 'selesai')
        AS open_count,
    COUNT(*) FILTER (WHERE t.status = 'baru' AND t.assigned_to IS NULL)
        AS unassigned_count,
    COUNT(*) FILTER (WHERE t.sla_deadline_at IS NOT NULL
                       AND t.sla_deadline_at > now()
                       AND t.sla_deadline_at < now() + INTERVAL '4 hours'
                       AND t.status <> 'selesai')
        AS at_risk_count,
    COUNT(*) FILTER (WHERE t.sla_deadline_at IS NOT NULL
                       AND t.sla_deadline_at < now()
                       AND t.status <> 'selesai')
        AS breached_count,
    COUNT(*) FILTER (WHERE t.status = 'selesai'
                       AND t.resolved_at >= date_trunc('day', now()))
        AS resolved_today
FROM tickets t
JOIN accounts a ON t.account_id = a.id AND a.deleted_at IS NULL
WHERE (
    sqlc.arg(scope_all)::boolean
    OR (sqlc.arg(is_own)::boolean AND (
        a.account_owner = sqlc.arg(uid)
        OR a.assigned_csm = sqlc.arg(uid)
        OR a.backup_csm = sqlc.arg(uid)
    ))
);

-- name: CountTicketsByAccount :one
-- Ringkasan tiket SATU desa (chip "Terkait" di detail Account): terbuka + breach
-- dalam satu baris, tanpa filter ownership (gerbangnya desa induk, pola sama
-- CountContactsByAccount). Predikat breach SAMA PERSIS dgn CountTicketKPIs
-- (tickets TANPA kolom deleted_at — lihat header file ini).
SELECT
    COUNT(*) FILTER (WHERE t.status <> 'selesai')
        AS open_count,
    COUNT(*) FILTER (WHERE t.sla_deadline_at IS NOT NULL
                       AND t.sla_deadline_at < now()
                       AND t.status <> 'selesai')
        AS breached_count
FROM tickets t
WHERE t.account_id = sqlc.arg(account_id);
