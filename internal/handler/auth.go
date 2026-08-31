package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/appmode"
	"go_starter/internal/auth"
	"go_starter/internal/db"
	"go_starter/internal/ui/pages"
)

// maxWorkspaceNameLen membatasi panjang nama workspace (validasi input —
// user-modifiable, WAJIB divalidasi backend).
const maxWorkspaceNameLen = 60

// LoginPage — GET /login (full page). Form password hanya di dev (devMode).
// ?err= (dari redirect PRG) → alert. Menutup juga jalur /login?err=inactive dari
// RefreshIdentity/OAuth yang dulu tak pernah dirender (pesan hilang senyap).
func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "Masuk", pages.Login(devMode, authErrMsg(r.URL.Query().Get("err"))))
}

// RegisterPage — GET /register (full page). ?err= → alert (pola PRG).
func (h *Handler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	h.renderPage(w, r, "Daftar", pages.Register(authErrMsg(r.URL.Query().Get("err")), appmode.IsMulti()))
}

// authErrMsg memetakan kode error auth (query ?err=) ke pesan ramah. Kode ringkas
// di URL (bukan pesan penuh) agar rapi + tak bocor detail. "" → tak ada alert.
func authErrMsg(code string) string {
	switch code {
	case "invalid":
		return "Email atau password salah"
	case "fields":
		return "Email dan password wajib diisi"
	case "short":
		return "Password minimal 8 karakter"
	case "exists":
		return "Email sudah terdaftar"
	case "workspace":
		return "Nama workspace wajib diisi"
	case "inactive":
		return "Akun tidak aktif. Hubungi administrator."
	default:
		return ""
	}
}

// Register — POST /register (native form, PRG). Buat workspace+owner, lalu login
// otomatis & redirect 303 ke home. Gagal → redirect /register?err=CODE.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(strings.ToLower(r.FormValue("email")))
	password := r.FormValue("password")
	// Nama workspace hanya relevan di mode MULTI (pendaftar membuat workspace-nya
	// sendiri). Di mode single ia bergabung ke aplikasi yang sudah ada & sudah
	// bernama — mewajibkannya di sana membuat pendaftaran mustahil diselesaikan,
	// sebab formnya pun tak menanyakannya.
	workspace := strings.TrimSpace(r.FormValue("workspace"))
	if appmode.IsMulti() && (workspace == "" || len(workspace) > maxWorkspaceNameLen) {
		http.Redirect(w, r, "/register?err=workspace", http.StatusSeeOther)
		return
	}
	if email == "" || password == "" {
		http.Redirect(w, r, "/register?err=fields", http.StatusSeeOther)
		return
	}
	if len(password) < 8 {
		http.Redirect(w, r, "/register?err=short", http.StatusSeeOther)
		return
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		h.Log.Error("hash password", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// Register = buat USER (identitas global) + WORKSPACE pertama + MEMBERSHIP owner.
	// Pre-identity: tenant belum ada → WithSuper (bypass RLS). Nama = input user;
	// slug = unik auto-suffix (nama boleh duplikat, slug tidak). Ketiganya dalam
	// SATU tx → atomic (gagal di tengah tak meninggalkan user/tenant yatim).
	var user db.User
	var tenantID int64
	err = db.WithSuper(r.Context(), h.Pool, func(q *db.Queries) error {
		u, e := q.CreateUser(r.Context(), db.CreateUserParams{Email: email, PassHash: &hash})
		if e != nil {
			return e
		}
		user = u
		// Penempatan berbeda per mode (0006 §6): multi = workspace baru + owner,
		// single = gabung ke aplikasi tunggal sebagai MEMBER. Dipusatkan agar
		// jalur ini & OAuth tak bisa menyimpang satu sama lain.
		t, e := placeNewUser(r.Context(), q, u.ID, workspace)
		if e != nil {
			return e
		}
		tenantID = t.ID
		return nil
	})
	if err != nil {
		// Email/slug unik: pelanggaran constraint = email sudah terpakai.
		h.Log.Warn("register: create tenant+user", "err", err)
		http.Redirect(w, r, "/register?err=exists", http.StatusSeeOther)
		return
	}

	if err := h.startIdentity(r, user, "password", tenantID); err != nil {
		h.Log.Error("register: start identity", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	h.acceptPendingInvite(r) // datang lewat tautan undangan? langsung gabung
	// Native redirect (bukan SSE): scs LoadAndSave commit cookie otomatis — tak
	// perlu WriteCookie manual (itu hanya utk jalur NewSSE yang bypass scs).
	http.Redirect(w, r, homeFor(r.Context()), http.StatusSeeOther)
}
