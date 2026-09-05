package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"go_starter/internal/authz"
	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/go-chi/chi/v5"
)

// accounts_test.go — hub Desa (Account) di sisi handler + helper bersama semua
// file test accounts_*. Yang dijaga di SINI: sumbu F2 (gerbang read/write) +
// keyset, plus penugasan CSM. CRUD happy-path di accounts_crud_test.go; F3
// (ownership) & F4 (field-level) di accounts_fls_test.go. Tiga sumbu izin bekerja
// tegak lurus (sistem-dan-role §3):
//
//   - F2 (Casbin bisnis): GET butuh "read", POST butuh "write". Support lolos
//     read tapi bukan write; business_role "" ditolak keduanya; super_admin/owner
//     TANPA peran CRM tak otomatis masuk (tak ada short-circuit root).
//   - F3 (ownership, layer app): Sales melihat account_owner-nya; CSM melihat
//     assigned/backup; Admin/Manager semua; di luar cakupan → 404 (bukan 403).
//   - F4 (field-level): nomor HP kontak utuh untuk Sales & Admin; role lain tak
//     pernah menerima nomor asli & tak bisa menimpanya.
//   - Keyset: pageSize+1 → tombol "Berikutnya" hanya saat memang ada halaman lanjut.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS
// sesungguhnya diuji terpisah di rls_test.go.

// --- setup & helper --------------------------------------------------------

// setupAccounts menyiapkan env + KEDUA enforcer nyata (dari policy embed).
// Enforcer bisnis (benf) dibutuhkan CanBusiness — tanpanya gate menolak semua &
// test membuktikan hal yang salah. Enforcer tenant (enf) dibutuhkan
// renderWorkspaceShell (navFor → canManageMembers dll).
func setupAccounts(t *testing.T) (*testEnv, int64) {
	t.Helper()
	env, uid := setupTest(t)
	be, err := authz.NewBusinessEmpty()
	if err != nil {
		t.Fatalf("authz.NewBusinessEmpty: %v", err)
	}
	// Subject bisnis di-fold `t<id>:<role>` sejak 00007 → muat matriks default
	// untuk tenant test, bukan subject telanjang (yang tak akan match CanBusiness).
	var perms []authz.BusinessPerm
	for _, r := range authz.DefaultBusinessRoles() {
		for _, p := range r.Perms {
			perms = append(perms, authz.BusinessPerm{
				TenantID: env.tenantID, Role: r.Name, Obj: p.Obj, Act: p.Act,
			})
		}
	}
	if err := authz.LoadBusiness(be, perms); err != nil {
		t.Fatalf("authz.LoadBusiness: %v", err)
	}
	authz.InitBusiness(be)
	e, err := authz.New(authz.Model, authz.Policy)
	if err != nil {
		t.Fatalf("authz.New (tenant): %v", err)
	}
	authz.Init(e)
	return env, uid
}

// accountsReq membangun request dengan chi param slug (+ {id} bila diberi) dan
// body form terkode. id "" = tanpa param.
func accountsReq(method, target string, form url.Values, id string) *http.Request {
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
	rctx.URLParams.Add(slugURLParam, "test") // slug dibaca handler (wsPath/wsRedirect)
	if id != "" {
		rctx.URLParams.Add("id", id)
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
}

// runAccount menjalankan fn dalam session (uid, tenantRole, businessRole) +
// Queries ber-scope (sim. Scope). Mengembalikan recorder utuh (status + body).
func (e *testEnv) runAccount(
	uid int64, tenantRole, businessRole string, req *http.Request, fn http.HandlerFunc,
) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session.SetIdentity(ctx, uid, "test@local", tenantRole, false,
			e.tenantID, "Test", "test", "")
		session.SetBusinessRole(ctx, businessRole)
		// F3: cakupan data disetel dari nama peran seperti RefreshIdentity
		// menurunkannya dari kolom data_scope. Tanpa ini, filter kepemilikan
		// membaca "" → ScopeNone → nol baris, dan tiap test peran melihat-desa gagal.
		session.SetBusinessDataScope(ctx, authz.DefaultDataScope(businessRole))
		fn(w, r.WithContext(withQueries(ctx, e.q)))
	})).ServeHTTP(rec, req)
	return rec
}

// runAccountScope = runAccount dengan data_scope EKSPLISIT alih-alih diturunkan
// dari nama role. Diperlukan untuk peran CUSTOM (di luar DefaultBusinessRoles)
// yang DefaultDataScope-nya jatuh ke None — mis. uji BL-58 (peran custom
// ber-cakupan 'all' + kapabilitas crm:subscriptions/arr).
func (e *testEnv) runAccountScope(
	uid int64, tenantRole, businessRole, dataScope string, req *http.Request, fn http.HandlerFunc,
) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	e.sm.LoadAndSave(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		session.SetIdentity(ctx, uid, "test@local", tenantRole, false,
			e.tenantID, "Test", "test", "")
		session.SetBusinessRole(ctx, businessRole)
		session.SetBusinessDataScope(ctx, dataScope)
		fn(w, r.WithContext(withQueries(ctx, e.q)))
	})).ServeHTTP(rec, req)
	return rec
}

// loadBusinessRolesWith menyegarkan enforcer bisnis tenant test dengan peran
// bawaan (DefaultBusinessRoles) DITAMBAH izin custom `extra` — untuk menguji
// grant kapabilitas pada peran non-bawaan (BL-58) tanpa menyentuh matriks
// bawaan. TenantID di-set di sini agar pemanggil cukup memberi Role/Obj/Act.
func (e *testEnv) loadBusinessRolesWith(t *testing.T, extra ...authz.BusinessPerm) {
	t.Helper()
	var perms []authz.BusinessPerm
	for _, r := range authz.DefaultBusinessRoles() {
		for _, p := range r.Perms {
			perms = append(perms, authz.BusinessPerm{
				TenantID: e.tenantID, Role: r.Name, Obj: p.Obj, Act: p.Act,
			})
		}
	}
	for _, x := range extra {
		x.TenantID = e.tenantID
		perms = append(perms, x)
	}
	if err := authz.ReloadBusinessTenant(e.tenantID, perms); err != nil {
		t.Fatalf("ReloadBusinessTenant: %v", err)
	}
}

// seedAccount menaruh satu desa langsung lewat pool (bypass handler) dengan
// kolom kepemilikan tertentu — untuk menguji F3 tanpa merangkai create.
func (e *testEnv) seedAccount(t *testing.T, name string, owner, assignedCSM, backupCSM *int64) db.Account {
	t.Helper()
	return e.seedAccountFull(t, name, owner, assignedCSM, backupCSM, nil)
}

// seedAccountWithPhone = seedAccount + nomor HP kontak (untuk F4).
func (e *testEnv) seedAccountWithPhone(t *testing.T, name string, owner int64, phone string) db.Account {
	t.Helper()
	return e.seedAccountFull(t, name, &owner, nil, nil, &phone)
}

func (e *testEnv) seedAccountFull(
	t *testing.T, name string, owner, assignedCSM, backupCSM *int64, phone *string,
) db.Account {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityAccount)
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	a, err := e.q.CreateAccount(t.Context(), db.CreateAccountParams{
		TenantID:     e.tenantID,
		EntityCode:   &code,
		VillageName:  name,
		AccountType:  "prospect",
		AccountOwner: owner,
		AssignedCsm:  assignedCSM,
		BackupCsm:    backupCSM,
		ContactPhone: phone,
	})
	if err != nil {
		t.Fatalf("seed account %s: %v", name, err)
	}
	return a
}

// allAccounts mendaftar SEMUA desa hidup (ScopeAll) langsung dari pool — untuk
// membuktikan sebuah aksi menyimpan / tak menyimpan baris.
func (e *testEnv) allAccounts(t *testing.T) []db.Account {
	t.Helper()
	at, id := firstPageCursor()
	rows, err := e.q.ListAccounts(t.Context(), db.ListAccountsParams{
		CursorCreatedAt: at, CursorID: id, ScopeAll: true, PageSize: 100,
	})
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	return rows
}

// assertAudited memastikan minimal satu audit dengan action tertentu tercatat.
func (e *testEnv) assertAudited(t *testing.T, action string) {
	t.Helper()
	logs, err := e.q.ListAuditLogs(t.Context(), 50)
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	for _, l := range logs {
		if l.Action == action {
			return
		}
	}
	t.Errorf("audit %q tak tercatat", action)
}

// accountFormValues merakit form values minimal yang valid untuk create/update.
func accountFormValues(name, accountType string) url.Values {
	return url.Values{
		"village_name": {name},
		"account_type": {accountType},
	}
}

// withField menyetel satu field lalu mengembalikan form (untuk tabel kasus).
func withField(v url.Values, key, val string) url.Values {
	v.Set(key, val)
	return v
}

var afterRe = regexp.MustCompile(`accounts\?after=([0-9]+_[0-9]+)`)

// extractAfter menarik cursor ?after= dari href "Berikutnya" di body.
func extractAfter(t *testing.T, body string) string {
	t.Helper()
	m := afterRe.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("tautan ?after= tak ditemukan di body")
	}
	return m[1]
}

// --- F2: gerbang read/write ------------------------------------------------

// TestAccounts_GateRead: siapa boleh MEMBUKA daftar (act read). write mencakup
// read (matcher) → admin/manager/sales/csm/support lolos; "" ditolak 403.
func TestAccounts_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"sales", true},
		{"csm", true},
		{"support", true}, // lolos gate read; F3 ScopeNone → nol baris (diuji terpisah)
		{"", false},       // tanpa peran CRM → 403 (deny-default, tak ada root)
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/accounts", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.AccountsList)
			if c.allow && rec.Code != http.StatusOK {
				t.Errorf("role %q harus lolos gate read, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if !strings.Contains(rec.Body.String(), "peran CRM") {
					t.Errorf("penolakan harus menjelaskan butuh peran CRM")
				}
			}
		})
	}
}

// TestAccounts_GateReadTegakLurusPlatform: owner/super_admin TANPA business_role
// tetap DITOLAK — sumbu bisnis tak mewarisi otoritas platform (§3). Tenant role
// setinggi apa pun tak membuka modul Desa tanpa peran CRM.
func TestAccounts_GateReadTegakLurusPlatform(t *testing.T) {
	env, uid := setupAccounts(t)
	for _, tenantRole := range []string{"owner", "admin", "super_admin"} {
		req := accountsReq(http.MethodGet, "/w/test/accounts", nil, "")
		rec := env.runAccount(uid, tenantRole, "", req, env.h.AccountsList)
		if rec.Code != http.StatusForbidden {
			t.Errorf("tenant role %q tanpa peran CRM harus 403, got %d", tenantRole, rec.Code)
		}
	}
}

// TestAccounts_GateWrite: POST create butuh act write. support (read saja) &
// "" ditolak; admin/sales lolos. Ditolak → 403 (renderAccountsForbidden), tak
// pernah menyentuh DB.
func TestAccounts_GateWrite(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"sales", true},
		{"support", false}, // read-only di sumbu bisnis
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			form := accountFormValues("Desa Uji", "prospect")
			// Kecamatan kini WAJIB di create (village_code diturunkan darinya) —
			// tanpa ini jalur "allow" ditolak district_required sebelum menyentuh gate.
			form.Set("district_id", strconv.FormatInt(firstDistrictID(t, env), 10))
			req := accountsReq(http.MethodPost, "/w/test/accounts", form, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.AccountCreate)

			rows := env.allAccounts(t)
			if c.allow {
				if rec.Code != http.StatusSeeOther {
					t.Errorf("role %q harus create (303), got %d\n%s", c.role, rec.Code, rec.Body.String())
				}
				if len(rows) != 1 {
					t.Errorf("role %q create harus menyimpan 1 baris, ada %d", c.role, len(rows))
				}
			} else {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if len(rows) != 0 {
					t.Errorf("role %q ditolak tak boleh menyimpan apa pun, ada %d baris", c.role, len(rows))
				}
			}
		})
	}
}

// --- Assign CSM ------------------------------------------------------------

// TestAccounts_AssignSuccess: menetapkan CSM anggota workspace → tersimpan,
// ok=assigned, audit tercatat.
func TestAccounts_AssignSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	csm := env.seedMember(t, "csm@local", "member", 0).ID
	a := env.seedAccount(t, "Desa Tugas", &uid, nil, nil)

	form := url.Values{"assigned_csm": {itoa(csm)}}
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/assign", form, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountAssign)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=assigned") {
		t.Errorf("harus ok=assigned, got %q (status %d)", loc, rec.Code)
	}
	got, _ := env.q.GetAccount(t.Context(), a.ID)
	if got.AssignedCsm == nil || *got.AssignedCsm != csm {
		t.Errorf("assigned_csm tak tersimpan, got %v", got.AssignedCsm)
	}
	env.assertAudited(t, "account.assign")
}

// TestAccounts_AssignNonMemberDitolak: kandidat CSM yang BUKAN anggota workspace
// ditolak (err=csm) — memasang non-anggota melahirkan baris yatim tak terlihat F3.
func TestAccounts_AssignNonMemberDitolak(t *testing.T) {
	env, uid := setupAccounts(t)
	outsider := env.seedUserOnly(t, "outsider@local").ID // user global tanpa membership
	a := env.seedAccount(t, "Desa Tugas", &uid, nil, nil)

	form := url.Values{"assigned_csm": {itoa(outsider)}}
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/assign", form, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountAssign)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=csm") {
		t.Errorf("non-anggota harus ditolak err=csm, got %q", loc)
	}
	got, _ := env.q.GetAccount(t.Context(), a.ID)
	if got.AssignedCsm != nil {
		t.Error("penugasan non-anggota tak boleh tersimpan")
	}
}

// --- Keyset pagination -----------------------------------------------------

// TestAccounts_KeysetPagination: dengan pageSize+1 desa, halaman pertama
// menampilkan tautan "Berikutnya"; mengikuti cursor menampilkan sisanya +
// "Ujung daftar." (baris ke-(pageSize+1) tetap terjangkau, tak hilang senyap).
func TestAccounts_KeysetPagination(t *testing.T) {
	env, uid := setupAccounts(t)
	total := pageSize + 1
	for i := 0; i < total; i++ {
		env.seedAccount(t, "Desa "+itoa(int64(i)), &uid, nil, nil)
	}

	// Halaman 1: ada tautan Berikutnya, belum ujung daftar.
	req := accountsReq(http.MethodGet, "/w/test/accounts", nil, "")
	body := env.runAccount(uid, "owner", "admin", req, env.h.AccountsList).Body.String()
	if !strings.Contains(body, "Berikutnya") {
		t.Fatal("halaman pertama dengan >pageSize desa harus punya tautan Berikutnya")
	}
	if strings.Contains(body, "Ujung daftar.") {
		t.Error("halaman pertama (masih ada lanjutan) tak boleh menyatakan ujung daftar")
	}

	// Ikuti cursor ke halaman 2.
	after := extractAfter(t, body)
	req2 := accountsReq(http.MethodGet, "/w/test/accounts?after="+after, nil, "")
	body2 := env.runAccount(uid, "owner", "admin", req2, env.h.AccountsList).Body.String()
	if !strings.Contains(body2, "Ujung daftar.") {
		t.Error("halaman terakhir harus menyatakan ujung daftar")
	}
	if strings.Contains(body2, "Berikutnya") {
		t.Error("halaman terakhir tak boleh menawarkan Berikutnya (berujung kosong)")
	}
}
