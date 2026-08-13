-- contacts.sql — orang di dalam sebuah desa (Contact), 1:N ke accounts. Isolasi
-- WORKSPACE ditegakkan RLS (GUC app.tenant_id di WithTenant, tenant_id di-AND-kan
-- otomatis). Isolasi ANTAR-DESA (F3) TIDAK punya filter sendiri di sini: kontak
-- MEWARISI kepemilikan desa induknya (docs/crm/tasks.md §Modul 3). Maka:
--   - daftar per-desa (ListContactsByAccount) tak menyaring ownership sama sekali —
--     handler sudah menjaga bahwa desa induknya boleh dilihat aktor (loadOwnedAccount);
--     begitu lolos, SEMUA kontak desa itu tampil.
--   - daftar global (ListContacts) menyaring lewat kolom ownership DESA INDUK
--     (JOIN accounts), memakai flag yang SAMA dengan ListAccounts — sumber tunggal
--     AccountsListFilter. "Kontak siapa yang tampil" = "desa siapa yang tampil".
--
-- Maks 1 kontak utama per desa dijaga idx_contacts_primary (partial UNIQUE). Set-
-- primary WAJIB dua langkah dalam satu tx: kosongkan primary lama dulu, baru pasang
-- yang baru — kalau tidak, INSERT/UPDATE bertabrakan dengan index (satu tx, satu
-- tenant, jadi tak ada balapan antar-request di jalur ini).

-- name: CreateContact :one
-- Buat kontak. account_id mengikat ke desa induk (RLS memverifikasi tenant_id =
-- GUC lewat FK + WITH CHECK). is_primary_contact di-set pemanggil SETELAH
-- mengosongkan primary lama (ClearAccountPrimaryContact) agar tak melanggar index.
INSERT INTO contacts (
    tenant_id, account_id, contact_owner, reports_to_id,
    first_name, last_name, salutation, job_title, position_category, contact_role,
    is_primary_contact, is_technical_contact, term_period,
    mobile_phone, whatsapp_number, office_phone, email, preferred_channel,
    mailing_address, city, postal_code,
    email_opt_out, do_not_contact,
    created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(account_id), sqlc.narg(contact_owner), sqlc.narg(reports_to_id),
    sqlc.arg(first_name), sqlc.narg(last_name), sqlc.narg(salutation),
    sqlc.narg(job_title), sqlc.narg(position_category), sqlc.narg(contact_role),
    sqlc.arg(is_primary_contact), sqlc.arg(is_technical_contact), sqlc.narg(term_period),
    sqlc.narg(mobile_phone), sqlc.narg(whatsapp_number), sqlc.narg(office_phone),
    sqlc.narg(email), sqlc.narg(preferred_channel),
    sqlc.narg(mailing_address), sqlc.narg(city), sqlc.narg(postal_code),
    sqlc.arg(email_opt_out), sqlc.arg(do_not_contact),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: GetContact :one
-- Satu kontak hidup. RLS menjamin tenant_id; deleted_at menyembunyikan yang
-- ter-soft-delete. Tak menerapkan ownership — pemanggil (handler) memutuskan lewat
-- desa INDUK apakah aktor boleh membukanya (kontak mewarisi kepemilikan desa).
SELECT * FROM contacts
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListContactsByAccount :many
-- Kontak SATU desa, keyset (created_at DESC, id DESC). TANPA filter ownership:
-- gerbangnya adalah desa induk (handler memvalidasi via loadOwnedAccount sebelum
-- memanggil ini). Kontak utama diangkat ke atas agar penanda primary langsung
-- terlihat tanpa menggeser urutan kronologis baris lainnya.
SELECT * FROM contacts
WHERE account_id = sqlc.arg(account_id)
  AND deleted_at IS NULL
  AND (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
ORDER BY is_primary_contact DESC, created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: ListContacts :many
-- Daftar kontak LINTAS-desa, keyset + filter kepemilikan DESA INDUK (bukan filter
-- kontak sendiri). JOIN accounts membawa kolom ownership desa; flag scope_all/
-- is_sales/is_csm identik dengan ListAccounts (AccountsListFilter) — "kontak siapa
-- yang tampil" diturunkan dari "desa siapa yang tampil", satu kebenaran.
-- Ketiganya false (Support/role kosong/liar) → NOL baris (fail-closed).
-- a.village_name dibawa untuk kolom "Desa" di daftar global (di daftar per-desa
-- redundan — sudah di judul halaman — jadi query per-desa tak mengambilnya).
SELECT c.*, a.village_name FROM contacts c
JOIN accounts a ON a.id = c.account_id AND a.deleted_at IS NULL
WHERE c.deleted_at IS NULL
  AND (c.created_at, c.id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_sales)::boolean AND a.account_owner = sqlc.arg(uid))
      OR (sqlc.arg(is_csm)::boolean AND (a.assigned_csm = sqlc.arg(uid) OR a.backup_csm = sqlc.arg(uid)))
  )
ORDER BY c.created_at DESC, c.id DESC
LIMIT sqlc.arg(page_size);

-- name: UpdateContact :one
-- Sunting kontak. is_primary_contact di-set pemanggil setelah mengosongkan primary
-- lama (ClearAccountPrimaryContact) bila dinaikkan jadi utama. account_id TIDAK
-- diubah di sini — memindahkan kontak antar-desa adalah aksi lain (belum ada).
UPDATE contacts SET
    contact_owner        = sqlc.narg(contact_owner),
    reports_to_id        = sqlc.narg(reports_to_id),
    first_name           = sqlc.arg(first_name),
    last_name            = sqlc.narg(last_name),
    salutation           = sqlc.narg(salutation),
    job_title            = sqlc.narg(job_title),
    position_category    = sqlc.narg(position_category),
    contact_role         = sqlc.narg(contact_role),
    is_primary_contact   = sqlc.arg(is_primary_contact),
    is_technical_contact = sqlc.arg(is_technical_contact),
    term_period          = sqlc.narg(term_period),
    mobile_phone         = sqlc.narg(mobile_phone),
    whatsapp_number      = sqlc.narg(whatsapp_number),
    office_phone         = sqlc.narg(office_phone),
    email                = sqlc.narg(email),
    preferred_channel    = sqlc.narg(preferred_channel),
    mailing_address      = sqlc.narg(mailing_address),
    city                 = sqlc.narg(city),
    postal_code          = sqlc.narg(postal_code),
    email_opt_out        = sqlc.arg(email_opt_out),
    do_not_contact       = sqlc.arg(do_not_contact),
    updated_by           = sqlc.narg(updated_by),
    updated_at           = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: ClearAccountPrimaryContact :exec
-- Lepas penanda utama dari SEMUA kontak hidup satu desa. Dipanggil SEBELUM
-- memasang primary baru (create/update/set-primary) supaya idx_contacts_primary
-- (maks 1 utama per desa) tak pernah dilanggar. Idempotent: nol baris bila belum
-- ada utama.
UPDATE contacts SET is_primary_contact = false, updated_by = sqlc.narg(updated_by), updated_at = now()
WHERE account_id = sqlc.arg(account_id) AND is_primary_contact AND deleted_at IS NULL;

-- name: SetPrimaryContact :exec
-- Angkat SATU kontak jadi utama. Pemanggil WAJIB memanggil ClearAccountPrimaryContact
-- lebih dulu (satu tx) — index memblokir dua utama. account_id ikut di WHERE sebagai
-- sabuk pengaman: kontak yang bukan milik desa itu tak bisa diangkat lewat jalurnya.
UPDATE contacts SET is_primary_contact = true, updated_by = sqlc.narg(updated_by), updated_at = now()
WHERE id = sqlc.arg(id) AND account_id = sqlc.arg(account_id) AND deleted_at IS NULL;

-- name: SoftDeleteContact :exec
-- Soft-delete: baris disembunyikan dari list/get tapi tetap ada (jejak & FK dari
-- entitas lain tak putus). Idempotent: hanya baris hidup. Kontak utama yang dihapus
-- membebaskan slot primary (index-nya partial WHERE deleted_at IS NULL).
UPDATE contacts SET deleted_at = now(), updated_by = sqlc.narg(updated_by)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: CountContactsByAccount :one
-- Jumlah kontak hidup satu desa — untuk badge/ringkasan di detail desa. Murah:
-- idx_contacts_account (partial WHERE deleted_at IS NULL) melayaninya langsung.
SELECT COUNT(*) FROM contacts
WHERE account_id = sqlc.arg(account_id) AND deleted_at IS NULL;
