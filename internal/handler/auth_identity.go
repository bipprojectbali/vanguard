package handler

import (
	"context"
	"errors"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// auth_identity.go — pembentukan sesi identitas pasca-login (startIdentity) &
// cek staf platform (isStaff). Dipisah dari auth.go agar file di bawah ambang
// tipe Route/Handler (150). Satu paket handler.

// errAccountBlocked & errAccountDisabled = status yang menolak login.
var (
	errAccountBlocked  = errors.New("akun diblokir")
	errAccountDisabled = errors.New("akun dinonaktifkan")
)

// startIdentity memutar token session (anti fixation), lalu menyimpan identitas
// lengkap: workspace AKTIF + role EFEKTIF di workspace itu + gate status. Root env
// kebal gate status. Dipakai password login & Google callback. method
// ("password"/"google") dicatat di audit auth.login.
//
// preferTenant: workspace yang ingin diaktifkan (mis. baru dibuat saat register).
// 0 = pilih workspace pertama user. Bila user tak punya workspace sama sekali,
// session tetap dibuat dgn tenant 0 — middleware Scope yang mengarahkan ke
// /workspace/new (keadaan sah di model membership).
func (h *Handler) startIdentity(r *http.Request, u db.User, method string, preferTenant int64) error {
	isRoot := isSuperAdminEmail(u.Email)

	// Gate status — root lolos (tak bisa dikunci lewat status DB).
	if !isRoot {
		switch u.Status {
		case "blocked":
			return errAccountBlocked
		case "disabled":
			return errAccountDisabled
		}
	}

	// Workspace aktif + role tenant di dalamnya. Pre-identity (belum ada tx ber-
	// scope) → WithSuper; memberships/platform_staff memang tanpa RLS.
	var (
		tenantID   int64
		tenantName string
		tenantSlug string
		tenantRole = authz.RoleNameMember
	)
	if err := db.WithSuper(r.Context(), h.Pool, func(q *db.Queries) error {
		// super_admin = OWNER workspace primer (0007). Dilakukan SEBELUM membaca
		// keanggotaan, supaya baris yang baru dibuat ikut terpilih di login yang
		// SAMA — kalau sesudahnya, super_admin pertama mendarat tanpa workspace
		// dan diarahkan ke /workspace/new padahal rumahnya sudah ada.
		//
		// Fail-soft: kehilangan kepemilikan sementara jauh lebih ringan daripada
		// super_admin yang tak bisa masuk sama sekali.
		if isRoot {
			if e := h.ensurePrimaryOwner(r.Context(), q, u.ID); e != nil {
				h.Log.Warn("startIdentity: pasang owner workspace primer", "uid", u.ID, "err", e)
			}
		}
		ms, e := q.ListMembershipsByUser(r.Context(), u.ID)
		if e != nil || len(ms) == 0 {
			return e // tanpa workspace → tenant tetap 0 (Scope arahkan buat baru)
		}
		pick := ms[0] // default: workspace pertama (terlama)
		if preferTenant != 0 {
			for _, m := range ms {
				if m.TenantID == preferTenant {
					pick = m
					break
				}
			}
		}
		tenantID, tenantName, tenantSlug, tenantRole = pick.TenantID, pick.Name, pick.Slug, pick.Role
		return nil
	}); err != nil {
		h.Log.Warn("startIdentity: load memberships", "err", err)
	}

	// Role efektif 2-bidang: env super_admin > staff (platform_staff) > role tenant
	// di workspace aktif. Lookup staff fail-soft.
	role := tenantRole
	if isRoot {
		role = authz.RoleNameSuperAdmin // env override menang atas semua
	} else if staff, err := h.isStaff(r.Context(), u.Email); err == nil && staff {
		role = authz.RoleNameStaff
	}

	if err := session.Renew(r.Context()); err != nil {
		return err
	}
	avatar := ""
	if u.AvatarUrl != nil {
		avatar = *u.AvatarUrl
	}
	session.SetIdentity(r.Context(), u.ID, u.Email, role, isRoot, tenantID, tenantName, tenantSlug, avatar)
	// Jejak login (fail-soft) — untuk panel aktivitas. actor = user sendiri.
	h.auditAuth(r.Context(), u.ID, "auth.login", method)
	return nil
}

// isStaff melaporkan apakah email = operator platform (platform_staff). Dipakai
// jalur pre-identity (login/oauth) — platform_staff TANPA RLS, WithSuper aman.
func (h *Handler) isStaff(ctx context.Context, email string) (bool, error) {
	var ok bool
	err := db.WithSuper(ctx, h.Pool, func(q *db.Queries) error {
		v, e := q.IsPlatformStaff(ctx, email)
		ok = v
		return e
	})
	return ok, err
}
