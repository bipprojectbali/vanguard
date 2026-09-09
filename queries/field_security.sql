-- name: ListFieldSecurityPolicies :many
-- Kebijakan FLS phone satu workspace, untuk halaman Settings DAN reload cache
-- per-tenant. Sedikit barisnya (satu per business_role), tak dipaginasi. Urut per
-- role agar tampilan & muat stabil. Nol baris = SAH: tenant belum dikonfigurasi →
-- pemanggil jatuh ke default terkunci (internal/fls).
SELECT * FROM field_security_policies WHERE tenant_id = $1 ORDER BY business_role;

-- name: ListAllFieldSecurityPolicies :many
-- Muat-semua saat startup (dipanggil dalam db.WithSuper, seperti loadBusinessPerms):
-- RLS menyembunyikan tenant lain di dalam tx ber-scope, jadi konteks super wajib
-- untuk memindai semua tenant sekaligus. Loader mengelompokkan per tenant_id.
SELECT * FROM field_security_policies ORDER BY tenant_id, business_role;

-- name: UpsertFieldSecurityPolicy :exec
-- Simpan/ubah kebijakan satu peran. Dipakai replace-all: handler menulis satu baris
-- untuk SETIAP peran tenant (termasuk all-false) agar "terkonfigurasi" bisa dibedakan
-- dari "default" (= tanpa baris sama sekali). created_by diisi saat pertama; updated_*
-- tiap kali diubah. CHECK edit⇒view ditegakkan DB (handler juga meng-coerce).
INSERT INTO field_security_policies
    (tenant_id, business_role, can_view_phone, can_edit_phone, created_by, updated_by)
VALUES ($1, $2, $3, $4, $5, $5)
ON CONFLICT (tenant_id, business_role) DO UPDATE
SET can_view_phone = EXCLUDED.can_view_phone,
    can_edit_phone = EXCLUDED.can_edit_phone,
    updated_by     = EXCLUDED.updated_by,
    updated_at     = now();

-- name: DeleteFieldSecurityPoliciesForTenant :exec
-- Bersihkan seluruh kebijakan tenant sebelum replace-all upsert. Menyimpan matriks
-- sebagai transaksi ganti-total menjaga "peran yang dihapus dari form" tak tertinggal.
DELETE FROM field_security_policies WHERE tenant_id = $1;
