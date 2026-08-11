-- Query sumbu RBAC bisnis (F2/F3) yang bisa diedit per-workspace. Dua tabel:
-- business_roles (definisi peran + data_scope) & business_role_permissions
-- (matriks obj/act). Enforcer Casbin di-load dari permissions; data_scope dibaca
-- per-request untuk filter kepemilikan desa (F3).

-- name: ListAllBusinessRolePermissions :many
-- STARTUP: seluruh izin SEMUA tenant, untuk membangun enforcer bisnis global
-- (subject di-fold jadi `t<tenant_id>:<role>` di Go). WAJIB dipanggil di dalam
-- WithSuper — di tenant-tx, RLS menyembunyikan tenant lain → enforcer deny-all
-- senyap untuk mereka.
SELECT tenant_id, role, obj, act
FROM business_role_permissions
ORDER BY tenant_id, role;

-- name: ListBusinessRolePermissionsByTenant :many
-- Izin SATU workspace — dipakai reload per-tenant setelah matriks diubah, dan
-- untuk merender editor. Urut agar diff/tampilan stabil.
SELECT tenant_id, role, obj, act
FROM business_role_permissions
WHERE tenant_id = $1
ORDER BY role, obj, act;

-- name: ListBusinessRoles :many
-- Daftar peran satu workspace (panel /roles + assign ke member). is_system dulu
-- agar admin (peran terkunci) tampil di atas. member_count = jumlah anggota yang
-- memegang peran ini (kolom "Jumlah User" di wireframe 9.2) — LEFT JOIN agar peran
-- tanpa pemegang tetap muncul dengan 0. COALESCE ke bigint: sqlc emit int64, bukan
-- interface{}. GROUP BY br.id (PK) sah — kolom br.* bergantung fungsional padanya.
SELECT br.id, br.tenant_id, br.name, br.display_name, br.description,
       br.data_scope, br.is_system, br.created_at, br.updated_at,
       COALESCE(COUNT(m.user_id), 0)::bigint AS member_count
FROM business_roles br
LEFT JOIN memberships m
    ON m.tenant_id = br.tenant_id AND m.business_role = br.name
WHERE br.tenant_id = $1
GROUP BY br.id
ORDER BY br.is_system DESC, br.name;

-- name: GetBusinessRole :one
-- Satu peran (edit/validasi). tenant_id di predikat = pertahanan berlapis di atas
-- RLS: nama peran datang dari URL, jadi cocokkan eksplisit ke workspace aktif.
SELECT id, tenant_id, name, display_name, description, data_scope, is_system, created_at, updated_at
FROM business_roles
WHERE tenant_id = $1 AND name = $2;

-- name: GetBusinessRoleDataScope :one
-- Cakupan desa (F3) untuk satu business_role — dibaca per-request di
-- RefreshIdentity, disimpan di session. Dipisah dari GetBusinessRole agar jalur
-- panas ini tak menarik kolom yang tak dipakainya.
SELECT data_scope
FROM business_roles
WHERE tenant_id = $1 AND name = $2;

-- name: CreateBusinessRole :one
-- Buat peran baru (atau seed default). name = subject Casbin & nilai
-- memberships.business_role; display_name = label layar; description = keterangan
-- satu baris (kolom Deskripsi wireframe 9.2, boleh ''). is_system hanya true untuk
-- seed admin. created_by NULL untuk seed migrasi/boot.
INSERT INTO business_roles (tenant_id, name, display_name, description, data_scope, is_system, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
ON CONFLICT (tenant_id, name) DO NOTHING
RETURNING id, tenant_id, name, display_name, description, data_scope, is_system, created_at, updated_at;

-- name: UpdateBusinessRole :exec
-- Sunting label, deskripsi & cakupan peran. name (subject Casbin) TAK diubah di
-- sini — mengganti nama peran memutus assign yang sudah ada; kalau perlu, buat
-- peran baru. is_system tak bisa disunting (dijaga di handler, bukan di query).
UPDATE business_roles
SET display_name = $3, description = $4, data_scope = $5, updated_by = $6, updated_at = now()
WHERE tenant_id = $1 AND name = $2;

-- name: DeleteBusinessRole :exec
-- Hapus peran. Perizinannya ikut (FK CASCADE); anggota yang memegangnya di-
-- unassign TERPISAH (UnassignBusinessRole) SEBELUM ini di handler — FK ke
-- memberships sengaja tak ada agar penghapusan peran bukan penghapusan orang.
-- Peran is_system ditolak di handler.
DELETE FROM business_roles WHERE tenant_id = $1 AND name = $2;

-- name: InsertBusinessRolePermission :exec
-- Tambah satu baris matriks (role,obj,act). Idempoten via UNIQUE. Dipakai seed
-- dan saat matriks disunting (handler menghitung selisih, sisipkan yang baru).
INSERT INTO business_role_permissions (tenant_id, role, obj, act)
VALUES ($1, $2, $3, $4)
ON CONFLICT (tenant_id, role, obj, act) DO NOTHING;

-- name: DeleteBusinessRolePermissionsForRole :exec
-- Kosongkan matriks satu peran sebelum menulis ulang (pola replace-all: handler
-- hapus semua lalu sisipkan set baru dalam satu tx — lebih sederhana & bebas
-- selisih daripada diff per-baris). CASCADE peran tak menyentuh ini; ini untuk
-- peran yang TETAP ada tapi izinnya diganti.
DELETE FROM business_role_permissions WHERE tenant_id = $1 AND role = $2;
