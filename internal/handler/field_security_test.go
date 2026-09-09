package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/fls"
)

// field_security_test.go — section "Field Security" di halaman /roles (BL-107,
// ADR 0012 opsi B), sisi HANDLER (gate + persist + reload cache + audit). Kontrak
// murni cache diuji di internal/fls; di sini yang dijaga:
//
//   - Gerbang crm:field_security (sumbu BISNIS): hanya admin (lewat glob crm:*)
//     boleh membuka /roles (canManageRoles) & menyimpan matriks; peran lain &
//     tenant-role setinggi apa pun tanpa izin itu ditolak — sejak opsi B section
//     ini menumpang gerbang /roles yang keduanya admin-only & selalu selaras.
//   - State awal matriks = kebijakan yang BERLAKU (WYSIWYG): tenant belum
//     dikonfigurasi merender default terkunci (Sales+Admin lihat, Sales sunting).
//   - POST replace-all: persist per peran, coerce edit⇒view, peran liar diabaikan
//     (closed set dari DB), cache berlaku seketika (ReloadTenant), ter-audit; PRG
//     balik ke /roles dengan kode `fsec_saved` (bukan `saved` milik edit peran).
//   - Read-only (arsip) menolak simpan walau admin.
//
// Pakai setupRoles: handler membaca ListBusinessRoles dari DB, jadi peran bawaan
// wajib tertanam di tabel — bukan sekadar dimuat ke enforcer (setupAccounts).

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

// --- gerbang crm:field_security -------------------------------------------

// TestFieldSecurity_GateRead: sejak opsi B matriks Field Security = section di
// /roles, jadi gerbangnya = gerbang RolesPage (canManageRoles). Hanya admin boleh
// membuka; peran CRM lain & "" ditolak 403 dengan penjelasan (menyebut admin CRM).
func TestFieldSecurity_GateRead(t *testing.T) {
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
			if c.allow {
				if rec.Code != http.StatusOK {
					t.Errorf("role %q harus lolos gate, got %d\n%s", c.role, rec.Code, rec.Body.String())
				}
				return
			}
			if rec.Code != http.StatusForbidden {
				t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
			}
			if !strings.Contains(rec.Body.String(), "admin CRM") {
				t.Errorf("penolakan harus menyebut admin CRM")
			}
		})
	}
}

// TestFieldSecurity_GateTegakLurusPlatform: owner/super_admin TANPA peran CRM
// tetap DITOLAK — sumbu F4 tak mewarisi otoritas platform (gerbang RolesPage).
func TestFieldSecurity_GateTegakLurusPlatform(t *testing.T) {
	env, uid := setupRoles(t)
	for _, tenantRole := range []string{"owner", "admin", "super_admin"} {
		req := rolesReq(http.MethodGet, "/w/test/roles", nil, "")
		rec := env.runAccount(uid, tenantRole, "", req, env.h.RolesPage)
		if rec.Code != http.StatusForbidden {
			t.Errorf("tenant role %q tanpa peran CRM harus 403, got %d", tenantRole, rec.Code)
		}
	}
}

// TestFieldSecurity_GETRendersDefault: tenant belum dikonfigurasi → section di
// /roles merender kebijakan default (WYSIWYG). Sales+Admin lihat tercentang, Sales
// sunting tercentang, Admin sunting TIDAK (lihat-saja), Manager lihat TIDAK.
func TestFieldSecurity_GETRendersDefault(t *testing.T) {
	env, uid := setupRoles(t)
	req := rolesReq(http.MethodGet, "/w/test/roles", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RolesPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin GET harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	checked := func(field string) bool {
		return strings.Contains(body, `name="`+field+`" value="1" checked`)
	}
	present := func(field string) bool {
		return strings.Contains(body, `name="`+field+`" value="1"`)
	}
	// Default: Sales lihat+sunting, Admin lihat (read-only), Manager tak lihat.
	if !checked("view.sales") || !checked("edit.sales") {
		t.Error("default: Sales lihat+sunting harus tercentang")
	}
	if !checked("view.admin") {
		t.Error("default: Admin lihat harus tercentang")
	}
	if checked("edit.admin") {
		t.Error("default: Admin lihat-saja — sunting tak boleh tercentang")
	}
	if checked("view.manager") {
		t.Error("default: Manager tak lihat — view tak boleh tercentang")
	}
	// Barisnya tetap ada (kotak hadir, sekadar tak tercentang) — bukti WYSIWYG
	// bukan baris hilang.
	if !present("view.manager") {
		t.Error("baris Manager harus dirender (kotak hadir walau tak tercentang)")
	}
}

// --- POST: gate ------------------------------------------------------------

// TestFieldSecurity_UpdateGate: POST hanya admin; peran lain ditolak
// (err=forbidden) & tak menyimpan baris apa pun.
func TestFieldSecurity_UpdateGate(t *testing.T) {
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
			form := url.Values{"view.manager": {"1"}}
			req := rolesReq(http.MethodPost, "/w/test/field-security", form, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.WorkspaceFieldSecurityUpdate)

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

// --- POST: persist + reload + audit ---------------------------------------

// TestFieldSecurity_UpdatePersistsAndReloads: admin memberi Manager lihat+sunting
// → 303 ok=fsec_saved, baris tersimpan, audit tercatat, dan cache berlaku SEKETIKA
// (fls.CanViewPhone Manager jadi true tanpa restart).
func TestFieldSecurity_UpdatePersistsAndReloads(t *testing.T) {
	env, uid := setupRoles(t)

	// Prakondisi: default → Manager belum boleh lihat.
	if fls.CanViewPhone(env.tenantID, "manager") {
		t.Fatal("prakondisi: Manager default tak boleh lihat")
	}

	form := url.Values{"view.manager": {"1"}, "edit.manager": {"1"}}
	req := rolesReq(http.MethodPost, "/w/test/field-security", form, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.WorkspaceFieldSecurityUpdate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=fsec_saved") {
		t.Fatalf("harus ok=fsec_saved, got %q (status %d)", loc, rec.Code)
	}

	got := env.fsecPolicies(t)
	if p := got["manager"]; !p.view || !p.edit {
		t.Errorf("Manager harus tersimpan lihat+sunting, got %+v", p)
	}
	// Replace-all menulis baris untuk SETIAP peran (termasuk yang all-false), agar
	// "terkonfigurasi" beda dari "default". Jadi Sales pun kini bermeja baris.
	if _, ok := got["sales"]; !ok {
		t.Error("replace-all harus menulis baris untuk semua peran (termasuk Sales all-false)")
	}

	env.assertAudited(t, "workspace.field_security")

	// Cache berlaku seketika.
	if !fls.CanViewPhone(env.tenantID, "manager") {
		t.Error("setelah simpan: Manager harus boleh lihat (cache reload)")
	}
	if !fls.CanEditPhone(env.tenantID, "manager") {
		t.Error("setelah simpan: Manager harus boleh sunting (cache reload)")
	}
	// Fail-closed: tenant kini TERKONFIGURASI → Sales (tak dicentang) jatuh ke
	// false, bukan default lama Sales+Admin.
	if fls.CanViewPhone(env.tenantID, "sales") {
		t.Error("tenant terkonfigurasi: Sales tanpa centang harus fail-closed (view=false)")
	}
}

// TestFieldSecurity_CoerceEditImpliesView: mencentang "sunting" tanpa "lihat"
// dinaikkan jadi lihat+sunting (nomor tak terlihat mustahil disunting; cermin
// CHECK DB edit⇒view). Tersimpan view=true walau form hanya kirim edit.
func TestFieldSecurity_CoerceEditImpliesView(t *testing.T) {
	env, uid := setupRoles(t)
	form := url.Values{"edit.manager": {"1"}} // sunting saja, tanpa view.manager
	req := rolesReq(http.MethodPost, "/w/test/field-security", form, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.WorkspaceFieldSecurityUpdate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=fsec_saved") {
		t.Fatalf("harus ok=fsec_saved, got %q", loc)
	}
	if p := env.fsecPolicies(t)["manager"]; !p.view || !p.edit {
		t.Errorf("edit tanpa view harus di-coerce jadi lihat+sunting, got %+v", p)
	}
}

// TestFieldSecurity_WildRoleIgnored: field peran ASING (nama dikarang / peran yang
// tak ada di DB) diabaikan diam-diam — set peran ditentukan dari DB (closed set),
// bukan dari nama field POST.
func TestFieldSecurity_WildRoleIgnored(t *testing.T) {
	env, uid := setupRoles(t)
	form := url.Values{
		"view.ghost":     {"1"}, // peran karangan
		"edit.ghost":     {"1"},
		"view.manager":   {"1"}, // peran sah, sebagai kontrol
		"view.__proto__": {"1"},
	}
	req := rolesReq(http.MethodPost, "/w/test/field-security", form, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.WorkspaceFieldSecurityUpdate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=fsec_saved") {
		t.Fatalf("harus ok=fsec_saved, got %q", loc)
	}
	got := env.fsecPolicies(t)
	if _, ok := got["ghost"]; ok {
		t.Error("peran karangan 'ghost' tak boleh tersimpan (closed set)")
	}
	if _, ok := got["__proto__"]; ok {
		t.Error("field peran asing tak boleh tersimpan")
	}
	if p := got["manager"]; !p.view {
		t.Error("peran sah 'manager' tetap harus tersimpan")
	}
}

// TestFieldSecurity_ReadOnlyBlocksPost: workspace arsip (read-only) menolak simpan
// walau admin — perubahan kebijakan keamanan tak boleh lolos di workspace beku.
func TestFieldSecurity_ReadOnlyBlocksPost(t *testing.T) {
	env, uid := setupRoles(t)
	form := url.Values{"view.manager": {"1"}}
	req := rolesReq(http.MethodPost, "/w/test/field-security", form, "")
	rec := env.runAccount(uid, "owner", "admin", req, func(w http.ResponseWriter, r *http.Request) {
		// Simulasikan konteks arsip yang biasanya disetel Scope.
		env.h.WorkspaceFieldSecurityUpdate(w, r.WithContext(withTenantStatus(r.Context(), TenantArchived)))
	})
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=forbidden") {
		t.Errorf("read-only harus ditolak err=forbidden, got %q", loc)
	}
	if len(env.fsecPolicies(t)) != 0 {
		t.Error("read-only tak boleh menyimpan kebijakan")
	}
}
