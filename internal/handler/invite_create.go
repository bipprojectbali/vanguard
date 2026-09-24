package handler

import (
	"net/http"
	"strings"
	"time"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/oauth"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5/pgtype"
)

// invite_create.go — dipisah dari invite.go agar tiap file di bawah batas 150
// baris. Isinya InviteCreate (aksi pengelola) + const inviteTTL (hanya
// dipakainya). InviteDelete/InvitePage/InviteAccept tetap di invite.go.

// inviteTTL = masa berlaku undangan. Cukup lama untuk dibagikan, cukup pendek
// agar token bocor tak berlaku selamanya.
const inviteTTL = 7 * 24 * time.Hour

// InviteCreate — POST /w/{workspace}/members/invite. Buat undangan. BL-170: role
// tenant SELALU "member" (promosi admin terjadi PASCA-join lewat panel Anggota,
// bukan lagi dipilih di form Undang) — form kini menentukan Peran CRM
// (business_role, opsional) + Jenis Anggota (kind, wajib), sepasang sumbu yang
// diterapkan otomatis saat undangan diterima (acceptInvite/acceptInvitesByEmail).
//
// Gerbang MELEBAR (BL-171): pengelola tenant ATAU aktor business-axis
// ber-"Kelola" crm:members. Role tenant sudah SELALU "member" apa pun aktornya
// (tak perlu dipaksa terpisah — tak pernah ada input form untuk itu). kind
// dibatasi actorKindScope: aktor bercakupan sempit tak bisa mengundang di luar
// jenis yang boleh ia lihat sendiri (defense-in-depth — UI sudah menguncinya
// via hidden input saat !canEditKind, lihat inviteForm).
func (h *Handler) InviteCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageMembers(ctx) && crmMemberAccess(ctx) != memberAccessManage {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
	tenantID := session.TenantID(ctx)
	q := h.q(ctx)
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	if email == "" || !strings.Contains(email, "@") {
		wsRedirect(w, r, "/members", "email")
		return
	}
	kind := r.FormValue("kind")
	if !authz.ValidKind(kind) {
		wsRedirect(w, r, "/members", "kind")
		return
	}
	if scope := actorKindScope(ctx, q); !scope.Allows(kind) {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
	businessRole := strings.TrimSpace(r.FormValue("business_role"))
	var businessRolePtr *string
	if businessRole != "" {
		// Tenant-aware DAN cocok Kind yang dipilih — pertahanan berlapis di atas
		// cascading select (yang bisa dilewati manipulasi form langsung).
		br, e := q.GetBusinessRole(ctx, db.GetBusinessRoleParams{TenantID: tenantID, Name: businessRole})
		if e != nil || br.Kind != kind {
			wsRedirect(w, r, "/members", "crm_role")
			return
		}
		businessRolePtr = &businessRole
	}
	// Guard (b): email yang sudah jadi anggota tak boleh diundang ulang — bukan
	// error DB (invites tak punya UNIQUE lintas-member), jadi dicek eksplisit.
	if exists, e := q.MemberExistsByEmail(ctx, db.MemberExistsByEmailParams{TenantID: tenantID, Lower: email}); e == nil && exists {
		wsRedirect(w, r, "/members", "invite_member")
		return
	}
	token, err := oauth.NewState() // 32-byte crypto/rand hex (dipakai ulang, nol dep)
	if err != nil {
		h.Log.Error("invite: token", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	uid := session.UserID(ctx)
	// Upsert-by-replace (c): undangan PENDING existing utk email ini diganti
	// (bukan ditolak bentrok/menumpuk) — admin yang re-undang mengharapkan field
	// terbarunya yang berlaku. rows>0 = ini KIRIM ULANG (bukan undangan baru) —
	// dipakai memilih pesan sukses; tanpa pembeda ini, re-undang tampak seperti
	// tak terjadi apa-apa (baris "Undangan Menunggu" tak berubah kasat mata).
	rows, e := q.DeletePendingInviteByEmail(ctx, db.DeletePendingInviteByEmailParams{TenantID: tenantID, Lower: email})
	if e != nil {
		h.Log.Warn("invite: hapus pending lama", "err", e)
	}
	resent := rows > 0
	if _, err := q.CreateInvite(ctx, db.CreateInviteParams{
		TenantID:     tenantID,
		Email:        email,
		Role:         authz.RoleNameMember,
		Token:        token,
		InvitedBy:    &uid,
		ExpiresAt:    pgtype.Timestamptz{Time: time.Now().Add(inviteTTL), Valid: true},
		BusinessRole: businessRolePtr,
		Kind:         kind,
	}); err != nil {
		h.Log.Error("invite: create", "err", err)
		wsRedirect(w, r, "/members", "failed")
		return
	}
	// metadata TANPA PII: email undangan tak dicatat, hanya role/kind.
	h.auditWorkspace(ctx, uid, "invite.create", tenantID, map[string]string{"role": authz.RoleNameMember, "kind": kind})
	okCode := "invited"
	if resent {
		okCode = "invite_resent"
	}
	wsRedirectOK(w, r, "/members", okCode)
}
