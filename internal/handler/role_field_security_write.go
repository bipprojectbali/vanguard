package handler

import (
	"context"

	"go_starter/internal/db"
	"go_starter/internal/fls"
)

// role_field_security_write.go — penulisan replace-all kebijakan FLS satu
// tenant, dipisah dari role_field_security.go agar file itu di bawah ambang
// tipe Route/Handler (150). Rasional fitur lengkap ada di header
// role_field_security.go.

// writeFieldSecurity menulis kebijakan FLS SATU peran (replace-all tenant,
// peran lain dibekukan ke nilai efektif saat ini) — inti bersama
// RoleFieldSecurityUpdate (form terpisah, peran sistem+kustom) dan RoleUpdate
// (form gabungan, HANYA peran kustom, lihat roles.go). `q`
// diserahkan pemanggil: RoleUpdate membungkusnya dalam db.WithSavepoint agar
// tulisannya terisolasi-gagal dari tulis matriks tanpa transaksi terpisah
// (ADR 0012).
func (h *Handler) writeFieldSecurity(ctx context.Context, q *db.Queries, tenantID, actorID int64, name string, isSystem bool, view, edit bool) (int, error) {
	// Closed set: peran DITENTUKAN dari DB (pola jalur lama) — barang siapa yang
	// tak lagi ada tak ikut ditulis ulang.
	roles, err := q.ListBusinessRoles(ctx, tenantID)
	if err != nil {
		return 0, err
	}

	// Nilai efektif SAAT INI (sebelum tulis) — sumber "pembekuan" bagi peran
	// selain yang sedang disunting: existing (kalau tenant sudah dikonfigurasi)
	// atau default terkunci fls.CanViewPhone/CanEditPhone (kalau belum).
	existingRows, err := q.ListFieldSecurityPolicies(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	existing := make(map[string]fls.Policy, len(existingRows))
	for _, p := range existingRows {
		existing[p.BusinessRole] = fls.Policy{CanViewPhone: p.CanViewPhone, CanEditPhone: p.CanEditPhone}
	}

	if edit {
		view = true
	}
	// Coercion server-side thd level Contacts/Leads (F2) — TAK cukup mengandalkan
	// disabled di klien, form bisa dikirim langsung. Peran sistem (admin) lolos:
	// diwakili glob crm:* di enforcer, tak punya matriks untuk direaksikan.
	if !isSystem {
		perms, err := q.ListBusinessRolePermissionsByTenant(ctx, tenantID)
		if err != nil {
			return 0, err
		}
		cells := permsByRole(perms)[name]
		contactsLevel := cellLevel(cells["crm:contacts"])
		leadsLevel := cellLevel(cells["crm:leads"])
		if contactsLevel == "none" && leadsLevel == "none" {
			view = false
			edit = false
		}
		if contactsLevel != "write" && leadsLevel != "write" {
			edit = false
		}
	}

	policies := make(map[string]fls.Policy, len(roles))
	for _, r2 := range roles {
		if r2.Name == name {
			policies[r2.Name] = fls.Policy{CanViewPhone: view, CanEditPhone: edit}
			continue
		}
		if p, ok := existing[r2.Name]; ok {
			policies[r2.Name] = p
			continue
		}
		policies[r2.Name] = fls.Policy{
			CanViewPhone: fls.CanViewPhone(tenantID, r2.Name),
			CanEditPhone: fls.CanEditPhone(tenantID, r2.Name),
		}
	}

	if err := q.DeleteFieldSecurityPoliciesForTenant(ctx, tenantID); err != nil {
		return 0, err
	}
	for roleName, p := range policies {
		if err := q.UpsertFieldSecurityPolicy(ctx, db.UpsertFieldSecurityPolicyParams{
			TenantID:     tenantID,
			BusinessRole: roleName,
			CanViewPhone: p.CanViewPhone,
			CanEditPhone: p.CanEditPhone,
			CreatedBy:    &actorID,
		}); err != nil {
			return 0, err
		}
	}

	fls.ReloadTenant(tenantID, policies)
	return len(policies), nil
}

// cellLevel menurunkan Level ("none"/"read"/"write") dari satu sel matriks —
// sel bisa nil (peran belum diberi izin modul itu sama sekali). Sama persis
// dgn logika roleModuleRows (roles_card.go), diulang di sini karena
// masukannya *permCell tunggal, bukan peta byRole.
func cellLevel(c *permCell) string {
	if c == nil {
		return "none"
	}
	switch {
	case c.write:
		return "write"
	case c.read:
		return "read"
	default:
		return "none"
	}
}
