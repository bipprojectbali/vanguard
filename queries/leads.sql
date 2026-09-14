-- leads.sql — funnel Sales (Lead). Isolasi WORKSPACE ditegakkan RLS (GUC
-- app.tenant_id di WithTenant); isolasi ANTAR-DESA (F3) ditegakkan di layer query
-- lewat flag ownership di ListLeads — lihat internal/db/ownership.go. Ownership
-- lead memakai SATU kolom (lead_owner), beda dari accounts yang tiga kolom.
--
-- entity_code (LEAD-001) dialokasikan di GenerateEntityCode DALAM tx create, lalu
-- dioper ke CreateLead. Field region/kontak mentah (province/mobile_phone/dst)
-- karena lead belum jadi account — konversi yang memindahkannya.

-- name: CreateLead :one
-- Buat lead. tenant_id di-set eksplisit (RLS WITH CHECK memverifikasinya = GUC).
-- entity_code sudah dirakit pemanggil (GenerateEntityCode).
INSERT INTO leads (
    tenant_id, entity_code, lead_name, lead_owner,
    contact_person, job_title, lead_source,
    lead_status, rating, unqualified_reason, estimated_value,
    district_id, mobile_phone, whatsapp, email,
    created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(entity_code), sqlc.arg(lead_name), sqlc.narg(lead_owner),
    sqlc.narg(contact_person), sqlc.narg(job_title), sqlc.narg(lead_source),
    sqlc.arg(lead_status), sqlc.narg(rating), sqlc.narg(unqualified_reason),
    sqlc.narg(estimated_value),
    sqlc.narg(district_id),
    sqlc.narg(mobile_phone), sqlc.narg(whatsapp), sqlc.narg(email),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: GetLead :one
-- Satu lead hidup. RLS menjamin tenant_id; filter deleted_at menyembunyikan yang
-- ter-soft-delete. Ownership diputuskan handler (LeadsListFilter.Allows) atas baris.
SELECT * FROM leads
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListLeads :many
-- Daftar lead, keyset (created_at DESC, id DESC) + filter ownership F3 + tab.
--
-- Ownership sebagai dua flag boolean (bukan SQL dinamis) supaya query tetap sqlc
-- murni & ter-scan typed — sumber SATU dengan LeadsListFilter (ownership.go):
--   scope_all → lihat semua (Admin/Manager)
--   is_own    → hanya lead_owner = uid (Sales)
-- Keduanya false (Support/role kosong/liar) → NOL baris (fail-closed).
--
-- Tab tambahan (ortogonal dari ownership):
--   mine_only  → paksa lead_owner = uid (tab "My Leads", walau aktor scope_all)
--   status_filter '' → semua status; selain itu = filter satu status ("Unqualified")
--
-- search '' → tak menyaring; selain itu MEMPERSEMPIT (ILIKE substring, case-
-- insensitive) di ATAS ownership+tab — tak pernah melebarkan baris yang boleh
-- dilihat aktor. Hanya kolom yang DITAMPILKAN tak-tersamar di daftar (lead_name +
-- entity_code); PII (telepon/email) tak ikut agar search bukan jalur enumerasi
-- data tersamar. Tetap keyset+LIMIT. Trigram/index ditunda (dataset kecil).
SELECT * FROM leads
WHERE deleted_at IS NULL
  AND (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND lead_owner = sqlc.arg(uid))
  )
  AND (NOT sqlc.arg(mine_only)::boolean OR lead_owner = sqlc.arg(uid))
  AND (sqlc.arg(status_filter)::text = '' OR lead_status = sqlc.arg(status_filter)::text)
  AND (
      sqlc.arg(search)::text = ''
      OR lead_name ILIKE '%' || sqlc.arg(search) || '%'
      OR entity_code ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: ListLeadsSortByName :many
-- BL-157b (fondasi sort per kolom Leads): SAMA PERSIS filter ListLeads
-- (ownership F3 + mine_only + status_filter + search) — hanya ORDER BY/keyset
-- yang beda, diurut lead_name (bukan created_at). lead_name TIDAK NULLABLE →
-- kloning pola ListSubscriptionsSortByVillage (tanpa kerumitan NULL). Sort
-- HANYA berdasar lead_name (baris bold di sel), BUKAN gabungan dgn
-- contact_person (subteks) — mirror keputusan "Desa" BL-157a.
SELECT * FROM leads
WHERE deleted_at IS NULL
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc'
          AND (lead_name, id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      OR (sqlc.arg(dir)::text = 'desc'
          AND (lead_name, id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
  )
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND lead_owner = sqlc.arg(uid))
  )
  AND (NOT sqlc.arg(mine_only)::boolean OR lead_owner = sqlc.arg(uid))
  AND (sqlc.arg(status_filter)::text = '' OR lead_status = sqlc.arg(status_filter)::text)
  AND (
      sqlc.arg(search)::text = ''
      OR lead_name ILIKE '%' || sqlc.arg(search) || '%'
      OR entity_code ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN lead_name END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN lead_name END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListLeadsSortByStatus :many
-- BL-157b: sort by lead_status ("Status" — RAW enum New/Contacted/Qualified/
-- Unqualified/Converted, alfabetis; tak menduplikasi urutan tingkat ke SQL,
-- mirror keputusan "Masa Berlaku" BL-157a). lead_status NOT NULL → kloning
-- PERSIS pola ListLeadsSortByName.
SELECT * FROM leads
WHERE deleted_at IS NULL
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc'
          AND (lead_status, id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      OR (sqlc.arg(dir)::text = 'desc'
          AND (lead_status, id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
  )
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND lead_owner = sqlc.arg(uid))
  )
  AND (NOT sqlc.arg(mine_only)::boolean OR lead_owner = sqlc.arg(uid))
  AND (sqlc.arg(status_filter)::text = '' OR lead_status = sqlc.arg(status_filter)::text)
  AND (
      sqlc.arg(search)::text = ''
      OR lead_name ILIKE '%' || sqlc.arg(search) || '%'
      OR entity_code ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN lead_status END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN lead_status END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListLeadsSortByCode :many
-- BL-157b: sort by entity_code ("Kode"). NULLABLE (entity_code diisi
-- GenerateEntityCode saat create, tapi kolom tetap nullable di skema). Pola
-- null-aware SAMA dgn ListSubscriptionsSortByPlan — cursor_is_null menandai
-- kelompok NULL/non-NULL lintas-request; NULLS default Postgres (ASC=LAST,
-- DESC=FIRST).
SELECT * FROM leads
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
      OR (sqlc.arg(is_own)::boolean AND lead_owner = sqlc.arg(uid))
  )
  AND (NOT sqlc.arg(mine_only)::boolean OR lead_owner = sqlc.arg(uid))
  AND (sqlc.arg(status_filter)::text = '' OR lead_status = sqlc.arg(status_filter)::text)
  AND (
      sqlc.arg(search)::text = ''
      OR lead_name ILIKE '%' || sqlc.arg(search) || '%'
      OR entity_code ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN entity_code END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN entity_code END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListLeadsSortBySource :many
-- BL-157b: sort by lead_source ("Sumber"). NULLABLE. Pola null-aware PERSIS
-- ListLeadsSortByCode, kolom beda.
SELECT * FROM leads
WHERE deleted_at IS NULL
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc' AND (
          (NOT sqlc.arg(cursor_is_null)::boolean
           AND (lead_source IS NULL OR (lead_source, id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint)))
          OR (sqlc.arg(cursor_is_null)::boolean AND lead_source IS NULL AND id > sqlc.arg(cursor_id)::bigint)
      ))
      OR (sqlc.arg(dir)::text = 'desc' AND (
          (sqlc.arg(cursor_is_null)::boolean
           AND (lead_source IS NOT NULL OR id < sqlc.arg(cursor_id)::bigint))
          OR (NOT sqlc.arg(cursor_is_null)::boolean AND lead_source IS NOT NULL
              AND (lead_source, id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      ))
  )
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND lead_owner = sqlc.arg(uid))
  )
  AND (NOT sqlc.arg(mine_only)::boolean OR lead_owner = sqlc.arg(uid))
  AND (sqlc.arg(status_filter)::text = '' OR lead_status = sqlc.arg(status_filter)::text)
  AND (
      sqlc.arg(search)::text = ''
      OR lead_name ILIKE '%' || sqlc.arg(search) || '%'
      OR entity_code ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN lead_source END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN lead_source END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListLeadsSortByRating :many
-- BL-157b: sort by rating ("Rating" — RAW enum Cold/Hot/Warm, alfabetis;
-- keputusan user: BUKAN urutan tingkat Hot→Warm→Cold, mirror keputusan
-- "Masa Berlaku" BL-157a — tak menduplikasi logika prioritas ke SQL).
-- NULLABLE. Pola null-aware PERSIS ListLeadsSortByCode.
SELECT * FROM leads
WHERE deleted_at IS NULL
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc' AND (
          (NOT sqlc.arg(cursor_is_null)::boolean
           AND (rating IS NULL OR (rating, id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint)))
          OR (sqlc.arg(cursor_is_null)::boolean AND rating IS NULL AND id > sqlc.arg(cursor_id)::bigint)
      ))
      OR (sqlc.arg(dir)::text = 'desc' AND (
          (sqlc.arg(cursor_is_null)::boolean
           AND (rating IS NOT NULL OR id < sqlc.arg(cursor_id)::bigint))
          OR (NOT sqlc.arg(cursor_is_null)::boolean AND rating IS NOT NULL
              AND (rating, id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      ))
  )
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND lead_owner = sqlc.arg(uid))
  )
  AND (NOT sqlc.arg(mine_only)::boolean OR lead_owner = sqlc.arg(uid))
  AND (sqlc.arg(status_filter)::text = '' OR lead_status = sqlc.arg(status_filter)::text)
  AND (
      sqlc.arg(search)::text = ''
      OR lead_name ILIKE '%' || sqlc.arg(search) || '%'
      OR entity_code ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN rating END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN rating END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListLeadsSortByValue :many
-- BL-157b: sort by estimated_value ("Estimasi"). NULLABLE numeric. Kunci sort
-- memakai nilai ASLI (tak ter-mask) — F4 (maskARR) hanya menyamarkan TAMPILAN
-- di handler, mirror presedan MRR BL-157a (ListSubscriptionsSortByMrr). Pola
-- null-aware SAMA dgn ListLeadsSortByCode, tipe kolom numeric bukan text.
SELECT * FROM leads
WHERE deleted_at IS NULL
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc' AND (
          (NOT sqlc.arg(cursor_is_null)::boolean
           AND (estimated_value IS NULL OR (estimated_value, id) > (sqlc.arg(cursor_val)::numeric, sqlc.arg(cursor_id)::bigint)))
          OR (sqlc.arg(cursor_is_null)::boolean AND estimated_value IS NULL AND id > sqlc.arg(cursor_id)::bigint)
      ))
      OR (sqlc.arg(dir)::text = 'desc' AND (
          (sqlc.arg(cursor_is_null)::boolean
           AND (estimated_value IS NOT NULL OR id < sqlc.arg(cursor_id)::bigint))
          OR (NOT sqlc.arg(cursor_is_null)::boolean AND estimated_value IS NOT NULL
              AND (estimated_value, id) < (sqlc.arg(cursor_val)::numeric, sqlc.arg(cursor_id)::bigint))
      ))
  )
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND lead_owner = sqlc.arg(uid))
  )
  AND (NOT sqlc.arg(mine_only)::boolean OR lead_owner = sqlc.arg(uid))
  AND (sqlc.arg(status_filter)::text = '' OR lead_status = sqlc.arg(status_filter)::text)
  AND (
      sqlc.arg(search)::text = ''
      OR lead_name ILIKE '%' || sqlc.arg(search) || '%'
      OR entity_code ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN estimated_value END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN estimated_value END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN id END DESC
LIMIT sqlc.arg(page_size);

-- name: ListLeadsSortByOwner :many
-- BL-157b: sort by CSM Pemilik (lead_owner). Kunci sort HARUS
-- COALESCE(NULLIF(u.name,''), u.email) — PERSIS logika tampil ownerName/
-- memberNameMap (sales_leads_detail.go: nama bila terisi, else email) — agar
-- urutan tak menyimpang dari yang ditampilkan. NULLABLE (lead_owner ON DELETE
-- SET NULL). LEFT JOIN users: baris tanpa owner ATAU owner terhapus →
-- owner_key NULL, masuk kelompok NULL (default Postgres). Mirror PERSIS
-- ListSubscriptionsSortByCsm.
SELECT leads.* FROM leads
LEFT JOIN users u ON u.id = lead_owner
WHERE leads.deleted_at IS NULL
  AND (
      NOT sqlc.arg(has_cursor)::boolean
      OR (sqlc.arg(dir)::text = 'asc' AND (
          (NOT sqlc.arg(cursor_is_null)::boolean
           AND (COALESCE(NULLIF(u.name, ''), u.email) IS NULL
                OR (COALESCE(NULLIF(u.name, ''), u.email), leads.id) > (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint)))
          OR (sqlc.arg(cursor_is_null)::boolean
              AND COALESCE(NULLIF(u.name, ''), u.email) IS NULL AND leads.id > sqlc.arg(cursor_id)::bigint)
      ))
      OR (sqlc.arg(dir)::text = 'desc' AND (
          (sqlc.arg(cursor_is_null)::boolean
           AND (COALESCE(NULLIF(u.name, ''), u.email) IS NOT NULL OR leads.id < sqlc.arg(cursor_id)::bigint))
          OR (NOT sqlc.arg(cursor_is_null)::boolean AND COALESCE(NULLIF(u.name, ''), u.email) IS NOT NULL
              AND (COALESCE(NULLIF(u.name, ''), u.email), leads.id) < (sqlc.arg(cursor_val)::text, sqlc.arg(cursor_id)::bigint))
      ))
  )
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_own)::boolean AND lead_owner = sqlc.arg(uid))
  )
  AND (NOT sqlc.arg(mine_only)::boolean OR lead_owner = sqlc.arg(uid))
  AND (sqlc.arg(status_filter)::text = '' OR lead_status = sqlc.arg(status_filter)::text)
  AND (
      sqlc.arg(search)::text = ''
      OR lead_name ILIKE '%' || sqlc.arg(search) || '%'
      OR entity_code ILIKE '%' || sqlc.arg(search) || '%'
  )
ORDER BY
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN COALESCE(NULLIF(u.name, ''), u.email) END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN COALESCE(NULLIF(u.name, ''), u.email) END DESC,
  CASE WHEN sqlc.arg(dir)::text = 'asc'  THEN leads.id END ASC,
  CASE WHEN sqlc.arg(dir)::text = 'desc' THEN leads.id END DESC
LIMIT sqlc.arg(page_size);

-- name: UpdateLead :one
-- Sunting profil & kualifikasi lead. entity_code tak diubah (kode identitas yang
-- dikutip). converted_* TAK disentuh di sini — itu efek konversi (ConvertLead),
-- bukan edit biasa.
UPDATE leads SET
    lead_name          = sqlc.arg(lead_name),
    lead_owner         = sqlc.narg(lead_owner),
    contact_person     = sqlc.narg(contact_person),
    job_title          = sqlc.narg(job_title),
    lead_source        = sqlc.narg(lead_source),
    lead_status        = sqlc.arg(lead_status),
    rating             = sqlc.narg(rating),
    unqualified_reason = sqlc.narg(unqualified_reason),
    estimated_value    = sqlc.narg(estimated_value),
    district_id        = sqlc.narg(district_id),
    mobile_phone       = sqlc.narg(mobile_phone),
    whatsapp           = sqlc.narg(whatsapp),
    email              = sqlc.narg(email),
    updated_by         = sqlc.narg(updated_by),
    updated_at         = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: UpdateLeadStatus :exec
-- BL-83: transisi STATUS lead sebagai aksi tersendiri (bukan efek edit profil),
-- cermin UpdateDealStage. Hanya lead_status + unqualified_reason (terkopel status
-- Unqualified, BL-80) yang disentuh — profil lead tak diubah di sini. Transisi
-- BEBAS antar-status manual (keputusan BL-83); 'Converted' TAK dapat dicapai lewat
-- jalur ini: guard `AND NOT converted` menolak baris hasil konversi agar invariant
-- "converted = terminal" tak bisa dipalsukan (handler juga menyembunyikan kontrol).
UPDATE leads SET
    lead_status        = sqlc.arg(lead_status),
    unqualified_reason = sqlc.narg(unqualified_reason),
    updated_by         = sqlc.narg(updated_by),
    updated_at         = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL AND NOT converted;

-- name: MarkLeadConverted :exec
-- Tautkan hasil konversi ke lead + kunci statusnya. Dipanggil DALAM tx konversi
-- (bersama INSERT account/contact/deal) → gagal-sebagian rollback penuh. Guard
-- "hanya Qualified & belum converted" ada di WHERE agar konversi ganda idempotent-
-- aman: baris yang sudah converted tak ter-update lagi.
UPDATE leads SET
    converted            = true,
    converted_account_id = sqlc.arg(converted_account_id),
    converted_contact_id = sqlc.narg(converted_contact_id),
    converted_deal_id    = sqlc.arg(converted_deal_id),
    converted_at         = now(),
    lead_status          = 'Converted',
    updated_by           = sqlc.narg(updated_by),
    updated_at           = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
  AND lead_status = 'Qualified' AND NOT converted;

-- name: SoftDeleteLead :exec
-- Soft-delete: baris disembunyikan dari list/get tapi tetap ada (jejak & FK
-- converted_* tak putus). Idempotent: hanya baris hidup.
UPDATE leads SET deleted_at = now(), updated_by = sqlc.narg(updated_by)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;
