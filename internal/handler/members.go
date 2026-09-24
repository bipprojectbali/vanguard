package handler

import (
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// members.go — AKSI atas anggota workspace aktif: ubah role. Halaman
// daftarnya ada di members_page.go (aturan apa yang boleh DILIHAT tumbuh
// bersama kebijakan privasi, bukan bersama daftar aksi). MemberSetKind &
// MemberRemove dipisah ke member_set_kind.go & member_remove.go agar tiap
// file di bawah ambang tipe Route/Handler (150).

// MemberSetRole — POST /w/{workspace}/members/{id}/role. Satu POST, DUA sumbu:
// role TENANT (member/admin/owner) & peran CRM (business_role). Tiap sumbu
// dinilai guard sendiri (applyTenantRole / applyBusinessRole) — kegagalan
// salah satu memantul dgn kode error masing-masing.
//
// Gerbang LUAR (fungsi ini) MELEBAR (BL-171): pengelola tenant ATAU aktor
// business-axis ber-"Kelola" crm:members. Sumbu TENANT sendiri TETAP terkunci
// canManageMembers MURNI, dicek ULANG di dalam blok itu (keputusan desain #1
// plan BL-171) — aktor business-axis tak pernah bisa mengeskalasi otoritas
// tenant lewat izin CRM yang melebar, walau lolos gerbang luar. Sumbu CRM
// dibatasi actorKindScope: target di luar cakupan Jenis Anggota aktor ditolak
// (defense-in-depth — UI sudah membatasi pilihan, ini menjaga request yang
// dipalsukan).
//
// Sumbu tenant DILEWATI untuk diri sendiri (baris sendiri tak memuat select role
// tenant — cegah menurunkan/mengunci diri, jaga owner terakhir). Sumbu CRM: bagi
// aktor business-axis MURNI (bukan pengelola tenant), menyentuh diri sendiri
// HANYA diterima bila peran CRM aktor SAAT INI "admin" (BL-171 — dicek ulang di
// sini thd request yang dipalsukan, UI (memberRow, role_edit package) sudah
// menyembunyikan form ini bagi non-admin di baris dirinya sendiri). Pengelola
// tenant (owner/admin/platform) TETAP boleh opt-in diri sendiri ke peran CRM
// tanpa syarat ini (TestMemberBiz_SelfOptIn) — otoritas mereka datang dari sumbu
// tenant yang sudah lengkap, bukan dari izin CRM yang sama dipakai mengelola
// anggota lain.
func (h *Handler) MemberSetRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageMembers(ctx) && crmMemberAccess(ctx) != memberAccessManage {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
	targetID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		wsRedirect(w, r, "/members", "failed")
		return
	}
	tenantID := session.TenantID(ctx)
	selfID := session.UserID(ctx)
	q := h.q(ctx)

	// Sumbu TENANT — hanya bila form membawanya DAN target bukan diri sendiri,
	// TETAP terkunci canManageMembers murni (tak melebar).
	if targetID != selfID && r.PostForm.Has("role") {
		if !canManageMembers(ctx) {
			wsRedirect(w, r, "/members", "forbidden")
			return
		}
		if code := h.applyTenantRole(ctx, targetID, tenantID, r.PostForm.Get("role")); code != "" {
			wsRedirect(w, r, "/members", code)
			return
		}
	}

	// Sumbu CRM — bila form membawanya (termasuk baris sendiri = opt-in).
	okCode := ""
	if r.PostForm.Has("business_role") {
		scope := actorKindScope(ctx, q)
		m, e := q.GetMembership(ctx, db.GetMembershipParams{UserID: targetID, TenantID: tenantID})
		if e != nil || !scope.Allows(m.Kind) {
			wsRedirect(w, r, "/members", "forbidden")
			return
		}
		// Baris sendiri: aktor business-axis MURNI (bukan pengelola tenant) hanya
		// boleh menyunting perannya sendiri bila SUDAH admin CRM (BL-171) — pengelola
		// tenant (owner/admin/platform) tetap boleh opt-in diri sendiri seperti
		// sebelumnya (TestMemberBiz_SelfOptIn), sebab otoritasnya datang dari sumbu
		// tenant yang sudah lengkap, bukan dari akses CRM yang sama dipakai
		// mengelola anggota lain.
		if targetID == selfID && !canManageMembers(ctx) &&
			(m.BusinessRole == nil || *m.BusinessRole != authz.BusinessRoleAdmin) {
			wsRedirect(w, r, "/members", "forbidden")
			return
		}
		changed, warnLastAdmin, code := h.applyBusinessRole(ctx, targetID, tenantID, r.PostForm.Get("business_role"))
		if code != "" {
			wsRedirect(w, r, "/members", code)
			return
		}
		switch {
		case warnLastAdmin:
			okCode = "crm_lastadmin"
		case changed:
			okCode = "crm_assigned"
		}
	}
	wsRedirectOK(w, r, "/members", okCode)
}
