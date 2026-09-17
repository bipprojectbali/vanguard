package handler

import (
	"context"

	"go_starter/internal/authz"
)

// workspace_field_security.go — gerbang sumbu F4 (Field-Level Security) HP &
// WhatsApp (BL-107, ADR 0012). Sejak BL-145 subtask 3, matriks ini hidup di
// halaman DETAIL peran (/roles/{name}, role_field_security.go +
// panel.FieldSecurityRoleView) — bukan lagi section tenant-wide di /roles —
// agar signal Datastar-nya bisa bereaksi thd level Contacts/Leads (F2) peran
// yang SAMA (keduanya wajib satu page load). Penyimpanannya
// (field_security_policies, per-tenant ber-RLS, di-cache di internal/fls) dan
// gerbangnya (crm:field_security) tetap TERPISAH dari matriks Casbin /roles:
// sumbu F4 & F2 tak dicampur di satu transaksi (fls.go:11-32).

// canManageFieldSecurity = gerbang section Field Security (sumbu BISNIS, objek
// "crm:field_security" aksi write). SATU sumber untuk render section DAN gate POST —
// keduanya menyebut objek/aksi sama agar section tak menawarkan pintu yang lalu ditolak
// (pola canManageRoles).
//
// crm:field_security dimiliki admin lewat glob crm:* (business_defaults). SENGAJA
// TIDAK masuk authz.CRMModules() → tak jadi kolom editor /roles, jadi tak bisa
// diberikan ke non-admin lewat panel (perlakuan sama crm:roles). Deny-default.
func canManageFieldSecurity(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:field_security", "write")
}
