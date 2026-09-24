package handler

import (
	"net/http"

	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// roles_page.go — HALAMAN baca peran CRM: DAFTAR (tabel) peran. Dipisah dari
// roles.go (AKSI create/update/delete): halaman ini hanya menyusun data untuk
// view, aturannya soal apa yang DITAMPILKAN.
//
// DETAIL/EDIT satu peran (RoleEditPage + memberScopeView) dipisah ke
// role_edit_page.go agar file ini di bawah ambang tipe Route/Handler (150).
// renderRolesForbidden tetap di sini, dipakai KEDUA halaman.

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

// renderRolesForbidden menjawab anggota tanpa izin crm:roles yang membuka panel
// lewat URL langsung: 403 + penjelasan, BUKAN 404 (penerima terbukti anggota,
// pola renderMembersForbidden). Status ditulis SEBELUM body — WriteHeader setelah
// body tak berpengaruh, halaman penolakan ber-status 200 tampak seperti sukses.
func (h *Handler) renderRolesForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Peran CRM", "/roles", panel.RolesForbidden())
}
