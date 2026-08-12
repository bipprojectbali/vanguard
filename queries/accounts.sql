-- accounts.sql — hub Desa (Account). Isolasi WORKSPACE ditegakkan RLS (GUC
-- app.tenant_id di WithTenant); isolasi ANTAR-DESA (F3) ditegakkan di layer query
-- lewat flag ownership di ListAccounts — lihat internal/db/ownership.go.
--
-- entity_code (DESA-001) dialokasikan di GenerateEntityCode DALAM tx create, lalu
-- dioper ke CreateAccount; village_code (Kode Kemendagri) datang dari user & boleh
-- kosong. Dua kode berbeda asal, keduanya disimpan di baris.

-- name: CreateAccount :one
-- Buat desa. tenant_id di-set eksplisit (RLS WITH CHECK memverifikasinya = GUC).
-- entity_code sudah dirakit pemanggil (GenerateEntityCode) — INSERT-nya dijaga
-- unik oleh idx_accounts_entity_code bila format diubah bertabrakan.
INSERT INTO accounts (
    tenant_id, entity_code, village_name, village_code, account_type,
    account_owner, assigned_csm, backup_csm,
    website, description,
    province, regency, district, village_address, postal_code, territory,
    village_status, village_classification, population, hamlets_count, village_budget,
    contact_phone, office_phone, office_email,
    created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(entity_code), sqlc.arg(village_name),
    sqlc.narg(village_code), sqlc.arg(account_type),
    sqlc.narg(account_owner), sqlc.narg(assigned_csm), sqlc.narg(backup_csm),
    sqlc.narg(website), sqlc.narg(description),
    sqlc.narg(province), sqlc.narg(regency), sqlc.narg(district),
    sqlc.narg(village_address), sqlc.narg(postal_code), sqlc.narg(territory),
    sqlc.narg(village_status), sqlc.narg(village_classification),
    sqlc.narg(population), sqlc.narg(hamlets_count), sqlc.narg(village_budget),
    sqlc.narg(contact_phone), sqlc.narg(office_phone), sqlc.narg(office_email),
    sqlc.narg(created_by)
)
RETURNING *;

-- name: GetAccount :one
-- Satu desa hidup. RLS menjamin tenant_id; filter deleted_at menyembunyikan yang
-- ter-soft-delete. Tak menerapkan ownership — pemanggil (handler) yang memutuskan
-- apakah aktor boleh membuka baris ini (detail bisa dibuka lewat tautan langsung).
SELECT * FROM accounts
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListAccounts :many
-- Daftar desa, keyset (created_at DESC, id DESC) + filter ownership F3.
--
-- Ownership dikodekan sebagai tiga flag boolean (bukan fragmen SQL dinamis) supaya
-- query tetap sqlc murni & ter-scan typed. Keputusan cakupannya SATU sumber dengan
-- AccountsOwnershipClause: keduanya diturunkan dari AccountsScopeFor lewat
-- AccountsListFilter — diuji bersama agar tak bercabang.
--   scope_all → lihat semua (Admin/Manager)
--   is_sales  → hanya account_owner = uid
--   is_csm    → assigned_csm = uid ATAU backup_csm = uid
-- Ketiganya false (Support/role kosong/liar) → OR selalu false → NOL baris
-- (fail-closed, bukan bocor). uid tetap dioper walau scope_all (diabaikan).
--
-- unowned = tapis SEJAJAR (tab "Belum ada Owner"): saring account_owner IS NULL
-- DI ATAS blok ownership, bukan menggantinya. false → NOT false = TRUE → tak
-- membatasi; true → hanya desa tanpa pemilik. Inheren cakupan-all: pemakai
-- ber-scope 'own' tak pernah punya baris owner-kosong, jadi handler hanya
-- menyalakannya untuk peran ScopeAll (Manager/Admin).
SELECT * FROM accounts
WHERE deleted_at IS NULL
  AND (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_sales)::boolean AND account_owner = sqlc.arg(uid))
      OR (sqlc.arg(is_csm)::boolean AND (assigned_csm = sqlc.arg(uid) OR backup_csm = sqlc.arg(uid)))
  )
  AND (NOT sqlc.arg(unowned)::boolean OR account_owner IS NULL)
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_size);

-- name: UpdateAccount :one
-- Sunting profil desa. entity_code & village_code tak diubah di sini (kode identitas
-- yang dikutip; village_code punya jalur khusus bila kelak perlu). Penugasan
-- (owner/CSM) juga TERPISAH (AssignAccountCSM) agar perubahan wewenang terlihat
-- sebagai aksi tersendiri, bukan efek samping edit profil.
UPDATE accounts SET
    village_name          = sqlc.arg(village_name),
    account_type          = sqlc.arg(account_type),
    website               = sqlc.narg(website),
    description           = sqlc.narg(description),
    province              = sqlc.narg(province),
    regency               = sqlc.narg(regency),
    district              = sqlc.narg(district),
    village_address       = sqlc.narg(village_address),
    postal_code           = sqlc.narg(postal_code),
    territory             = sqlc.narg(territory),
    village_status        = sqlc.narg(village_status),
    village_classification = sqlc.narg(village_classification),
    population            = sqlc.narg(population),
    hamlets_count         = sqlc.narg(hamlets_count),
    village_budget        = sqlc.narg(village_budget),
    contact_phone         = sqlc.narg(contact_phone),
    office_phone          = sqlc.narg(office_phone),
    office_email          = sqlc.narg(office_email),
    updated_by            = sqlc.narg(updated_by),
    updated_at            = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL
RETURNING *;

-- name: AssignAccountCSM :exec
-- Penugasan CSM (binaan + cadangan) — jalur terpisah dari edit profil (§8.1).
-- NULL = lepas penugasan. updated_by/at ikut agar jejaknya jelas.
UPDATE accounts SET
    assigned_csm = sqlc.narg(assigned_csm),
    backup_csm   = sqlc.narg(backup_csm),
    updated_by   = sqlc.narg(updated_by),
    updated_at   = now()
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: SoftDeleteAccount :exec
-- Soft-delete: baris disembunyikan dari list/get tapi tetap ada (jejak & FK dari
-- entitas lain — deal/tiket — tak putus). Idempotent: hanya baris hidup.
UPDATE accounts SET deleted_at = now(), updated_by = sqlc.narg(updated_by)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;
