package handler

import (
	"context"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/fls"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// workspace_field_security.go — sumbu F4 (Field-Level Security) HP & WhatsApp: gerbang
// + data section "Field Security" (BL-107, ADR 0012). Sejak opsi B, matriks ini DIRENDER
// sebagai section di dalam halaman Peran CRM (/roles), BUKAN halaman Settings tersendiri
// — satu pintu pengaturan per-peran. Namun POST-nya (/field-security),
// penyimpanannya (field_security_policies, per-tenant ber-RLS, di-cache di internal/fls),
// dan gerbangnya (crm:field_security) tetap TERPISAH dari matriks Casbin /roles: sumbu F4
// & F2 tak dicampur di satu transaksi (fls.go:11-32).
//
// Absennya baris untuk sebuah tenant = "belum dikonfigurasi" = default terkunci
// (Sales+Admin lihat, Sales sunting). Karena itu state awal kotak centang dibaca dari
// fls.CanViewPhone/CanEditPhone — WYSIWYG dengan yang DITEGAKKAN: tenant baru melihat
// default tercentang, bukan matriks kosong yang menyesatkan.

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

// fieldSecurityViewFor menyiapkan data section Field Security untuk ditanam di halaman
// /roles. Mengembalikan nil bila peninjau tak berwenang (crm:field_security) → view
// tak merender section sama sekali. Peran & state kotak centang dibaca dari cache yang
// SAMA dengan enforcement (fls) → apa yang ditampilkan persis apa yang ditegakkan.
func (h *Handler) fieldSecurityViewFor(r *http.Request) (*panel.FieldSecurityView, error) {
	ctx := r.Context()
	if !canManageFieldSecurity(ctx) {
		return nil, nil
	}
	tenantID := session.TenantID(ctx)
	roles, err := h.q(ctx).ListBusinessRoles(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	rows := make([]panel.FieldSecurityRoleRow, 0, len(roles))
	for _, role := range roles {
		rows = append(rows, panel.FieldSecurityRoleRow{
			Name:         role.Name,
			DisplayName:  role.DisplayName,
			IsSystem:     role.IsSystem,
			CanViewPhone: fls.CanViewPhone(tenantID, role.Name),
			CanEditPhone: fls.CanEditPhone(tenantID, role.Name),
		})
	}
	// Arsip/read-only → matriks tampil, tombol simpan disembunyikan (pola /roles).
	return &panel.FieldSecurityView{
		Base:    wsPath(slugFromRequest(r), ""),
		CanEdit: !IsReadOnly(ctx),
		Roles:   rows,
	}, nil
}
