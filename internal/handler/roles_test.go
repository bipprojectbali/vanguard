package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/authz"
	"go_starter/internal/db"

	"github.com/go-chi/chi/v5"
)

// roles_test.go — panel peran CRM per-workspace (/w/{slug}/roles). Yang dijaga:
//
//   - Gerbang crm:roles (sumbu BISNIS): hanya pemegangnya (admin, lewat glob
//     crm:*) boleh membuka/menyunting; role CRM lain & tenant-role setinggi apa
//     pun (owner/super_admin) TANPA peran itu ditolak — tak ada short-circuit root.
//   - Validasi backend: nama peran, nama tampilan, cakupan data (F3) — bukan cuma
//     atribut form; input invalid tak menyentuh DB.
//   - Peran sistem (admin, is_system) kebal sunting & hapus.
//   - reload enforcer seketika: sunting matriks langsung mengubah CanBusiness
//     tanpa restart (bukti ReloadBusinessTenant dipanggil dengan set penuh tenant).
//   - hapus peran meng-unassign anggota lebih dulu (peran hilang ≠ orang hilang).
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS diuji
// terpisah di rls_test.go. Enforcer bisnis nyata dimuat dari DefaultBusinessRoles.

// --- setup & helper --------------------------------------------------------

// setupRoles = setupAccounts + menanam peran bawaan ke DB tenant test. Tanpa
// seed DB ini, reloadBusinessTenant (yang membaca DB) akan MENGHAPUS peran default
// yang dimuat setupAccounts ke enforcer in-memory — DB kosong direplay sebagai
// "tenant tak punya peran". Dengan seed, DB & enforcer sepakat sejak awal.
func setupRoles(t *testing.T) (*testEnv, int64) {
	t.Helper()
	env, uid := setupAccounts(t)
	if err := seedBusinessRoles(t.Context(), env.q, env.tenantID); err != nil {
		t.Fatalf("seed business roles: %v", err)
	}
	return env, uid
}

// rolesReq membangun request dengan chi param workspace (+ {name} bila diberi) dan
// body form terkode. name "" = tanpa param.
func rolesReq(method, target string, form url.Values, name string) *http.Request {
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("workspace", "test")
	if name != "" {
		rctx.URLParams.Add("name", name)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// roleFormValues = form minimal valid untuk create/update peran.
func roleFormValues(display, scope string) url.Values {
	return url.Values{"display_name": {display}, "data_scope": {scope}}
}

// seedRole menaruh satu peran langsung lewat pool (bypass handler) — untuk menguji
// update/delete tanpa merangkai create.
func (e *testEnv) seedRole(t *testing.T, name, display, scope string, system bool) {
	t.Helper()
	if _, err := e.q.CreateBusinessRole(t.Context(), db.CreateBusinessRoleParams{
		TenantID: e.tenantID, Name: name, DisplayName: display,
		DataScope: scope, IsSystem: system, CreatedBy: nil,
	}); err != nil {
		t.Fatalf("seed role %s: %v", name, err)
	}
}

// hasRole = true bila peran name masih ada di tenant test.
func (e *testEnv) hasRole(t *testing.T, name string) bool {
	t.Helper()
	_, err := e.q.GetBusinessRole(t.Context(), db.GetBusinessRoleParams{
		TenantID: e.tenantID, Name: name,
	})
	return err == nil
}

// hasPerm = true bila baris (role,obj,act) tercatat di matriks tenant test.
func (e *testEnv) hasPerm(t *testing.T, role, obj, act string) bool {
	t.Helper()
	rows, err := e.q.ListBusinessRolePermissionsByTenant(t.Context(), e.tenantID)
	if err != nil {
		t.Fatalf("list perms: %v", err)
	}
	for _, r := range rows {
		if r.Role == role && r.Obj == obj && r.Act == act {
			return true
		}
	}
	return false
}

// canBiz menjalankan CanBusiness dalam session (uid, businessRole) — untuk
// membuktikan efek reload enforcer di sisi izin, bukan cuma di DB.
func (e *testEnv) canBiz(uid int64, role, obj, act string) bool {
	var out bool
	req := rolesReq(http.MethodGet, "/w/test/roles", nil, "")
	e.runAccount(uid, "member", role, req, func(w http.ResponseWriter, r *http.Request) {
		out = authz.CanBusiness(r.Context(), obj, act)
	})
	return out
}

// --- gerbang crm:roles -----------------------------------------------------

// TestRoles_GateRead: hanya admin (pemegang crm:roles) boleh MEMBUKA panel;
// peran CRM lain & "" ditolak 403 dengan penjelasan.
func TestRoles_GateRead(t *testing.T) {
	env, uid := setupRoles(t)
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", false},
		{"sales", false},
		{"support", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := rolesReq(http.MethodGet, "/w/test/roles", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.RolesPage)
			if c.allow && rec.Code != http.StatusOK {
				t.Errorf("role %q harus lolos gate, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if !strings.Contains(rec.Body.String(), "admin CRM") {
					t.Errorf("penolakan harus menyebut admin CRM")
				}
			}
		})
	}
}

// TestRoles_GateTegakLurusPlatform: owner/super_admin TANPA peran CRM tetap
// DITOLAK — sumbu bisnis tak mewarisi otoritas platform.
func TestRoles_GateTegakLurusPlatform(t *testing.T) {
	env, uid := setupRoles(t)
	for _, tenantRole := range []string{"owner", "admin", "super_admin"} {
		req := rolesReq(http.MethodGet, "/w/test/roles", nil, "")
		rec := env.runAccount(uid, tenantRole, "", req, env.h.RolesPage)
		if rec.Code != http.StatusForbidden {
			t.Errorf("tenant role %q tanpa peran CRM harus 403, got %d", tenantRole, rec.Code)
		}
	}
}

// TestRoles_CreateGate: POST create hanya untuk admin; role lain ditolak
// (err=forbidden) & tak menyimpan peran apa pun.
func TestRoles_CreateGate(t *testing.T) {
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
			form := roleFormValues("Keuangan", "all")
			form.Set("name", "finance")
			req := rolesReq(http.MethodPost, "/w/test/roles", form, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.RoleCreate)

			if c.allow {
				if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
					t.Errorf("admin create harus ok=created, got %q (status %d)", loc, rec.Code)
				}
				if !env.hasRole(t, "finance") {
					t.Error("peran harus tersimpan")
				}
			} else {
				if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=forbidden") {
					t.Errorf("role %q harus ditolak err=forbidden, got %q", c.role, loc)
				}
				if env.hasRole(t, "finance") {
					t.Errorf("role %q ditolak tak boleh menyimpan peran", c.role)
				}
			}
		})
	}
}

// --- create ----------------------------------------------------------------

// TestRoles_CreateSuccess: admin buat peran → 303 ok=created, tersimpan dengan
// cakupan data yang dipilih, bukan sistem, matriks kosong, audit tercatat.
func TestRoles_CreateSuccess(t *testing.T) {
	env, uid := setupRoles(t)
	form := roleFormValues("Keuangan", "own")
	form.Set("name", "finance")
	req := rolesReq(http.MethodPost, "/w/test/roles", form, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleCreate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Fatalf("harus ok=created, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	got, err := env.q.GetBusinessRole(t.Context(), db.GetBusinessRoleParams{
		TenantID: env.tenantID, Name: "finance",
	})
	if err != nil {
		t.Fatalf("peran tak tersimpan: %v", err)
	}
	if got.DataScope != "own" {
		t.Errorf("data_scope = %q, want own", got.DataScope)
	}
	if got.IsSystem {
		t.Error("peran buatan user tak boleh is_system")
	}
	if rows, _ := env.q.ListBusinessRolePermissionsByTenant(t.Context(), env.tenantID); anyRole(rows, "finance") {
		t.Error("peran baru harus bermatriks kosong")
	}
	env.assertAudited(t, "crm.role.create")
}

// TestRoles_CreateRejectsInvalid: input yang melanggar validasi backend ditolak
// → redirect err + tak menyentuh DB.
func TestRoles_CreateRejectsInvalid(t *testing.T) {
	cases := []struct {
		name    string
		form    func() url.Values
		roleKey string
		wantErr string
	}{
		{"nama diawali angka", func() url.Values {
			f := roleFormValues("Keuangan", "all")
			f.Set("name", "1bad")
			return f
		}, "1bad", "err=role_name"},
		{"nama tampilan kosong", func() url.Values {
			f := roleFormValues("", "all")
			f.Set("name", "finance")
			return f
		}, "finance", "err=role_display"},
		{"cakupan asing", func() url.Values {
			f := roleFormValues("Keuangan", "planet")
			f.Set("name", "finance2")
			return f
		}, "finance2", "err=role_scope"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, uid := setupRoles(t)
			req := rolesReq(http.MethodPost, "/w/test/roles", c.form(), "")
			rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleCreate)
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, c.wantErr) {
				t.Errorf("harus %q, got %q", c.wantErr, loc)
			}
			if env.hasRole(t, c.roleKey) {
				t.Errorf("input invalid tak boleh menyimpan peran %q", c.roleKey)
			}
		})
	}
}

// TestRoles_CreateDuplicate: nama peran UNIQUE per tenant — tabrakan dikenali
// sebagai galat spesifik (role_exists), bukan "internal error".
func TestRoles_CreateDuplicate(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "all", false)

	form := roleFormValues("Keuangan Lain", "own")
	form.Set("name", "finance")
	req := rolesReq(http.MethodPost, "/w/test/roles", form, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleCreate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=role_exists") {
		t.Errorf("nama bentrok harus err=role_exists, got %q", loc)
	}
}

// --- update + reload -------------------------------------------------------
