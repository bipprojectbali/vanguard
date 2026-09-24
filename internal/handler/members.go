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
