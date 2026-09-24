package handler

import (
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// member_remove.go — dipisah dari members.go agar tiap file di bawah ambang
// tipe Route/Handler (150). Hanya MemberRemove; MemberSetRole tetap di
// members.go, MemberSetKind di member_set_kind.go.

// MemberRemove — POST /w/{workspace}/members/{id}/remove. Keluarkan anggota dari
// workspace aktif (membership dihapus; USER-nya tetap ada — identitas global).
//
// Gerbang LUAR MELEBAR (BL-171): pengelola tenant ATAU aktor business-axis
// ber-"Kelola" crm:members, dibatasi target ∈ actorKindScope. authz.GuardDelete
// TETAP TAK DIUBAH (lapisan kedua, sumbu ROLE TENANT — bukan sumbu CRM): ia
// menolak penghapusan bila role tenant target TAK LEBIH RENDAH dari aktor
// (`target.Role >= effectiveRole(actor)`). Bagi aktor business-axis murni (role
// tenant "member", tanpa promosi tenant apa pun), ini berarti GuardDelete akan
// SELALU menolak penghapusan anggota ber-role tenant "member" lain (setara,
// bukan lebih rendah) — sengaja: BL-171 melebarkan akses LIHAT/KELOLA data CRM,
// bukan hierarki wewenang tenant, dan plan secara eksplisit mempertahankan
// GuardDelete sebagai penjaga sesungguhnya di sini.
func (h *Handler) MemberRemove(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageMembers(ctx) && crmMemberAccess(ctx) != memberAccessManage {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
	targetID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	tenantID := session.TenantID(ctx)
	q := h.q(ctx)
	scope := actorKindScope(ctx, q)
	if m, e := q.GetMembership(ctx, db.GetMembershipParams{UserID: targetID, TenantID: tenantID}); e != nil || !scope.Allows(m.Kind) {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
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
