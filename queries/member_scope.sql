-- name: GetMemberScopePolicy :one
-- Cakupan jenis anggota SATU peran — dipanggil tiap request oleh
-- actorKindScope (BL-171) untuk aktor yang masuk lewat sumbu bisnis (bukan
-- canManageMembers). Baris tak ada = pgx.ErrNoRows → pemanggil jatuh ke
-- default KEDUA jenis terbuka (beda dari FLS yang defaultnya tertutup).
SELECT * FROM member_scope_policies WHERE tenant_id = $1 AND business_role = $2;

-- name: UpsertMemberScopePolicy :exec
-- Simpan cakupan SATU peran (dipanggil dari RoleUpdate, bukan replace-all
-- tenant — beda dari field_security karena cakupan ini murni properti peran
-- yang sedang disunting, tak membekukan peran lain). created_by diisi saat
-- pertama; updated_* tiap kali diubah. CHECK minimal-satu ditegakkan DB
-- (handler juga meng-coerce sebelum sampai sini).
INSERT INTO member_scope_policies
    (tenant_id, business_role, can_view_internal, can_view_external, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $5)
ON CONFLICT (tenant_id, business_role) DO UPDATE
SET can_view_internal = EXCLUDED.can_view_internal,
    can_view_external = EXCLUDED.can_view_external,
    updated_by        = EXCLUDED.updated_by,
    updated_at        = now();

-- name: DeleteMemberScopePolicyForRole :exec
-- Hapus baris cakupan SATU peran — dipanggil saat level crm:members peran itu
-- kembali ke "none" (CHECK minimal-satu tak mengizinkan false/false, jadi
-- "nonaktif" direpresentasikan lewat ABSENnya baris, bukan dua kolom false).
DELETE FROM member_scope_policies WHERE tenant_id = $1 AND business_role = $2;
