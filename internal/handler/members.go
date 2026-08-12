package handler

import (
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// members.go — AKSI atas anggota workspace aktif: ubah role & keluarkan.
// Halaman daftarnya ada di members_page.go (aturan apa yang boleh DILIHAT tumbuh
// bersama kebijakan privasi, bukan bersama daftar aksi).

// MemberSetRole — POST /w/{workspace}/members/{id}/role. Satu POST, DUA sumbu:
// role TENANT (member/admin/owner) & peran CRM (business_role). Owner/admin saja
// (gerbang sumbu tenant). Tiap sumbu dinilai guard sendiri (applyTenantRole /
// applyBusinessRole) — kegagalan salah satu memantul dgn kode error masing-masing.
//
// Sumbu tenant DILEWATI untuk diri sendiri (baris sendiri tak memuat select role
// tenant — cegah menurunkan/mengunci diri, jaga owner terakhir). Sumbu CRM boleh
// menyentuh diri sendiri: itu opt-in owner ke CRM (penugasan business_role).
func (h *Handler) MemberSetRole(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageMembers(ctx) {
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

	// Sumbu TENANT — hanya bila form membawanya DAN target bukan diri sendiri.
	if targetID != selfID && r.PostForm.Has("role") {
		if code := h.applyTenantRole(ctx, targetID, tenantID, r.PostForm.Get("role")); code != "" {
			wsRedirect(w, r, "/members", code)
			return
		}
	}

	// Sumbu CRM — bila form membawanya (termasuk baris sendiri = opt-in).
	okCode := ""
	if r.PostForm.Has("business_role") {
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

// MemberRemove — POST /w/{workspace}/members/{id}/remove. Keluarkan anggota dari
// workspace aktif (membership dihapus; USER-nya tetap ada — identitas global).
func (h *Handler) MemberRemove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageMembers(ctx) {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
	targetID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	tenantID := session.TenantID(ctx)
	actor, target, err := h.loadActorTarget(ctx, targetID, tenantID)
	if err != nil {
		wsRedirect(w, r, "/members", "notfound")
		return
	}
	if err := authz.GuardDelete(actor, target); err != nil {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
	if target.Role == authz.RoleOwner {
		if n, e := h.q(ctx).CountTenantOwners(ctx, tenantID); e == nil && n <= 1 {
			wsRedirect(w, r, "/members", "lastowner")
			return
		}
	}
	if err := h.q(ctx).DeleteMembership(ctx, db.DeleteMembershipParams{
		UserID: targetID, TenantID: tenantID,
	}); err != nil {
		h.Log.Error("members: remove", "err", err)
		wsRedirect(w, r, "/members", "failed")
		return
	}
	h.audit(ctx, actor.ID, "member.remove", targetID, nil)
	// Notifikasi ditulis SETELAH membership dihapus — sengaja: baris notifikasi
	// hanya ber-FK ke tenants (bukan memberships), jadi tetap ada & terbaca oleh
	// mantan anggota yang sudah tak punya akses ke workspace itu.
	h.notify(ctx, targetID, tenantID, "member.removed", notifPayload{})
	wsRedirect(w, r, "/members", "")
}
