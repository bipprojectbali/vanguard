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
