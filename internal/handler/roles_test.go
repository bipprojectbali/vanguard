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

// TestRoles_UpdateMatrixAndReload: sunting matriks peran langsung mengubah izin
// (bukti reload enforcer set-penuh), tersimpan di DB, cakupan data ikut berubah,
// audit tercatat. write MENCAKUP read; approve independen.
func TestRoles_UpdateMatrixAndReload(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "own", false)

	// Sebelum: peran finance belum punya izin apa pun di enforcer.
	if env.canBiz(uid, "finance", "crm:accounts", "read") {
		t.Fatal("prasyarat: finance belum boleh apa-apa sebelum disunting")
	}

	form := roleFormValues("Keuangan", "all")
	form.Set("level.crm:accounts", "write")
	form.Set("approve.crm:deals", "1")
	req := rolesReq(http.MethodPost, "/w/test/roles/finance", form, "finance")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Fatalf("harus ok=saved, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	// DB: matriks & cakupan tersimpan.
	if !env.hasPerm(t, "finance", "crm:accounts", "write") {
		t.Error("crm:accounts write harus tersimpan")
	}
	if !env.hasPerm(t, "finance", "crm:deals", "approve") {
		t.Error("crm:deals approve harus tersimpan")
	}
	got, _ := env.q.GetBusinessRole(t.Context(), db.GetBusinessRoleParams{
		TenantID: env.tenantID, Name: "finance",
	})
	if got.DataScope != "all" {
		t.Errorf("data_scope harus berubah ke all, got %q", got.DataScope)
	}
	// Enforcer: reload seketika. write ⊇ read.
	if !env.canBiz(uid, "finance", "crm:accounts", "write") {
		t.Error("reload gagal — finance harus boleh write crm:accounts")
	}
	if !env.canBiz(uid, "finance", "crm:accounts", "read") {
		t.Error("write harus mencakup read (matcher bisnis)")
	}
	if !env.canBiz(uid, "finance", "crm:deals", "approve") {
		t.Error("approve crm:deals harus berlaku setelah reload")
	}
	// Peran default lain TAK terhapus oleh reload set-penuh.
	if !env.canBiz(uid, "sales", "crm:accounts", "read") {
		t.Error("reload set-penuh tak boleh menghapus izin peran lain (sales)")
	}
	env.assertAudited(t, "crm.role.update")
}

// TestRoles_UpdateSystemRejected: peran sistem (admin) kebal sunting → err,
// nama tampilannya tak berubah.
func TestRoles_UpdateSystemRejected(t *testing.T) {
	env, uid := setupRoles(t)
	form := roleFormValues("Diretas", "none")
	req := rolesReq(http.MethodPost, "/w/test/roles/admin", form, "admin")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=role_system") {
		t.Errorf("sunting peran sistem harus err=role_system, got %q", loc)
	}
	got, _ := env.q.GetBusinessRole(t.Context(), db.GetBusinessRoleParams{
		TenantID: env.tenantID, Name: "admin",
	})
	if got.DisplayName == "Diretas" {
		t.Error("peran sistem tak boleh berubah")
	}
}

// --- delete ----------------------------------------------------------------

// TestRoles_DeleteSuccess: hapus peran → 303 ok=deleted, peran hilang, anggota
// pemegangnya di-unassign (business_role NULL), audit tercatat.
func TestRoles_DeleteSuccess(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "own", false)
	member := env.seedMember(t, "fin@local", "member", 0).ID
	fin := "finance"
	if err := env.q.UpdateMemberBusinessRole(t.Context(), db.UpdateMemberBusinessRoleParams{
		UserID: member, TenantID: env.tenantID, BusinessRole: &fin,
	}); err != nil {
		t.Fatalf("assign role: %v", err)
	}

	req := rolesReq(http.MethodPost, "/w/test/roles/finance/delete", url.Values{}, "finance")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleDelete)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=deleted") {
		t.Errorf("harus ok=deleted, got %q (status %d)", loc, rec.Code)
	}
	if env.hasRole(t, "finance") {
		t.Error("peran harus terhapus")
	}
	m, err := env.q.GetMembership(t.Context(), db.GetMembershipParams{
		UserID: member, TenantID: env.tenantID,
	})
	if err != nil {
		t.Fatalf("get membership: %v", err)
	}
	if m.BusinessRole != nil {
		t.Errorf("anggota pemegang peran terhapus harus di-unassign, got %v", *m.BusinessRole)
	}
	env.assertAudited(t, "crm.role.delete")
}

// TestRoles_DeleteSystemRejected: peran sistem (admin) kebal hapus → err, peran
// tetap ada.
func TestRoles_DeleteSystemRejected(t *testing.T) {
	env, uid := setupRoles(t)
	req := rolesReq(http.MethodPost, "/w/test/roles/admin/delete", url.Values{}, "admin")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleDelete)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=role_system") {
		t.Errorf("hapus peran sistem harus err=role_system, got %q", loc)
	}
	if !env.hasRole(t, "admin") {
		t.Error("peran sistem tak boleh terhapus")
	}
}

// anyRole = true bila ada baris izin milik role di daftar.
func anyRole(rows []db.ListBusinessRolePermissionsByTenantRow, role string) bool {
	for _, r := range rows {
		if r.Role == role {
			return true
		}
	}
	return false
}
