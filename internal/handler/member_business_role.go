package handler

import (
	"context"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// member_business_role.go — sumbu CRM (business_role) di panel /members. Dipisah
// dari members.go agar MemberSetRole (yang kini menilai DUA sumbu dalam satu POST)
// tetap di bawah batas handler 150 baris, dan agar tiap sumbu dinilai guard-nya
// sendiri di satu tempat yang jelas.
//
// Gerbang penugasan = canManageMembers (sumbu TENANT), bukan CanBusiness: sumbu
// CRM fail-closed, jadi workspace baru tak punya admin CRM untuk membuka pintu ini
// — kuncinya harus di sumbu yang selalu ada. Kelayakan itu diperiksa di
// MemberSetRole sebelum kedua helper ini dipanggil.

// applyTenantRole menerapkan perubahan role SUMBU TENANT satu anggota. Mengembalikan
// kode PRG error ("" = sukses). Cermin logika MemberSetRole lama, dipindah utuh:
// ValidRoleName → loadActorTarget → GuardSetRole → jaga owner terakhir → simpan +
// audit + notify.
func (h *Handler) applyTenantRole(ctx context.Context, targetID, tenantID int64, newRole string) string {
	if !authz.ValidRoleName(newRole) {
		return "role"
	}
	actor, target, err := h.loadActorTarget(ctx, targetID, tenantID)
	if err != nil {
		return "notfound"
	}
	if err := authz.GuardSetRole(actor, target, authz.ParseRole(newRole)); err != nil {
		return "forbidden"
	}
	// Owner terakhir tak boleh diturunkan (workspace yatim). Ini pula anti-lockout
	// SEBENARNYA untuk sumbu CRM: owner tetak permanen selalu bisa menugaskan admin
	// CRM lagi, jadi guard B di sumbu CRM cukup jadi peringatan lunak.
	if target.Role == authz.RoleOwner && newRole != authz.RoleNameOwner {
		if n, e := h.q(ctx).CountTenantOwners(ctx, tenantID); e == nil && n <= 1 {
			return "lastowner"
		}
	}
	if err := h.q(ctx).UpdateMemberRole(ctx, db.UpdateMemberRoleParams{
		UserID: targetID, TenantID: tenantID, Role: newRole,
	}); err != nil {
		h.Log.Error("members: update role", "err", err)
		return "failed"
	}
	h.audit(ctx, actor.ID, "member.role.update", targetID, map[string]string{"to": newRole})
	// Beri tahu yang bersangkutan — perubahan role mengubah apa yang bisa ia lakukan.
	h.notify(ctx, targetID, tenantID, "member.role.changed", notifPayload{Role: newRole})
	return ""
}

// applyBusinessRole menerapkan perubahan SUMBU CRM (business_role) satu anggota.
// newRole "" = cabut peran (business_role → NULL). Mengembalikan:
//   - changed: nilai benar-benar berubah (no-op tak di-audit/notify),
//   - warnLastAdmin: baru saja mencabut Admin CRM terakhir (peringatan LUNAK, tetap
//     lanjut — bukan blokir; anti-lockout bersandar owner tenant permanen),
//   - errCode: kode PRG error ("" = sukses).
func (h *Handler) applyBusinessRole(ctx context.Context, targetID, tenantID int64, newRole string) (changed, warnLastAdmin bool, errCode string) {
	// Nilai saat ini: deteksi no-op + guard B "admin CRM terakhir".
	cur := ""
	if m, e := h.q(ctx).GetMembership(ctx, db.GetMembershipParams{UserID: targetID, TenantID: tenantID}); e == nil && m.BusinessRole != nil {
		cur = *m.BusinessRole
	}
	if newRole == cur {
		return false, false, ""
	}
	// Nilai non-kosong WAJIB ada di workspace ini (tenant-aware, pertahanan di atas
	// RLS): nama peran datang dari form (user-controlled).
	if newRole != "" {
		if _, e := h.q(ctx).GetBusinessRole(ctx, db.GetBusinessRoleParams{TenantID: tenantID, Name: newRole}); e != nil {
			return false, false, "crm_role"
		}
	}
	// Guard B LUNAK: mencabut admin CRM terakhir → tetap simpan, tapi peringatkan
	// bahwa matriks peran tak bisa disunting sampai ada admin CRM lagi.
	if cur == authz.BusinessRoleAdmin && newRole != authz.BusinessRoleAdmin {
		if h.countBusinessRoleHolders(ctx, tenantID, authz.BusinessRoleAdmin) <= 1 {
			warnLastAdmin = true
		}
	}
	// NULL (cabut) diteruskan pointer nil; selainnya pointer ke nilai.
	var val *string
	if newRole != "" {
		val = &newRole
	}
	if err := h.q(ctx).UpdateMemberBusinessRole(ctx, db.UpdateMemberBusinessRoleParams{
		UserID: targetID, TenantID: tenantID, BusinessRole: val,
	}); err != nil {
		h.Log.Error("members: update business role", "err", err)
		return false, false, "failed"
	}
	// RefreshIdentity membaca business_role per-request → menu CRM muncul/hilang
	// tanpa re-login. Enforcer TAK di-reload: matriks peran tak berubah, hanya
	// pemetaan orang→peran.
	h.audit(ctx, session.UserID(ctx), "member.business_role.changed", targetID, map[string]string{"to": newRole})
	h.notify(ctx, targetID, tenantID, "member.business_role.changed", notifPayload{Role: newRole})
	return true, warnLastAdmin, ""
}

// countBusinessRoleHolders menghitung anggota pemegang satu peran CRM lewat
// member_count ListBusinessRoles (dihitung DB, hindari query per-baris). 0 bila
// peran tak ada / query gagal — pemanggil (guard B) hanya perlu ambang "≤1".
func (h *Handler) countBusinessRoleHolders(ctx context.Context, tenantID int64, name string) int64 {
	roles, err := h.q(ctx).ListBusinessRoles(ctx, tenantID)
	if err != nil {
		return 0
	}
	for _, r := range roles {
		if r.Name == name {
			return r.MemberCount
		}
	}
	return 0
}

// membersMsg memetakan kode SUKSES PRG (?ok=CODE) khusus halaman /members ke
// kalimat. Dipisah dari wsErrMsg (yang untuk ?err=): sukses & galat dua kanal
// berbeda, dan crm_lastadmin adalah peringatan yang tampil sebagai alert sukses.
func membersMsg(code string) string {
	switch code {
	case "crm_assigned":
		return "Peran CRM anggota diperbarui."
	case "crm_lastadmin":
		return "Peran CRM diperbarui. Anda baru saja mencabut Admin CRM terakhir — " +
			"matriks peran (menu Peran) tak bisa disunting sampai ada anggota yang " +
			"ditugaskan sebagai Admin CRM lagi."
	default:
		return ""
	}
}
