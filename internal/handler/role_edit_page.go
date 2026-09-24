package handler

import (
	"context"
	"errors"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/fls"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// role_edit_page.go — HALAMAN DETAIL/EDIT satu peran CRM (editor matriks izin +
// cakupan data F3 + FLS + cakupan Anggota). Dipisah dari roles_page.go (DAFTAR/
// tabel peran) agar file itu di bawah ambang tipe Route/Handler (150).
// renderRolesForbidden (dipakai kedua halaman) tetap di roles_page.go.

// RoleEditPage — GET /w/{workspace}/roles/{name}. DETAIL satu peran: editor
// matriks izin (obj × aksi) + cakupan data F3. Peran tak ada → kembali ke daftar
// dengan role_notfound (bukan 404 telanjang: penerima terbukti admin CRM, cukup
// diberi tahu perannya tak ditemukan). Peran sistem tampil terkunci.
//
// Gerbang sama dengan RolesPage (canManageRoles) — URL detail pun bisa diketik
// langsung, jadi izinnya ditegakkan di sini, bukan diandalkan dari daftar.
func (h *Handler) RoleEditPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageRoles(ctx) {
		h.renderRolesForbidden(w, r)
		return
	}
	tenantID := session.TenantID(ctx)
	name := chi.URLParam(r, "name")
	role, err := h.q(ctx).GetBusinessRole(ctx, db.GetBusinessRoleParams{TenantID: tenantID, Name: name})
	if err != nil {
		wsRedirect(w, r, "/roles", "role_notfound")
		return
	}
	perms, err := h.q(ctx).ListBusinessRolePermissionsByTenant(ctx, tenantID)
	if err != nil {
		h.Log.Error("roles: list perms", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Editor tak boleh menulis (arsip/read-only) → matriks tampil, tombol simpan
	// disembunyikan. Dihitung sekali; canManageRoles membaca session tiap panggil.
	canEdit := !IsReadOnly(ctx)
	card := buildRoleCard(role.Name, role.DisplayName, role.Description, role.DataScope, role.Kind, role.IsSystem, permsByRole(perms))

	// fsec nil → peninjau tak berwenang crm:field_security: section Field
	// Security tak dirender sama sekali (RoleEdit, ADR 0012 F4 tetap gerbang
	// terpisah dari canManageRoles F2). Reactive=false utk peran sistem (admin
	// diwakili glob crm:*, tak ada matriks Contacts/Leads utk direaksikan).
	base := wsPath(slugFromRequest(r), "")
	var fsec *panel.FieldSecurityRoleView
	if canManageFieldSecurity(ctx) {
		fsec = &panel.FieldSecurityRoleView{
			Base:         base,
			Name:         role.Name,
			CanEdit:      canEdit,
			CanViewPhone: fls.CanViewPhone(tenantID, role.Name),
			CanEditPhone: fls.CanEditPhone(tenantID, role.Name),
			Reactive:     !role.IsSystem,
		}
	}

	h.renderWorkspaceShell(w, r, "Peran CRM", "/roles",
		panel.RoleEdit(
			base,
			card,
			businessScopeOptions(),
			canEdit,
			wsErrMsg(r.URL.Query().Get("err")),
			rolesMsg(r.URL.Query().Get("ok")),
			fsec,
			memberScopeView(ctx, h, tenantID, role.Name),
		))
}

// memberScopeView (BL-171) membaca cakupan "Anggota" tersimpan SATU peran —
// dipanggil tanpa syarat (termasuk peran sistem, walau RoleEdit lalu
// mengabaikannya di cabang itu: tak ada matriks Contacts/Leads-nya utk
// direaksikan bila IsSystem, sama alasan fsec.Reactive). Baris tak ada
// (pgx.ErrNoRows, role baru saja diberi akses crm:members, atau memang belum
// pernah dikonfigurasi) → default KEDUA jenis terbuka (beda dari FLS —
// komentar migrasi 00052). Error lain (koneksi dsb.) → fail-closed ke default
// yang SAMA (bukan {false,false}, krn CHECK DB menolak itu & sengaja opt-in
// terbuka): baris terlindungi RLS/CHECK di tulis, baca gagal tak boleh
// mengunci admin dari peran yang baru dibuatnya.
func memberScopeView(ctx context.Context, h *Handler, tenantID int64, roleName string) panel.MemberScopeRoleView {
	pol, err := h.q(ctx).GetMemberScopePolicy(ctx, db.GetMemberScopePolicyParams{
		TenantID: tenantID, BusinessRole: roleName,
	})
	if err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			h.Log.Error("roles: get member scope", "err", err)
		}
		return panel.MemberScopeRoleView{CanViewInternal: true, CanViewExternal: true}
	}
	return panel.MemberScopeRoleView{CanViewInternal: pol.CanViewInternal, CanViewExternal: pol.CanViewExternal}
}
