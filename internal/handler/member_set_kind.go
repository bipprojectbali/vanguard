package handler

import (
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// member_set_kind.go — dipisah dari members.go agar tiap file di bawah ambang
// tipe Route/Handler (150). Hanya MemberSetKind; MemberSetRole tetap di
// members.go, MemberRemove di member_remove.go.

// MemberSetKind — POST /w/{workspace}/members/{id}/kind. Satu POST, DUA sumbu:
// Jenis Anggota (BL-170, kind) & peran CRM (business_role) — digabung SATU
// form/tombol Simpan di baris anggota (memberKindRoleForm) agar ubah kind yang
// otomatis menukar cascading select Peran CRM tersimpan sekaligus, bukan dua
// klik terpisah. business_role OPSIONAL (Has-gated) demi kompatibilitas mundur
// dgn pemanggil yang cuma mengirim kind. Boleh menyentuh diri sendiri (bukan
// sumbu lockout seperti role tenant).
//
// Gerbang MELEBAR (BL-171): pengelola tenant ATAU aktor business-axis
// ber-"Kelola" crm:members — endpoint TETAP TERBUKA bagi keduanya meski
// aktor business-axis bercakupan SATU jenis saja, sebab business_role tetap
// harus bisa disunting aktor semacam itu (memberKindRoleForm merender select
// Peran CRM terlepas dari canEditKind, lihat members_roles.go). Yang DIBATASI
// khusus adalah REASSIGNMENT kind: target harus ∈ cakupan aktor (tak bisa
// menyentuh anggota di luar cakupan sama sekali), dan kind BARU yang beda
// dari kind SAAT INI target hanya diterima dari aktor bercakupan KEDUA jenis
// (keputusan desain #3 plan BL-171) — UI aktor cakupan-sempit selalu mengirim
// kind tak berubah (hidden input, members_roles.go), jadi cek ini defense-in-
// depth thd request yang dipalsukan, bukan penghalang alur normal. Baris
// SENDIRI: aktor business-axis MURNI (bukan pengelola tenant) hanya boleh
// menyunting Jenis Anggota/Peran CRM dirinya sendiri lewat endpoint ini bila
// SUDAH admin CRM (BL-171, cermin guard sama di MemberSetRole) — pengelola
// tenant (owner/admin/platform) tak kena syarat ini. UI sudah menyembunyikan
// form ini bagi non-admin di baris dirinya sendiri, ini dicek ulang thd
// request yang dipalsukan.
func (h *Handler) MemberSetKind(w http.ResponseWriter, r *http.Request) {
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
	scope := actorKindScope(ctx, q)
	target, err := q.GetMembership(ctx, db.GetMembershipParams{UserID: targetID, TenantID: tenantID})
	if err != nil {
		wsRedirect(w, r, "/members", "notfound")
		return
	}
	if !scope.Allows(target.Kind) {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
	if targetID == selfID && !canManageMembers(ctx) &&
		(target.BusinessRole == nil || *target.BusinessRole != authz.BusinessRoleAdmin) {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
	newKind := r.PostForm.Get("kind")
	if newKind != target.Kind && !scope.Both() {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}

	kindChanged, resetRole, code := h.applyMemberKind(ctx, targetID, tenantID, newKind)
	if code != "" {
		wsRedirect(w, r, "/members", code)
		return
	}

	roleChanged, warnLastAdmin := false, false
	if r.PostForm.Has("business_role") {
		var roleCode string
		roleChanged, warnLastAdmin, roleCode = h.applyBusinessRole(ctx, targetID, tenantID, r.PostForm.Get("business_role"))
		if roleCode != "" {
			wsRedirect(w, r, "/members", roleCode)
			return
		}
	}

	okCode := ""
	switch {
	case warnLastAdmin:
		okCode = "crm_lastadmin"
	case kindChanged && roleChanged:
		okCode = "kind_role_changed"
	case kindChanged && resetRole:
		okCode = "kind_changed_reset"
	case kindChanged:
		okCode = "kind_changed"
	case roleChanged:
		okCode = "crm_assigned"
	}
	wsRedirectOK(w, r, "/members", okCode)
}
