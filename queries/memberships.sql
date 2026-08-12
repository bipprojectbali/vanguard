-- name: CreateMembership :one
-- Jadikan user anggota workspace dgn role tertentu. Dipakai: register/OAuth (owner
-- workspace pertama), buat workspace baru (owner), terima invite (admin/member).
INSERT INTO memberships (user_id, tenant_id, role)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, tenant_id) DO NOTHING
RETURNING *;

-- name: GetMembership :one
-- Validasi keanggotaan — dipakai middleware Scope SEBELUM membuka tx ber-tenant
-- (memastikan tenant aktif di session memang milik user; anti tenant-forcing).
SELECT * FROM memberships WHERE user_id = $1 AND tenant_id = $2;

-- name: ListMembershipsByUser :many
-- Daftar workspace milik user (untuk switcher sidebar). Urut terlama dulu agar
-- workspace pertama (dari register) jadi default stabil.
--
-- Workspace TERHAPUS disembunyikan (0005): ia juga jadi sumber pilihan fallback
-- middleware Scope, jadi tanpa filter ini user bisa dilempar ke workspace yang
-- sudah dihapus. status ikut dikembalikan agar switcher bisa menandai yang
-- suspended/archived alih-alih membiarkan user menabrak 403 setelah mengklik.
-- is_primary ikut dikembalikan karena sidebar menghitung sisa kuota dari daftar
-- ini; tanpanya hitungannya akan BERBEDA dari CountOwnedWorkspaces, dan user
-- melihat tombol "Buat workspace" yang lalu ditolak (atau sebaliknya).
SELECT m.tenant_id, m.role, t.name, t.slug, t.status, t.is_primary
FROM memberships m
JOIN tenants t ON t.id = m.tenant_id
WHERE m.user_id = $1 AND t.deleted_at IS NULL
ORDER BY m.created_at, m.id;

-- name: ListMembersByTenant :many
-- Daftar anggota SATU workspace (panel /admin/members). JOIN users untuk data
-- tampilan — users kini tabel global (tanpa RLS), jadi filter tenant di sini.
--
-- `u.name` ikut diambil dan MENDAHULUI email sebagai penanda orang di layar.
-- Emailnya tetap dibawa karena pengelola membutuhkannya (mengundang,
-- mencocokkan orang) — yang menahannya dari mata lain adalah handler, yang
-- menyamarkannya sebelum data menyentuh view.
--
-- m.business_role (sumbu CRM, tegak lurus role tenant) ikut agar kolom "Peran
-- CRM" di /members bisa memilih nilai saat ini tanpa query per-baris (Rule 13);
-- NULL = belum diberi peran CRM.
SELECT m.id, m.user_id, m.role, m.business_role, m.created_at, u.email, u.name, u.avatar_url, u.status
FROM memberships m
JOIN users u ON u.id = m.user_id
WHERE m.tenant_id = $1 AND u.deleted_at IS NULL
ORDER BY m.created_at, m.id;

-- name: UpdateMemberRole :exec
UPDATE memberships SET role = $3 WHERE user_id = $1 AND tenant_id = $2;

-- name: UpdateMemberBusinessRole :exec
-- Set/ganti business_role (sumbu CRM F2) satu anggota. Nilai divalidasi tenant-
-- aware di handler (ada di business_roles workspace ini) SEBELUM query — kolom
-- tak lagi punya CHECK sejak 00007. NULL = cabut peran CRM (mis. saat perannya
-- dihapus) — pgtype/pointer NULL diteruskan apa adanya.
UPDATE memberships SET business_role = $3 WHERE user_id = $1 AND tenant_id = $2;

-- name: UnassignBusinessRole :exec
-- Cabut satu business_role dari SEMUA anggota workspace yang memegangnya —
-- dipanggil SEBELUM menghapus perannya, agar penghapusan peran meng-unassign
-- orang alih-alih (lewat FK) menghapus keanggotaannya. Idempoten.
UPDATE memberships SET business_role = NULL
WHERE tenant_id = $1 AND business_role = $2;

-- name: DeleteMembership :exec
-- Keluarkan anggota dari workspace (atau user keluar sendiri).
DELETE FROM memberships WHERE user_id = $1 AND tenant_id = $2;

-- name: ListMembershipsForUsers :many
-- Membership BANYAK user sekaligus (panel /dev: tampilkan workspace+role tiap
-- user). Satu query untuk semua id → hindari N+1 (Rule 13).
SELECT m.user_id, m.tenant_id, m.role, t.name, t.slug
FROM memberships m
JOIN tenants t ON t.id = m.tenant_id
WHERE m.user_id = ANY(@user_ids::bigint[]) AND t.deleted_at IS NULL
ORDER BY m.user_id, m.created_at, m.id;

-- name: CountOwnedWorkspaces :one
-- Berapa workspace yang DIMILIKI user (role owner) — untuk cek kuota sebelum
-- membuat workspace baru. Diundang jadi member/admin TIDAK memakan kuota.
--
-- Workspace TERHAPUS tak dihitung (0005 §7): kuota yang masih tertahan oleh
-- workspace yang sudah dihapus terasa seperti bug, dan mendorong user memurge
-- lebih cepat — kebalikan dari tujuan masa tenggang. Yang TERARSIP TETAP
-- dihitung: datanya masih disimpan & bisa diaktifkan kapan saja.
--
-- Workspace PRIMER juga tak dihitung (0007): kuota membatasi berapa banyak yang
-- boleh DIBUAT, dan rumah aplikasi tak dibuat siapa pun — ia sudah ada sebelum
-- user pertama mendaftar. Tanpa pengecualian ini, super_admin yang jadi ownernya
-- memakai jatahnya sendiri, dan dengan default 1 orang yang MENETAPKAN aturan
-- kuota justru tak bisa membuat workspace apa pun.
SELECT count(*)::bigint
FROM memberships m
JOIN tenants t ON t.id = m.tenant_id
WHERE m.user_id = $1 AND m.role = 'owner' AND t.deleted_at IS NULL AND NOT t.is_primary;

-- name: CountTenantOwners :one
-- Jumlah owner di satu workspace — cegah menghapus/menurunkan owner terakhir
-- (workspace tanpa owner = yatim).
SELECT count(*)::bigint FROM memberships WHERE tenant_id = $1 AND role = 'owner';

-- name: ListMembersByBusinessRole :many
-- user_id anggota workspace dgn business_role tertentu — dipakai menarget
-- notifikasi (mis. semua Manager saat renewal Upsell menunggu persetujuan).
-- memberships SENGAJA tanpa RLS (dibaca untuk MENENTUKAN scope), jadi filter
-- tenant_id EKSPLISIT. Cast ::text pada param → sqlc emit `string` (non-null);
-- baris ber-business_role NULL tak pernah cocok, itu benar (belum berperan CRM).
SELECT user_id FROM memberships
WHERE tenant_id = sqlc.arg(tenant_id) AND business_role = sqlc.arg(business_role)::text;
