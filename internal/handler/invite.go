package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/oauth"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// invite.go — undang orang ke workspace. Undangan dibuat owner/admin, penerima
// membuka link bertoken untuk bergabung. Email BELUM dikirim (task terbuka):
// link ditampilkan di panel anggota untuk disalin manual.
//
// TODO(invite): kirim email otomatis — butuh SMTP/provider (net/smtp stdlib,
// nol dependency). Sampai itu ada, link disalin manual dari halaman anggota.

// inviteTTL = masa berlaku undangan. Cukup lama untuk dibagikan, cukup pendek
// agar token bocor tak berlaku selamanya.
const inviteTTL = 7 * 24 * time.Hour

// InviteCreate — POST /w/{workspace}/members/invite. Buat undangan (owner/admin
// saja). BL-170: role tenant SELALU "member" (promosi admin terjadi PASCA-join
// lewat panel Anggota, bukan lagi dipilih di form Undang) — form kini menentukan
// Peran CRM (business_role, opsional) + Jenis Anggota (kind, wajib), sepasang
// sumbu yang diterapkan otomatis saat undangan diterima (acceptInvite/
// acceptInvitesByEmail).
func (h *Handler) InviteCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageMembers(ctx) {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
	tenantID := session.TenantID(ctx)
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
	businessRole := strings.TrimSpace(r.FormValue("business_role"))
	var businessRolePtr *string
	if businessRole != "" {
		// Tenant-aware DAN cocok Kind yang dipilih — pertahanan berlapis di atas
		// cascading select (yang bisa dilewati manipulasi form langsung).
		br, e := h.q(ctx).GetBusinessRole(ctx, db.GetBusinessRoleParams{TenantID: tenantID, Name: businessRole})
		if e != nil || br.Kind != kind {
			wsRedirect(w, r, "/members", "crm_role")
			return
		}
		businessRolePtr = &businessRole
	}
	// Guard (b): email yang sudah jadi anggota tak boleh diundang ulang — bukan
	// error DB (invites tak punya UNIQUE lintas-member), jadi dicek eksplisit.
	if exists, e := h.q(ctx).MemberExistsByEmail(ctx, db.MemberExistsByEmailParams{TenantID: tenantID, Lower: email}); e == nil && exists {
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
	rows, e := h.q(ctx).DeletePendingInviteByEmail(ctx, db.DeletePendingInviteByEmailParams{TenantID: tenantID, Lower: email})
	if e != nil {
		h.Log.Warn("invite: hapus pending lama", "err", e)
	}
	resent := rows > 0
	if _, err := h.q(ctx).CreateInvite(ctx, db.CreateInviteParams{
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

// InviteDelete — POST /w/{workspace}/members/invite/{id}/delete. Batalkan undangan.
func (h *Handler) InviteDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageMembers(ctx) {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		wsRedirect(w, r, "/members", "notfound")
		return
	}
	if err := h.q(ctx).DeleteInvite(ctx, db.DeleteInviteParams{
		ID: id, TenantID: session.TenantID(ctx), // filter tenant: tak bisa hapus milik orang lain
	}); err != nil {
		h.Log.Error("invite: delete", "err", err)
	}
	wsRedirect(w, r, "/members", "")
}

// InvitePage — GET /invite/{token}. PUBLIK: penerima belum tentu login (atau
// belum punya akun). Belum login → simpan token di session, arahkan ke register.
// Sudah login → halaman konfirmasi "Gabung ke workspace X?".
func (h *Handler) InvitePage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := chi.URLParam(r, "token")

	inv, err := h.loadInvite(ctx, token)
	if err != nil {
		h.renderPage(w, r, "Undangan", panel.InviteResult("", inviteErrText(err), false))
		return
	}
	if session.UserID(ctx) == 0 {
		// Belum login: ingat token agar setelah register/login otomatis diterima.
		session.PutPendingInvite(ctx, token)
		if err := session.WriteCookie(ctx, w); err != nil {
			h.Log.Error("invite: write cookie", "err", err)
		}
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}
	h.renderPage(w, r, "Undangan", panel.InviteResult(inv.TenantName, "", true))
}

// InviteAccept — POST /invite/{token}/accept. Butuh login. Buat membership +
// tandai undangan terpakai, lalu pindah ke workspace itu.
func (h *Handler) InviteAccept(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := chi.URLParam(r, "token")
	uid := session.UserID(ctx)
	if uid == 0 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := h.acceptInvite(ctx, token, uid); err != nil {
		h.renderPage(w, r, "Undangan", panel.InviteResult("", inviteErrText(err), false))
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
