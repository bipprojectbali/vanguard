package handler

import (
	"net/http"
	"strings"
	"testing"
)

// accounts_list_test.go — bilah tab cakupan (All/My/Belum-ada-Owner) + kolom
// Owner/CSM di DAFTAR desa. Yang dijaga di sini adalah keputusan "tab mana untuk
// siapa" (accounts_list.go) yang tegak lurus dari gerbang F2 (accounts_test.go)
// & filter kepemilikan F3 (accounts_fls_test.go):
//
//   - Tab HANYA untuk peran ScopeAll (admin/manager). Sales/CSM (ScopeOwn) melihat
//     All≡My → tab redundan & disembunyikan; ?view= mereka diabaikan (dinormalkan
//     ke cakupan asli), jadi URL yang diutak-atik tak bisa menembus batas F3.
//   - view=my override cakupan default ScopeAll → union kepemilikan aktor.
//   - view=unowned = ScopeAll + saring account_owner IS NULL.
//   - Kolom Owner/CSM menampilkan NAMA anggota (jatuh ke email) yang diresolusi
//     handler; id tak-tertugas → "—".

// tabLabels = penanda tab non-default; kehadirannya membuktikan bilah tab dirender.
var tabLabels = []string{"Desa Saya", "Belum ada Owner"}

func assertHasTabs(t *testing.T, body string, want bool) {
	t.Helper()
	for _, l := range tabLabels {
		has := strings.Contains(body, l)
		if has != want {
			t.Errorf("tab %q: ada=%v, mau=%v", l, has, want)
		}
	}
}

// TestAccountsList_TabsVisibility: bilah tab muncul HANYA untuk peran ScopeAll.
// Peran ScopeOwn (sales/csm) & ScopeNone (support) tak menampilkannya — bagi
// mereka All≡My, jadi tab tak punya makna.
func TestAccountsList_TabsVisibility(t *testing.T) {
	env, uid := setupAccounts(t)
	cases := []struct {
		role     string
		wantTabs bool
	}{
		{"admin", true},
		{"manager", true},
		{"sales", false},
		{"csm", false},
		{"support", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/accounts", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.AccountsList)
			if rec.Code != http.StatusOK {
				t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
			}
			assertHasTabs(t, rec.Body.String(), c.wantTabs)
		})
	}
}

// TestAccountsList_ViewMy: bagi peran ScopeAll, ?view=my MENGGANTIKAN cakupan
// default (semua desa) dengan union kepemilikan aktor — hanya desa yang dimiliki
// uid yang tampil, walau ia admin yang berhak melihat semuanya.
func TestAccountsList_ViewMy(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain@x", "member", env.tenantID)
	env.seedAccount(t, "Desa Mine", &uid, nil, nil)
	env.seedAccount(t, "Desa Lain", &lain.ID, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts?view=my", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Mine") {
		t.Errorf("view=my harus memuat desa milik aktor (Desa Mine)")
	}
	if strings.Contains(body, "Desa Lain") {
		t.Errorf("view=my tak boleh memuat desa milik orang lain (Desa Lain)")
	}
}

// TestAccountsList_ViewUnowned: ?view=unowned menyaring HANYA desa tanpa owner —
// pekerjaan bilah "Belum ada Owner" (menemukan desa yang perlu ditugaskan).
func TestAccountsList_ViewUnowned(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAccount(t, "Desa Berpemilik", &uid, nil, nil)
	env.seedAccount(t, "Desa Yatim", nil, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts?view=unowned", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Yatim") {
		t.Errorf("view=unowned harus memuat desa tanpa owner (Desa Yatim)")
	}
	if strings.Contains(body, "Desa Berpemilik") {
		t.Errorf("view=unowned tak boleh memuat desa ber-owner (Desa Berpemilik)")
	}
}

// TestAccountsList_ViewIgnoredForScopeOwn: peran ScopeOwn tak punya tab, jadi
// ?view=unowned di URL diabaikan (dinormalkan ke cakupan asli). Bukti F3 tak bisa
// ditembus lewat URL: sales tetap hanya melihat desanya, tak pernah desa yatim
// milik siapa pun.
func TestAccountsList_ViewIgnoredForScopeOwn(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAccount(t, "Desa Sales", &uid, nil, nil)
	env.seedAccount(t, "Desa Yatim", nil, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts?view=unowned", nil, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	assertHasTabs(t, body, false) // ScopeOwn: tak ada tab
	if !strings.Contains(body, "Desa Sales") {
		t.Errorf("sales harus tetap melihat desanya (Desa Sales)")
	}
	if strings.Contains(body, "Desa Yatim") {
		t.Errorf("?view=unowned tak boleh menembus F3: sales tak boleh melihat desa yatim")
	}
}

// TestAccountsList_OwnerCSMColumns: kolom Owner & CSM menampilkan nama anggota
// (di sini email, karena member seed tak bernama) hasil resolusi handler dari
// user_id — sekali per muat halaman, bukan N+1.
func TestAccountsList_OwnerCSMColumns(t *testing.T) {
	env, uid := setupAccounts(t)
	owner := env.seedMember(t, "villowner@desa.test", "member", env.tenantID)
	csm := env.seedMember(t, "villcsm@desa.test", "member", env.tenantID)
	env.seedAccount(t, "Desa Bertugas", &owner.ID, &csm.ID, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "villowner@desa.test") {
		t.Errorf("kolom Owner harus menampilkan nama pemilik")
	}
	if !strings.Contains(body, "villcsm@desa.test") {
		t.Errorf("kolom CSM harus menampilkan nama CSM binaan")
	}
}
