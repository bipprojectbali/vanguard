package handler

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/oauth"
	"go_starter/internal/session"
)

// googleProvider adalah kontrak minimal yang dibutuhkan handler dari OAuth
// provider. Interface (bukan *oauth.Provider konkret) agar test bisa menyuntik
// stub tanpa memanggil Google. *oauth.Provider memenuhi ini.
type googleProvider interface {
	AuthURL(state, nonce, verifier string) string
	Exchange(ctx context.Context, code, verifier string) (rawIDToken string, err error)
	oauth.Verifier // VerifyIDToken(ctx, rawIDToken, wantNonce) (*Claims, error)
}

// googleOAuth di-inject saat startup via SetGoogleOAuth (pola setter global,
// konsisten dgn SetCSSPath/session.Init). nil bila Google tak terkonfigurasi.
var googleOAuth googleProvider

// SetGoogleOAuth menyuntik provider Google (dipanggil dari main bila kredensial
// tersedia). Bila tak dipanggil, handler membalas 503 (Google nonaktif).
func SetGoogleOAuth(p googleProvider) { googleOAuth = p }

// GoogleLogin — GET /api/auth/google. Mulai authorization-code flow: generate
// state/nonce/verifier, simpan di session, redirect ke consent Google.
func (h *Handler) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	if googleOAuth == nil {
		http.Error(w, "login Google tidak tersedia", http.StatusServiceUnavailable)
		return
	}
	state, err := oauth.NewState()
	if err != nil {
		h.Log.Error("google login: state", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	nonce, err := oauth.NewState()
	if err != nil {
		h.Log.Error("google login: nonce", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	verifier := oauth.NewVerifier()

	session.PutOAuthFlow(r.Context(), state, nonce, verifier)
	// Commit session + tulis cookie SEBELUM redirect (state harus persist agar
	// bisa diverifikasi saat callback).
	if err := session.WriteCookie(r.Context(), w); err != nil {
		h.Log.Error("google login: write cookie", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, googleOAuth.AuthURL(state, nonce, verifier), http.StatusFound)
}

// GoogleCallback — GET /api/auth/callback/google. Verifikasi state (anti-CSRF),
// tukar code (PKCE), verifikasi id_token (+nonce, +email_verified), lalu
// find-or-link user dalam satu transaksi dan mulai session.
func (h *Handler) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if googleOAuth == nil {
		http.Error(w, "login Google tidak tersedia", http.StatusServiceUnavailable)
		return
	}
	ctx := r.Context()

	// Ambil state/nonce/verifier tersimpan lalu HAPUS (one-time), apa pun hasilnya.
	wantState, nonce, verifier := session.OAuthFlow(ctx)
	session.ClearOAuthFlow(ctx)

	// Anti-CSRF: state dari Google harus cocok dengan yang kita simpan.
	gotState := r.URL.Query().Get("state")
	if wantState == "" || subtle.ConstantTimeCompare([]byte(gotState), []byte(wantState)) != 1 {
		h.Log.Warn("google callback: state mismatch")
		http.Error(w, "state tidak valid", http.StatusBadRequest)
		return
	}
	// Google mengirim error (mis. user membatalkan consent).
	if e := r.URL.Query().Get("error"); e != "" {
		h.Log.Warn("google callback: provider error", "error", e)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	code := r.URL.Query().Get("code")
	rawIDToken, err := googleOAuth.Exchange(ctx, code, verifier)
	if err != nil {
		h.Log.Error("google callback: exchange", "err", err)
		http.Error(w, "gagal menukar kode", http.StatusBadGateway)
		return
	}
	claims, err := googleOAuth.VerifyIDToken(ctx, rawIDToken, nonce)
	if err != nil {
		// email_verified=false, nonce mismatch, signature invalid → tolak.
		h.Log.Warn("google callback: verify id_token", "err", err)
		http.Error(w, "verifikasi Google gagal", http.StatusUnauthorized)
		return
	}

	userID, err := h.findOrLinkGoogleUser(ctx, claims)
	if err != nil {
		h.Log.Error("google callback: find-or-link", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Ambil user (dgn role/status/avatar terkini) lalu mulai identitas. Pre-identity
	// (tenant belum di session) → WithSuper. uid global-unique.
	var user db.User
	if err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		u, e := q.GetUser(ctx, userID)
		user = u
		return e
	}); err != nil {
		h.Log.Error("google callback: get user", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := h.startIdentity(r, user, "google", 0); err != nil {
		if errors.Is(err, errAccountBlocked) || errors.Is(err, errAccountDisabled) {
			// Akun tak aktif — tolak, arahkan ke login dgn pesan.
			session.ClearOAuthFlow(ctx)
			http.Redirect(w, r, "/login?err=inactive", http.StatusSeeOther)
			return
		}
		h.Log.Error("google callback: start identity", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if err := session.WriteCookie(ctx, w); err != nil {
		h.Log.Error("google callback: write cookie", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, homeFor(ctx), http.StatusSeeOther)
}
