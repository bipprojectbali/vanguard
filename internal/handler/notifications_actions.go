package handler

import (
	"net/http"
	"net/url"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/go-chi/chi/v5"
)

// notifications_actions.go — aksi terima/tolak undangan dari halaman notifikasi
// (notifications.go). SEMUA akses lewat db.WithSuper (lihat catatan di
// notifications.go).

// NotificationAccept — POST /notifications/invite/{token}/accept. Memakai ulang
// acceptInvite (invite_service.go) — jalur terima yang sama dengan /invite/{token},
// termasuk guard one-time & pindah workspace aktif.
func (h *Handler) NotificationAccept(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		http.Redirect(w, r, "/notifications?err=notfound", http.StatusSeeOther)
		return
	}
	if err := h.acceptInvite(r.Context(), token, session.UserID(r.Context())); err != nil {
		h.Log.Warn("notifications: accept invite", "err", err)
		http.Redirect(w, r, "/notifications?err="+inviteErrCode(err), http.StatusSeeOther)
		return
	}
	// Undangan diterima → user kini di workspace itu; antar ke berandanya.
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// NotificationDecline — POST /notifications/invite/{token}/decline. DeclineInvite
// mengunci token DAN email penerima: pemegang token tak bisa menolak undangan
// milik orang lain.
func (h *Handler) NotificationDecline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token := chi.URLParam(r, "token")
	if token == "" {
		http.Redirect(w, r, "/notifications?err=notfound", http.StatusSeeOther)
		return
	}
	email := normalizeEmail(session.Email(ctx))
	// BL-103: pertahankan asal agar akun platform kembali ke shell workspace, bukan
	// memantul ke panel dev. Ditempel ke redirect sukses & gagal (keduanya /notifications).
	suffix := ""
	if from := r.URL.Query().Get("from"); from != "" {
		suffix = "&from=" + url.QueryEscape(from)
	}
	if err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		return q.DeclineInvite(ctx, db.DeclineInviteParams{Token: token, Email: email})
	}); err != nil {
		h.Log.Error("notifications: decline invite", "err", err)
		http.Redirect(w, r, "/notifications?err=failed"+suffix, http.StatusSeeOther)
		return
	}
	dest := "/notifications"
	if suffix != "" {
		dest += "?" + strings.TrimPrefix(suffix, "&")
	}
	http.Redirect(w, r, dest, http.StatusSeeOther)
}
