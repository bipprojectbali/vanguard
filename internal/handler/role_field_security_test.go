package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/fls"
)

// role_field_security_test.go — aksi simpan Field Security SATU peran (BL-145
// subtask 3), POST /w/{workspace}/roles/{name}/field-security. Menggantikan
// field_security_test.go (jalur bulk /field-security, dihapus). Yang dijaga:
//
//   - Gerbang crm:field_security (sumbu BISNIS, TERPISAH dari crm:roles):
//     hanya admin (glob crm:*) boleh menyimpan; peran lain & "" ditolak, tanpa
//     menyimpan baris apa pun — berlaku juga saat peran TARGET adalah sistem
//     (admin kini punya form FLS sendiri, BL-145 subtask 3).
//   - Replace-all: menyunting satu peran TAK mengubah nilai efektif peran lain
//     (dibekukan ke keadaan sebelum POST).
//   - Coercion server-side (bukan cuma disabled di klien): edit⇒view; kedua
//     level Contacts/Leads "none" → paksa view+edit false; tak ada level
//     "write" di keduanya → paksa edit false (view boleh tetap terkirim);
//     peran SISTEM melewati guard level ini sepenuhnya (diwakili glob crm:*,
//     tak ada matriks Contacts/Leads untuk direaksikan).
//   - PRG ke /roles/{name} (bukan lagi /roles bulk) dengan ok=fsec_saved; audit
//     metadata memuat field "role" baru.
//   - Read-only (arsip) menolak simpan walau admin.
//
// Pakai setupRoles: handler membaca ListBusinessRoles dari DB, jadi peran
// bawaan wajib tertanam di tabel — bukan sekadar dimuat ke enforcer.

// fsecPolicies mendaftar kebijakan FLS phone tenant test langsung dari pool —
// untuk membuktikan sebuah POST menyimpan / tak menyimpan baris.
func (e *testEnv) fsecPolicies(t *testing.T) map[string]struct{ view, edit bool } {
	t.Helper()
	rows, err := e.q.ListFieldSecurityPolicies(t.Context(), e.tenantID)
	if err != nil {
		t.Fatalf("list field security policies: %v", err)
	}
	out := make(map[string]struct{ view, edit bool }, len(rows))
	for _, r := range rows {
		out[r.BusinessRole] = struct{ view, edit bool }{r.CanViewPhone, r.CanEditPhone}
	}
	return out
}

// --- gerbang crm:field_security ---------------------------------------------

// TestRoleFieldSecurity_Gate: hanya admin boleh menyimpan FLS satu peran; peran
// CRM lain & "" ditolak err=forbidden, tanpa menyimpan baris apa pun.
func TestRoleFieldSecurity_Gate(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"sales", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupRoles(t)
			form := url.Values{"view": {"1"}}
			req := rolesReq(http.MethodPost, "/w/test/roles/manager/field-security", form, "manager")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.RoleFieldSecurityUpdate)

			if c.allow {
				if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=fsec_saved") {
					t.Errorf("admin simpan harus ok=fsec_saved, got %q (status %d)", loc, rec.Code)
				}
				return
			}
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=forbidden") {
				t.Errorf("role %q harus ditolak err=forbidden, got %q", c.role, loc)
			}
			if len(env.fsecPolicies(t)) != 0 {
				t.Errorf("role %q ditolak tak boleh menyimpan kebijakan", c.role)
			}
		})
	}
}

// TestRoleFieldSecurity_SystemRoleTargetAllowed: menyunting FLS peran SISTEM
// (admin) kini didukung (dulu tak ada form sama sekali) — gerbangnya sama
// (canManageFieldSecurity), bukan canManageRoles, jadi tak butuh perlakuan
// khusus di handler untuk mengizinkan targetnya.
func TestRoleFieldSecurity_SystemRoleTargetAllowed(t *testing.T) {
	env, uid := setupRoles(t)
	form := url.Values{"view": {"1"}, "edit": {"1"}}
	req := rolesReq(http.MethodPost, "/w/test/roles/admin/field-security", form, "admin")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleFieldSecurityUpdate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=fsec_saved") {
		t.Fatalf("harus ok=fsec_saved, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	if p := env.fsecPolicies(t)["admin"]; !p.view || !p.edit {
		t.Errorf("admin harus tersimpan lihat+sunting, got %+v", p)
	}
}

// --- replace-all preserva peran lain -----------------------------------------

// TestRoleFieldSecurity_ReplaceAllPreservesOtherRoles: menyunting Manager tak
// mengubah nilai EFEKTIF Sales (dibekukan ke default sebelum POST — sama
// dengan pola bulk lama, hanya scope form yang mengecil).
func TestRoleFieldSecurity_ReplaceAllPreservesOtherRoles(t *testing.T) {
	env, uid := setupRoles(t)
	salesViewBefore := fls.CanViewPhone(env.tenantID, "sales")
	salesEditBefore := fls.CanEditPhone(env.tenantID, "sales")

	form := url.Values{"view": {"1"}, "edit": {"1"}}
	req := rolesReq(http.MethodPost, "/w/test/roles/manager/field-security", form, "manager")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleFieldSecurityUpdate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=fsec_saved") {
		t.Fatalf("harus ok=fsec_saved, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}

	if p := env.fsecPolicies(t)["manager"]; !p.view || !p.edit {
		t.Errorf("Manager harus tersimpan lihat+sunting, got %+v", p)
	}
	if fls.CanViewPhone(env.tenantID, "sales") != salesViewBefore {
		t.Error("Sales: nilai efektif 'lihat' tak boleh berubah")
	}
	if fls.CanEditPhone(env.tenantID, "sales") != salesEditBefore {
		t.Error("Sales: nilai efektif 'sunting' tak boleh berubah")
	}
	env.assertAudited(t, "workspace.field_security")
}

// --- coercion server-side -----------------------------------------------------

// TestRoleFieldSecurity_CoerceEditImpliesView: "sunting" tanpa "lihat" naik
// jadi lihat+sunting. Manager (contacts=write, leads=write) tak kena guard
// level, jadi murni menguji coercion edit⇒view.
func TestRoleFieldSecurity_CoerceEditImpliesView(t *testing.T) {
	env, uid := setupRoles(t)
	form := url.Values{"edit": {"1"}} // sunting saja, tanpa view
	req := rolesReq(http.MethodPost, "/w/test/roles/manager/field-security", form, "manager")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleFieldSecurityUpdate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=fsec_saved") {
		t.Fatalf("harus ok=fsec_saved, got %q", loc)
	}
	if p := env.fsecPolicies(t)["manager"]; !p.view || !p.edit {
		t.Errorf("edit tanpa view harus di-coerce jadi lihat+sunting, got %+v", p)
	}
}

// TestRoleFieldSecurity_CoerceBothNoneForcesViewFalse: peran KUSTOM tanpa izin
// apa pun di Contacts maupun Leads (keduanya "none") → view DIPAKSA false
// walau form kirim view=1&edit=1 (klien tak bisa dipercaya).
func TestRoleFieldSecurity_CoerceBothNoneForcesViewFalse(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "noaccess", "Tanpa Akses", "own", false)

	form := url.Values{"view": {"1"}, "edit": {"1"}}
	req := rolesReq(http.MethodPost, "/w/test/roles/noaccess/field-security", form, "noaccess")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleFieldSecurityUpdate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=fsec_saved") {
		t.Fatalf("harus ok=fsec_saved, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	if p := env.fsecPolicies(t)["noaccess"]; p.view || p.edit {
		t.Errorf("Contacts+Leads none: view & edit harus dipaksa false, got %+v", p)
	}
}

// TestRoleFieldSecurity_CoerceNoWriteForcesEditFalse: Support (contacts=read,
// leads=none — tak ada "write" di keduanya, tapi bukan keduanya "none") →
// edit DIPAKSA false, tapi view boleh tetap terkirim (contacts=read cukup
// bermakna untuk "lihat").
func TestRoleFieldSecurity_CoerceNoWriteForcesEditFalse(t *testing.T) {
	env, uid := setupRoles(t)
	form := url.Values{"view": {"1"}, "edit": {"1"}}
	req := rolesReq(http.MethodPost, "/w/test/roles/support/field-security", form, "support")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleFieldSecurityUpdate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=fsec_saved") {
		t.Fatalf("harus ok=fsec_saved, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	p := env.fsecPolicies(t)["support"]
	if p.edit {
		t.Error("tak ada level write di Contacts/Leads: edit harus dipaksa false")
	}
	if !p.view {
		t.Error("Support contacts=read: view boleh tetap tersimpan true")
	}
}

// TestRoleFieldSecurity_SystemRoleBypassesLevelGuard: peran SISTEM (admin)
// melewati guard level Contacts/Leads sepenuhnya (diwakili glob crm:*, tak
// pernah "none" secara matriks) — view=1&edit=1 tersimpan apa adanya, tanpa
// dipaksa false oleh coercion level yang hanya berlaku !role.IsSystem.
func TestRoleFieldSecurity_SystemRoleBypassesLevelGuard(t *testing.T) {
	env, uid := setupRoles(t)
	form := url.Values{"view": {"1"}, "edit": {"1"}}
	req := rolesReq(http.MethodPost, "/w/test/roles/admin/field-security", form, "admin")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleFieldSecurityUpdate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=fsec_saved") {
		t.Fatalf("harus ok=fsec_saved, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	if p := env.fsecPolicies(t)["admin"]; !p.view || !p.edit {
		t.Errorf("peran sistem harus lolos tanpa guard level, got %+v", p)
	}
}

// --- PRG + read-only -----------------------------------------------------

// TestRoleFieldSecurity_RedirectsToRoleDetail: PRG kembali ke /roles/{name}
// (bukan lagi /roles bulk) dengan ok=fsec_saved.
func TestRoleFieldSecurity_RedirectsToRoleDetail(t *testing.T) {
	env, uid := setupRoles(t)
	form := url.Values{"view": {"1"}}
	req := rolesReq(http.MethodPost, "/w/test/roles/manager/field-security", form, "manager")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleFieldSecurityUpdate)
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/roles/manager") || !strings.Contains(loc, "ok=fsec_saved") {
		t.Errorf("harus redirect ke /roles/manager?ok=fsec_saved, got %q", loc)
	}
}

// TestRoleFieldSecurity_ReadOnlyBlocksPost: workspace arsip (read-only) menolak
// simpan walau admin — perubahan kebijakan keamanan tak boleh lolos di
// workspace beku.
func TestRoleFieldSecurity_ReadOnlyBlocksPost(t *testing.T) {
	env, uid := setupRoles(t)
	form := url.Values{"view": {"1"}}
	req := rolesReq(http.MethodPost, "/w/test/roles/manager/field-security", form, "manager")
	rec := env.runAccount(uid, "owner", "admin", req, func(w http.ResponseWriter, r *http.Request) {
		env.h.RoleFieldSecurityUpdate(w, r.WithContext(withTenantStatus(r.Context(), TenantArchived)))
	})
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=forbidden") {
		t.Errorf("read-only harus ditolak err=forbidden, got %q", loc)
	}
	if len(env.fsecPolicies(t)) != 0 {
		t.Error("read-only tak boleh menyimpan kebijakan")
	}
}
