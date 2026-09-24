package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/go-chi/chi/v5"
)

// invite.go — undang orang ke workspace. Undangan dibuat owner/admin, penerima
// membuka link bertoken untuk bergabung. Email BELUM dikirim (task terbuka):
// link ditampilkan di panel anggota untuk disalin manual.
//
// TODO(invite): kirim email otomatis — butuh SMTP/provider (net/smtp stdlib,
// nol dependency). Sampai itu ada, link disalin manual dari halaman anggota.
//
// InviteCreate (+ const inviteTTL) dipisah ke invite_create.go agar tiap file
// di bawah batas 150 baris.

// InviteDelete — POST /w/{workspace}/members/invite/{id}/delete. Batalkan undangan.
//
// Gerbang MELEBAR (BL-171): pengelola tenant ATAU aktor business-axis
// ber-"Kelola" crm:members, dibatasi target invite ∈ actorKindScope. Tak ada
// query GetInviteByID tenant-scoped di sqlc — dipakai ListInvitesByTenant
// (sudah dimuat halaman Anggota, daftar undangan pending kecil) untuk
// menemukan kind target sebelum hapus, alih-alih menambah query baru untuk
// satu pemakaian.
func (h *Handler) InviteDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageMembers(ctx) && crmMemberAccess(ctx) != memberAccessManage {
		wsRedirect(w, r, "/members", "forbidden")
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		wsRedirect(w, r, "/members", "notfound")
		return
	}
	tenantID := session.TenantID(ctx)
	q := h.q(ctx)
	if !canManageMembers(ctx) {
		scope := actorKindScope(ctx, q)
		invites, e := q.ListInvitesByTenant(ctx, tenantID)
		if e != nil {
			wsRedirect(w, r, "/members", "failed")
			return
		}
		found := false
		for _, inv := range invites {
			if inv.ID == id {
				found = true
				if !scope.Allows(inv.Kind) {
					wsRedirect(w, r, "/members", "forbidden")
					return
				}
				break
			}
		}
		if !found {
			wsRedirect(w, r, "/members", "notfound")
			return
		}
	}
	if err := q.DeleteInvite(ctx, db.DeleteInviteParams{
		ID: id, TenantID: tenantID, // filter tenant: tak bisa hapus milik orang lain
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
