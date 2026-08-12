package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"go_starter/internal/authz"
	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/go-chi/chi/v5"
)

// accounts_test.go — hub Desa (Account) di sisi handler. Yang dijaga adalah tiga
// sumbu izin yang bekerja tegak lurus (sistem-dan-role §3) plus keyset:
//
//   - F2 (Casbin bisnis): GET butuh "read", POST butuh "write". Support lolos
//     read tapi bukan write; business_role "" ditolak keduanya; super_admin/owner
//     TANPA peran CRM tak otomatis masuk (tak ada short-circuit root).
//   - F3 (ownership, layer app): Sales melihat account_owner-nya; CSM melihat
//     assigned/backup; Admin/Manager semua; di luar cakupan → 404 (bukan 403).
//   - F4 (field-level): nomor HP kontak utuh HANYA Sales; editor non-Sales tak
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

// --- CRUD happy path -------------------------------------------------------

// TestAccounts_CreateSuccess: create sebagai sales → 303 ke detail dengan
// ok=created, baris tersimpan dengan pembuat sebagai owner, entity_code terisi,
// audit tercatat.
func TestAccounts_CreateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	form := accountFormValues("Desa Sukamaju", "prospect")
	form.Set("village_code", "3201012001")
	form.Set("province", "Jawa Barat")
	form.Set("contact_phone", "0812-1111-2222")
	req := accountsReq(http.MethodPost, "/w/test/accounts", form, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountCreate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Errorf("redirect harus ok=created, got %q", loc)
	}
	rows := env.allAccounts(t)
	if len(rows) != 1 {
		t.Fatalf("harus 1 baris, ada %d", len(rows))
	}
	a := rows[0]
	if a.VillageName != "Desa Sukamaju" {
		t.Errorf("nama = %q", a.VillageName)
	}
	if a.AccountOwner == nil || *a.AccountOwner != uid {
		t.Errorf("pembuat harus jadi owner awal (F3), got %v", a.AccountOwner)
	}
	if a.EntityCode == nil || *a.EntityCode == "" {
		t.Error("entity_code harus dialokasikan saat create")
	}
	env.assertAudited(t, "account.create")
}

// TestAccounts_CreateRejectsInvalid: input yang melanggar validasi backend
// (bukan cuma atribut form) ditolak → redirect err + tak menyentuh DB.
func TestAccounts_CreateRejectsInvalid(t *testing.T) {
	cases := []struct {
		name    string
		form    url.Values
		wantErr string
	}{
		{"nama kosong", accountFormValues("", "prospect"), "err=village_name"},
		{"tipe asing", accountFormValues("Desa X", "bukan_tipe"), "err=account_type"},
		{"penduduk negatif", withField(accountFormValues("Desa X", "prospect"), "population", "-5"), "err=number"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, uid := setupAccounts(t)
			req := accountsReq(http.MethodPost, "/w/test/accounts", c.form, "")
			rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountCreate)
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, c.wantErr) {
				t.Errorf("harus %q, got %q", c.wantErr, loc)
			}
			if rows := env.allAccounts(t); len(rows) != 0 {
				t.Errorf("input invalid tak boleh menyimpan, ada %d baris", len(rows))
			}
		})
	}
}

// TestAccounts_VillageCodeDuplicate: village_code UNIQUE per tenant — tabrakan
// dikenali sebagai galat spesifik (village_code_dup), bukan "internal error".
func TestAccounts_VillageCodeDuplicate(t *testing.T) {
	env, uid := setupAccounts(t)
	first := accountFormValues("Desa Satu", "prospect")
	first.Set("village_code", "3201012001")
	req1 := accountsReq(http.MethodPost, "/w/test/accounts", first, "")
	if rec := env.runAccount(uid, "owner", "sales", req1, env.h.AccountCreate); rec.Code != http.StatusSeeOther {
		t.Fatalf("create pertama gagal: %d\n%s", rec.Code, rec.Body.String())
	}

	dup := accountFormValues("Desa Dua", "prospect")
	dup.Set("village_code", "3201012001") // sama
	req2 := accountsReq(http.MethodPost, "/w/test/accounts", dup, "")
	rec := env.runAccount(uid, "owner", "sales", req2, env.h.AccountCreate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=village_code_dup") {
		t.Errorf("tabrakan village_code harus err=village_code_dup, got %q", loc)
	}
}

// TestAccounts_UpdateSuccess: update sebagai owner (business admin) → tersimpan,
// redirect ok=saved, audit tercatat.
func TestAccounts_UpdateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Lama", &uid, nil, nil)

	form := accountFormValues("Desa Baru", "customer")
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID), form, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Errorf("harus ok=saved, got %q (status %d)", loc, rec.Code)
	}
	got, _ := env.q.GetAccount(t.Context(), a.ID)
	if got.VillageName != "Desa Baru" || got.AccountType != "customer" {
		t.Errorf("update tak tersimpan: %q / %q", got.VillageName, got.AccountType)
	}
	env.assertAudited(t, "account.update")
}

// TestAccounts_SoftDelete: delete → baris hilang dari GetAccount (deleted_at),
// redirect ke daftar dengan ok=deleted, audit tercatat.
func TestAccounts_SoftDelete(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Hapus", &uid, nil, nil)

	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/delete", url.Values{}, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountDelete)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=deleted") {
		t.Errorf("harus ok=deleted, got %q (status %d)", loc, rec.Code)
	}
	if _, err := env.q.GetAccount(t.Context(), a.ID); err == nil {
		t.Error("baris ter-soft-delete tak boleh lagi terbaca GetAccount")
	}
	env.assertAudited(t, "account.delete")
}

// --- F3: ownership ---------------------------------------------------------

// TestAccounts_F3_SalesLihatMiliknya: Sales melihat HANYA desa yang ia miliki
// (account_owner). Desa milik sales lain → tak tampil di daftar & detailnya 404.
func TestAccounts_F3_SalesLihatMiliknya(t *testing.T) {
	env, salesA := setupAccounts(t)
	salesB := env.seedMember(t, "salesb@local", "member", 0).ID

	mine := env.seedAccount(t, "Desa Milik A", &salesA, nil, nil)
	theirs := env.seedAccount(t, "Desa Milik B", &salesB, nil, nil)

	// Daftar sebagai salesA: hanya "Desa Milik A".
	req := accountsReq(http.MethodGet, "/w/test/accounts", nil, "")
	body := env.runAccount(salesA, "member", "sales", req, env.h.AccountsList).Body.String()
	if !strings.Contains(body, "Desa Milik A") {
		t.Error("sales A harus melihat desanya sendiri")
	}
	if strings.Contains(body, "Desa Milik B") {
		t.Error("sales A TAK boleh melihat desa milik sales B (F3 bocor)")
	}

	// Detail desa milik B → 404 (menyangkal keberadaan, bukan 403).
	dReq := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(theirs.ID), nil, itoa(theirs.ID))
	if rec := env.runAccount(salesA, "member", "sales", dReq, env.h.AccountDetail); rec.Code != http.StatusNotFound {
		t.Errorf("detail desa di luar cakupan harus 404, got %d", rec.Code)
	}

	// Detail desa sendiri → 200.
	okReq := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(mine.ID), nil, itoa(mine.ID))
	if rec := env.runAccount(salesA, "member", "sales", okReq, env.h.AccountDetail); rec.Code != http.StatusOK {
		t.Errorf("detail desa sendiri harus 200, got %d", rec.Code)
	}
}

// TestAccounts_F3_CSMLihatBinaan: CSM melihat desa yang di-assign padanya
// (assigned_csm atau backup_csm), bukan yang dimiliki sales lain.
func TestAccounts_F3_CSMLihatBinaan(t *testing.T) {
	env, sales := setupAccounts(t)
	csm := env.seedMember(t, "csm@local", "member", 0).ID

	env.seedAccount(t, "Desa Binaan", &sales, &csm, nil)
	env.seedAccount(t, "Desa Cadangan", &sales, nil, &csm)
	env.seedAccount(t, "Desa Lain", &sales, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts", nil, "")
	body := env.runAccount(csm, "member", "csm", req, env.h.AccountsList).Body.String()
	if !strings.Contains(body, "Desa Binaan") || !strings.Contains(body, "Desa Cadangan") {
		t.Error("CSM harus melihat desa assigned & backup")
	}
	if strings.Contains(body, "Desa Lain") {
		t.Error("CSM tak boleh melihat desa yang bukan binaannya")
	}
}

// TestAccounts_F3_SupportNolBaris: Support lolos gate read tapi ScopeNone →
// daftar KOSONG walau ada desa. Ia menyentuh desa hanya lewat konteks tiket.
func TestAccounts_F3_SupportNolBaris(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAccount(t, "Desa Ada", &uid, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts", nil, "")
	rec := env.runAccount(uid, "owner", "support", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("support lolos gate read, harus 200, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "Desa Ada") {
		t.Error("support ScopeNone harus nol baris — desa tak boleh tampil")
	}
}

// TestAccounts_F3_AdminLihatSemua: Admin (business) melihat SEMUA desa di
// workspace, tak peduli siapa ownernya.
func TestAccounts_F3_AdminLihatSemua(t *testing.T) {
	env, sales := setupAccounts(t)
	other := env.seedMember(t, "other@local", "member", 0).ID
	env.seedAccount(t, "Desa P", &sales, nil, nil)
	env.seedAccount(t, "Desa Q", &other, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts", nil, "")
	body := env.runAccount(sales, "owner", "admin", req, env.h.AccountsList).Body.String()
	if !strings.Contains(body, "Desa P") || !strings.Contains(body, "Desa Q") {
		t.Error("admin bisnis harus melihat semua desa di workspace")
	}
}

// --- F4: field masking -----------------------------------------------------

// TestAccounts_F4_PhoneMasking: nomor HP kontak utuh HANYA Sales; role lain
// menerima mask. Nilai asli tak boleh SAMPAI ke browser non-Sales (view-source).
func TestAccounts_F4_PhoneMasking(t *testing.T) {
	env, uid := setupAccounts(t)
	phone := "0812-3456-7890"
	a := env.seedAccountWithPhone(t, "Desa Kontak", uid, phone)

	cases := []struct {
		role string
		full bool
	}{
		{"sales", true},
		{"admin", false},
		{"manager", false},
		{"csm", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID), nil, itoa(a.ID))
			body := env.runAccount(uid, "owner", c.role, req, env.h.AccountDetail).Body.String()
			has := strings.Contains(body, phone)
			if c.full && !has {
				t.Errorf("sales harus melihat nomor utuh")
			}
			if !c.full && has {
				t.Errorf("role %q BOCOR — nomor asli sampai ke browser non-Sales", c.role)
			}
		})
	}
}

// TestAccounts_F4_NonSalesTakBisaTimpaPhone: editor non-Sales mengirim mask
// (field terkunci), tapi handler mempertahankan nomor ASLI — mask tak boleh
// menimpa nilai tersimpan.
func TestAccounts_F4_NonSalesTakBisaTimpaPhone(t *testing.T) {
	env, uid := setupAccounts(t)
	phone := "0812-3456-7890"
	a := env.seedAccountWithPhone(t, "Desa Kontak", uid, phone)

	// Admin menyunting: form mengirim mask (flsHidden) sebagai contact_phone.
	form := accountFormValues("Desa Kontak", "prospect")
	form.Set("contact_phone", flsHidden)
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID), form, itoa(a.ID))
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountUpdate); rec.Code != http.StatusSeeOther {
		t.Fatalf("update gagal: %d\n%s", rec.Code, rec.Body.String())
	}

	got, _ := env.q.GetAccount(t.Context(), a.ID)
	if got.ContactPhone == nil || *got.ContactPhone != phone {
		t.Errorf("nomor asli harus dipertahankan, got %v (mask menimpa = F4 bocor)", got.ContactPhone)
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
