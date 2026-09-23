package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/fls"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/go-chi/chi/v5"
)

// roles_page.go — HALAMAN baca peran CRM: DAFTAR (tabel) + DETAIL/EDIT satu peran.
// Dipisah dari roles.go (AKSI create/update/delete): halaman ini hanya menyusun
// data untuk view, aturannya soal apa yang DITAMPILKAN.

// RolesPage — GET /w/{workspace}/roles. TABEL peran CRM workspace. Tiap baris
// menuju halaman detail (RoleEditPage) atau dihapus langsung.
//
// Gerbang di HANDLER (canManageRoles), BUKAN route: sejak 0004 satu alamat
// melayani semua role, yang membedakan adalah izin di dalamnya. Menu yang
// disembunyikan bukan pengaman — URL-nya tetap bisa diketik. Ditolak → 403 +
// penjelasan, BUKAN 404 (penerima terbukti anggota workspace; Scope memvalidasi).
func (h *Handler) RolesPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageRoles(ctx) {
		h.renderRolesForbidden(w, r)
		return
	}
	tenantID := session.TenantID(ctx)
	roles, err := h.q(ctx).ListBusinessRoles(ctx, tenantID)
	if err != nil {
		h.Log.Error("roles: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	// Daftar (arsip/read-only) tetap tampil; form tambah & aksi hapus disembunyikan.
	canEdit := !IsReadOnly(ctx)

	// Blok B (Permission Sets, BL-145 subtask 6): matriks presentasional peran
	// yang disorot lewat ?role= (default roles[0] — admin selalu ter-seed, jadi
	// roles kosong bukan jalur nyata). Butuh matriks izin tenant yang sama dgn
	// RoleEditPage (permsByRole), belum di-fetch di halaman ini sebelum ini.
	base := wsPath(slugFromRequest(r), "")
	var psv *panel.PermissionSetView
	if len(roles) > 0 {
		perms, err := h.q(ctx).ListBusinessRolePermissionsByTenant(ctx, tenantID)
		if err != nil {
			h.Log.Error("roles: list perms", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		v := buildPermissionSetView(base, roles, permsByRole(perms), r.URL.Query().Get("role"))
		psv = &v
	}

	h.renderWorkspaceShell(w, r, "Peran CRM", "/roles",
		panel.Roles(
			base,
			roleRows(roles),
			businessScopeOptions(),
			canEdit,
			wsErrMsg(r.URL.Query().Get("err")),
			rolesMsg(r.URL.Query().Get("ok")),
			psv,
		))
}

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
		))
}

// renderRolesForbidden menjawab anggota tanpa izin crm:roles yang membuka panel
// lewat URL langsung: 403 + penjelasan, BUKAN 404 (penerima terbukti anggota,
// pola renderMembersForbidden). Status ditulis SEBELUM body — WriteHeader setelah
// body tak berpengaruh, halaman penolakan ber-status 200 tampak seperti sukses.
func (h *Handler) renderRolesForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Peran CRM", "/roles", panel.RolesForbidden())
}
