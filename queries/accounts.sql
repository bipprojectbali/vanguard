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
    district_id, village_address, postal_code, territory,
    village_status, village_classification, population, hamlets_count, village_budget,
    contact_phone, office_phone, office_email,
    created_by
) VALUES (
    sqlc.arg(tenant_id), sqlc.arg(entity_code), sqlc.arg(village_name),
    sqlc.narg(village_code), sqlc.arg(account_type),
    sqlc.narg(account_owner), sqlc.narg(assigned_csm), sqlc.narg(backup_csm),
    sqlc.narg(website), sqlc.narg(description),
    sqlc.narg(district_id),
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
--
-- search = pencarian teks (BL-6): '' → tak menyaring; selain itu ILIKE contains
-- pada village_name/village_code (case-insensitive). MENYEMPITKAN di atas ownership,
-- tak pernah memperluas — RLS+F3 tetap gerbang cakupan. Tetap keyset+LIMIT (bukan
-- full scan tak berbatas). Indeks trigram ditunda (lihat catatan handler).
SELECT * FROM accounts
WHERE deleted_at IS NULL
  AND (created_at, id) < (sqlc.arg(cursor_created_at)::timestamptz, sqlc.arg(cursor_id)::bigint)
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_sales)::boolean AND account_owner = sqlc.arg(uid))
      OR (sqlc.arg(is_csm)::boolean AND (assigned_csm = sqlc.arg(uid) OR backup_csm = sqlc.arg(uid)))
  )
  AND (NOT sqlc.arg(unowned)::boolean OR account_owner IS NULL)
  AND (
      sqlc.arg(search)::text = ''
      OR village_name ILIKE '%' || sqlc.arg(search) || '%'
      OR village_code ILIKE '%' || sqlc.arg(search) || '%'
  )
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
    district_id           = sqlc.narg(district_id),
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

-- name: FindDuplicateAccountsByNameRegion :many
-- Kandidat desa dgn nama sama (case-insensitive, trim) di tenant yang sama — dipakai
-- sbg soft-warning di halaman review konversi lead (M4-6, follow-up), BUKAN hard
-- block: nama desa yang sama bisa valid beda dusun/kabupaten. district_id opsional
-- (FK exact-match sejak 0009 — sebelumnya fuzzy teks pada regency): diisi → ikut
-- menyaring persis, kosong → cukup cocokkan nama. Ditopang index functional
-- idx_accounts_village_name_ci (00020). Dibatasi 5 kandidat, cukup utk peringatan,
-- bukan daftar lengkap.
SELECT id, entity_code, village_name, district_id
FROM accounts
WHERE tenant_id = sqlc.arg(tenant_id)
  AND deleted_at IS NULL
  AND lower(trim(village_name)) = lower(trim(sqlc.arg(village_name)))
  AND (
      sqlc.narg(district_id)::bigint IS NULL
      OR district_id = sqlc.narg(district_id)
  )
ORDER BY created_at DESC
LIMIT 5;

-- name: MaxVillageSeqForPrefix :one
-- Nomor urut (segmen ke-4) TERTINGGI yang sudah dipakai village_code otomatis
-- untuk satu tenant + prefix Kecamatan (mis. prefix "32.01.01."). Dihitung atas
-- SEMUA baris — termasuk yang ter-soft-delete — supaya nomor desa yang pernah ada
-- TAK PERNAH dipakai ulang (village_code adalah identitas yang orang kutip).
-- Guard regex '^[0-9]{1,9}$' membuat cast ::int aman terhadap village_code lama
-- warisan field manual yang mungkin tak berformat 4-segmen. COALESCE → 0 bila
-- belum ada, jadi pemanggil cukup +1.
SELECT COALESCE(MAX((split_part(village_code, '.', 4))::int), 0)::int AS max_seq
FROM accounts
WHERE tenant_id = sqlc.arg(tenant_id)
  AND village_code LIKE sqlc.arg(prefix)
  AND split_part(village_code, '.', 4) ~ '^[0-9]{1,9}$';

-- name: VillageCodeExists :one
-- Apakah village_code persis ini sudah dipakai di tenant (SEMUA baris, termasuk
-- soft-deleted). Dipakai generateVillageCode utk melewati nomor yang sudah
-- terpakai SEBELUM INSERT — INSERT gagal akan meracuni tx ber-tenant, jadi kode
-- bebas harus dipastikan lebih dulu lewat SELECT (yang tak membatalkan tx).
SELECT EXISTS(
    SELECT 1 FROM accounts
    WHERE tenant_id = sqlc.arg(tenant_id) AND village_code = sqlc.arg(village_code)
) AS exists;

-- name: AccountEntityCodeExists :one
-- Apakah entity_code (kode sistem, mis. "DESA-001") ini sudah dipakai di tenant
-- (SEMUA baris). Dipakai allocEntityCode agar jalur OTOMATIS melewati slot yang
-- sudah direbut override manual — pola cek-sebelum-INSERT yang sama dgn
-- VillageCodeExists (tx tak boleh dibatalkan lalu dicoba ulang).
SELECT EXISTS(
    SELECT 1 FROM accounts
    WHERE tenant_id = sqlc.arg(tenant_id) AND entity_code = sqlc.arg(entity_code)
) AS exists;

-- name: ListAccountsForSelect :many
-- Desa yang boleh DITULIS aktor (F3), untuk dropdown pemilih desa di form "Tambah
-- Kontak" global. Predikat ownership IDENTIK ListAccounts (scope_all/is_sales/
-- is_csm → fail-closed: ketiganya false = NOL baris), tapi TANPA keyset dan hanya
-- kolom untuk <option> (id + nama), urut nama agar dropdown terbaca. Tak
-- dipaginasi: dipakai untuk MEMILIH satu desa, bukan menelusuri — RLS sudah
-- mengurung ke satu workspace.
SELECT id, village_name FROM accounts
WHERE deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_sales)::boolean AND account_owner = sqlc.arg(uid))
      OR (sqlc.arg(is_csm)::boolean AND (assigned_csm = sqlc.arg(uid) OR backup_csm = sqlc.arg(uid)))
  )
ORDER BY village_name ASC;

-- name: CountAccountsForSelect :one
-- Jumlah desa yang boleh ditulis aktor (predikat sama ListAccountsForSelect).
-- Dipakai gerbang tombol "Tambah Kontak" di daftar kontak global: 0 desa →
-- sembunyikan tombol (tak ada induk yang bisa dipilih). Murah (indeks + RLS
-- satu workspace), tak memuat baris ke handler daftar.
SELECT count(*) FROM accounts
WHERE deleted_at IS NULL
  AND (
      sqlc.arg(scope_all)::boolean
      OR (sqlc.arg(is_sales)::boolean AND account_owner = sqlc.arg(uid))
      OR (sqlc.arg(is_csm)::boolean AND (assigned_csm = sqlc.arg(uid) OR backup_csm = sqlc.arg(uid)))
  );
