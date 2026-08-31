package handler

import (
	"errors"
	"net/http"
	"strings"

	"go_starter/internal/auth"
	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// auth_login.go — handler login/logout password (dev-only auth). Dipisah dari
// auth.go agar tiap file di bawah ambang tipe Route/Handler (150). Satu paket
// handler — perilaku identik.

// Login — POST /login (native form, PRG). Verifikasi argon2id, mulai session,
// redirect 303 ke home. Gagal → /login?err=CODE (pesan generik anti-enumeration).
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	password := r.FormValue("password")
	if email == "" || password == "" {
		http.Redirect(w, r, "/login?err=fields", http.StatusSeeOther)
		return
	}

	// Pre-identity: tenant user belum diketahui (login MEMUTUSKANNYA dari hasil ini).
	// email UNIQUE global → WithSuper (bypass RLS) untuk mencari lintas-tenant.
	var user db.User
	err := db.WithSuper(r.Context(), h.Pool, func(q *db.Queries) error {
		u, e := q.GetUserByEmail(r.Context(), email)
		user = u
		return e
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// Kode generik — jangan bocorkan email terdaftar/tidak (anti user-enumeration).
			http.Redirect(w, r, "/login?err=invalid", http.StatusSeeOther)
			return
		}
		h.Log.Error("login: get user", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Akun Google-only (pass_hash NULL) tak bisa login password. Kode generik
	// yang sama (anti user-enumeration) — jangan ungkap bahwa akun ini OAuth.
	if user.PassHash == nil {
		http.Redirect(w, r, "/login?err=invalid", http.StatusSeeOther)
		return
	}

	ok, err := auth.VerifyPassword(password, *user.PassHash)
	if err != nil {
		h.Log.Error("login: verify", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Redirect(w, r, "/login?err=invalid", http.StatusSeeOther)
		return
	}

	if err := h.startIdentity(r, user, "password", 0); err != nil {
		if errors.Is(err, errAccountBlocked) || errors.Is(err, errAccountDisabled) {
			http.Redirect(w, r, "/login?err=inactive", http.StatusSeeOther)
			return
		}
		h.Log.Error("login: start identity", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.acceptPendingInvite(r) // datang lewat tautan undangan? langsung gabung
	http.Redirect(w, r, homeFor(r.Context()), http.StatusSeeOther)
}

// Logout — POST /logout (native form, PRG). Hancurkan session, redirect 303.
// SELALU HTTP redirect (bukan SSE): redirect via SSE menyuntik <script> yang
// diblokir CSP proyek — logout tak akan berpindah halaman (lihat gotcha).
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	// Jejak logout SEBELUM Clear (uid hilang setelahnya). Fail-soft.
	if uid := session.UserID(r.Context()); uid != 0 {
		h.auditAuth(r.Context(), uid, "auth.logout", "")
	}
	if err := session.Clear(r.Context()); err != nil {
		h.Log.Error("logout", "err", err)
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
