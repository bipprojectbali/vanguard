package handler

import (
	"context"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// member_kind_role.go — sumbu KETIGA (Kind, BL-170) + helper penghitung
// pemegang peran CRM + pemetaan kode sukses PRG /members. Dipisah dari
// member_business_role.go agar file itu di bawah ambang tipe Route/Handler
// (150) — rasional gerbang & guard dua sumbu lain ada di header file itu.

// applyMemberKind menerapkan perubahan Jenis Anggota (kind, BL-170) satu anggota
// — sumbu KETIGA, tegak lurus role tenant & business_role. Mengembalikan:
//   - changed: nilai benar-benar berubah (no-op tak di-audit/notify),
//   - resetRole: Peran CRM saat ini di-reset ke NULL karena tak lagi cocok kind
//     baru (dipakai membersMsg memilih kalimat sukses yang menyebut efek sampingnya),
//   - errCode: kode PRG error ("" = sukses).
//
// Peran CRM lama yang jadi tak valid untuk kind baru DIRESET OTOMATIS dalam
// submit yang sama (keputusan desain dikonfirmasi user) — tanpa ini anggota
// bisa memegang business_role dari Jenis Anggota yang bukan miliknya lagi,
// merusak asumsi cascading select (BL-170 §7) yang mengira keduanya selalu
// selaras.
func (h *Handler) applyMemberKind(ctx context.Context, targetID, tenantID int64, newKind string) (changed, resetRole bool, errCode string) {
	if !authz.ValidKind(newKind) {
		return false, false, "kind"
	}
	m, err := h.q(ctx).GetMembership(ctx, db.GetMembershipParams{UserID: targetID, TenantID: tenantID})
	if err != nil {
		return false, false, "notfound"
	}
	if newKind == m.Kind {
		return false, false, ""
	}
	if err := h.q(ctx).UpdateMembershipKind(ctx, db.UpdateMembershipKindParams{
		UserID: targetID, TenantID: tenantID, Kind: newKind,
	}); err != nil {
		h.Log.Error("members: update kind", "err", err)
		return false, false, "failed"
	}
	// Peran CRM saat ini (bila ada) mungkin tak lagi cocok kind baru — cek &
	// reset. GetBusinessRole tenant-aware; gagal (peran terhapus) diperlakukan
	// sama seperti tak cocok (reset), fail-soft ke keadaan aman.
	if m.BusinessRole != nil {
		br, e := h.q(ctx).GetBusinessRole(ctx, db.GetBusinessRoleParams{TenantID: tenantID, Name: *m.BusinessRole})
		if e != nil || br.Kind != newKind {
			if err := h.q(ctx).UpdateMemberBusinessRole(ctx, db.UpdateMemberBusinessRoleParams{
				UserID: targetID, TenantID: tenantID, BusinessRole: nil,
			}); err != nil {
				h.Log.Error("members: reset business role after kind change", "err", err)
			} else {
				resetRole = true
			}
		}
	}
	h.audit(ctx, session.UserID(ctx), "member.kind.changed", targetID, map[string]string{"to": newKind})
	h.notify(ctx, targetID, tenantID, "member.kind.changed", notifPayload{Role: newKind})
	return true, resetRole, ""
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
	case "kind_changed":
		return "Jenis Anggota diperbarui."
	case "kind_changed_reset":
		return "Jenis Anggota diperbarui. Peran CRM anggota ini dicabut karena tak " +
			"lagi cocok dengan Jenis Anggota barunya."
	case "kind_role_changed":
		return "Jenis Anggota & Peran CRM anggota diperbarui."
	case "invited":
		return "Undangan dibuat."
	case "invite_resent":
		return "Email ini sudah diundang sebelumnya — undangan diperbarui."
	default:
		return ""
	}
}
