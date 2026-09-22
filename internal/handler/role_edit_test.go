package handler

import (
	"net/http"
	"strings"
	"testing"
)

// role_edit_test.go — halaman DETAIL/EDIT satu peran (GET /w/{slug}/roles/{name}).
// Yang dijaga: gerbang crm:roles sama seperti daftar (URL detail pun bisa diketik
// langsung), peran tak ada → redirect role_notfound, peran kustom render editor,
// peran sistem render terkunci (tanpa tombol simpan). Sibling roles_test.go —
// berbagi helper (setupRoles, rolesReq, runAccount) di package yang sama.

// TestRoleEdit_Gate: hanya admin (pemegang crm:roles) boleh membuka detail;
// peran CRM lain & "" ditolak 403 dengan penjelasan — bukan diandalkan dari
// daftar, sebab URL detail bisa diketik langsung.
func TestRoleEdit_Gate(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "own", false)
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", false},
		{"sales", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := rolesReq(http.MethodGet, "/w/test/roles/finance", nil, "finance")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.RoleEditPage)
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

// TestRoleEdit_NotFound: admin membuka peran yang tak ada → kembali ke daftar
// dengan role_notfound (bukan 404 telanjang), tanpa menyentuh render editor.
func TestRoleEdit_NotFound(t *testing.T) {
	env, uid := setupRoles(t)
	req := rolesReq(http.MethodGet, "/w/test/roles/ghost", nil, "ghost")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleEditPage)
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "err=role_notfound") {
		t.Errorf("peran tak ada harus redirect role_notfound, got %q (status %d)", loc, rec.Code)
	}
}

// TestRoleEdit_RendersCustom: peran kustom → editor tampil (nama tampilan +
// tombol simpan), bukti halaman memuat form sunting yang bisa di-POST.
func TestRoleEdit_RendersCustom(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "own", false)
	req := rolesReq(http.MethodGet, "/w/test/roles/finance", nil, "finance")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleEditPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Keuangan") {
		t.Error("body harus memuat nama tampilan peran")
	}
	if !strings.Contains(body, "Simpan Perubahan") {
		t.Error("peran kustom harus punya tombol simpan (form editor)")
	}
}

// TestRoleEdit_DealsApproveColumnHidden: kolom "Setujui" TAK tampil sbg
// checkbox di baris Deals (BL-145 subtask 1, CanApprove=false — nol enforcement
// point hari ini), walau Manager punya default grant crm:deals approve
// (business_defaults.go). Baris Renewals TETAP tampil checkbox-nya
// (CanApprove=true sejak subtask 2, fiturnya nyata) — pembanding negatif agar
// bukti kolom lain tak ikut disembunyikan.
func TestRoleEdit_DealsApproveColumnHidden(t *testing.T) {
	env, uid := setupRoles(t)
	req := rolesReq(http.MethodGet, "/w/test/roles/manager", nil, "manager")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleEditPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, `name="approve.crm:deals"`) {
		t.Error("kolom Setujui baris Deals harus disembunyikan (CanApprove=false)")
	}
	if !strings.Contains(body, `name="approve.crm:renewals"`) {
		t.Error("kolom Setujui baris Renewals harus tetap tampil (CanApprove=true)")
	}
}

// TestRoleEdit_RenewalApproveMovedToRenewals: kolom "Setujui" tampil di baris
// Renewals, TAK di baris Renewal Management (BL-145 subtask 2 — approve renewal
// upsell cuma pernah muncul di halaman Subscription detail/dunia Renewals, nol
// kemunculan di halaman Renewal Management manapun). Pembanding negatif+positif
// sekaligus, agar bukti flag CanApprove benar-benar TERTUKAR posisi (bukan cuma
// ditambah di satu sisi lalu lupa dicabut di sisi lain).
func TestRoleEdit_RenewalApproveMovedToRenewals(t *testing.T) {
	env, uid := setupRoles(t)
	req := rolesReq(http.MethodGet, "/w/test/roles/manager", nil, "manager")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleEditPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `name="approve.crm:renewals"`) {
		t.Error("kolom Setujui baris Renewals harus tampil (CanApprove=true)")
	}
	if strings.Contains(body, `name="approve.crm:renewal_mgmt"`) {
		t.Error("kolom Setujui baris Renewal Management harus disembunyikan (CanApprove=false)")
	}
}

// TestRoleEdit_ApproveARRReactiveToLevel: kolom "Setujui" (baris Renewals,
// satu-satunya CanApprove sejak subtask 2) & "Lihat ARR" (Subscriptions,
// satu-satunya CanARR) ter-render reaktif Datastar (BL-145 subtask 4/5) — level
// select ter-bind signal per baris, checkbox terkait ter-bind signal SENDIRI +
// dinonaktifkan kondisional, dan handler on:change level memaksa signal
// checkbox itu balik ke false saat kondisi disabled terpenuhi. Reaktivitas ini
// murni UX klien — backend (readRoleMatrix guard hasLevel/arrGateOpen, BL-145
// subtask 0 + fix cross-module) tetap penjaga sesungguhnya, tak diuji ulang di
// sini.
//
// arr_subscriptions kini digerbangi LINTAS MODUL (authz.ARRGateObjects, fix
// checkbox "Lihat Nilai Kontrak" tak terbuka walau modul lain sudah "Kelola")
// — disabled/reset checkbox bukan lagi semata $lvl_subscriptions=='none',
// tapi AND semua modul gate == 'none' (urut mengikuti crmModules: accounts,
// leads, deals, subscriptions, renewals, churn, reports_sales, reports_cs,
// reports_subscriptions). resetStmt itu terpasang di on:change SETIAP baris
// gate, termasuk Renewals (makanya baris Renewals kini juga membawa reset arr
// setelah reset apv miliknya sendiri, dipisah ";").
func TestRoleEdit_ApproveARRReactiveToLevel(t *testing.T) {
	env, uid := setupRoles(t)
	req := rolesReq(http.MethodGet, "/w/test/roles/manager", nil, "manager")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleEditPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	const arrGateDisabled = `$lvl_accounts==&#39;none&#39;&amp;&amp;$lvl_leads==&#39;none&#39;&amp;&amp;` +
		`$lvl_deals==&#39;none&#39;&amp;&amp;$lvl_subscriptions==&#39;none&#39;&amp;&amp;` +
		`$lvl_renewals==&#39;none&#39;&amp;&amp;$lvl_churn==&#39;none&#39;&amp;&amp;` +
		`$lvl_reports_sales==&#39;none&#39;&amp;&amp;$lvl_reports_cs==&#39;none&#39;&amp;&amp;` +
		`$lvl_reports_subscriptions==&#39;none&#39;`
	for _, want := range []string{
		`data-bind="lvl_renewals"`, // level select baris Renewals ter-bind
		`data-bind="apv_renewals"`, // checkbox Setujui ter-bind signal sendiri
		`data-attr="{disabled: $lvl_renewals == &#39;none&#39;}"`,
		// baris Renewals: reset apv (kondisi sendiri) + reset arr (gate lintas modul), digabung ";"
		`data-on:change="evt.target.value===&#39;none&#39;&amp;&amp;($apv_renewals=false);` +
			`(` + arrGateDisabled + `)&amp;&amp;($arr_subscriptions=false)"`,
		`data-bind="lvl_subscriptions"`, // level select baris Subscriptions ter-bind
		`data-bind="arr_subscriptions"`, // checkbox Lihat ARR ter-bind signal sendiri
		`data-attr="{disabled: ` + arrGateDisabled + `}"`,
		// baris Subscriptions: cuma reset arr (tak CanApprove)
		`data-on:change="(` + arrGateDisabled + `)&amp;&amp;($arr_subscriptions=false)"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("matriks harus memuat %q (reaktivitas approve/arr ↔ level):\n%s", want, body)
		}
	}
}

// TestRoleEdit_UnenforcedModulesHideKelolaOption: audit 2026-09 — 7 modul tanpa
// titik enforcement CanBusiness(ctx,obj,"write") (dashboard/subscriptions/
// activities/4 Reports) tak boleh lagi menawarkan opsi "Kelola" (value="write")
// di dropdown levelnya, walau peran Manager punya grant default
// crm:subscriptions write (business_defaults.go) — moduleLevel (roles_card.go)
// merendahkannya ke "read" sebelum dirender, jadi select-nya harus tetap
// ter-render "read" terpilih, BUKAN default diam-diam ke opsi pertama. Baris
// Accounts (WriteEnforced=true) jadi pembanding positif: opsi "Kelola" harus
// tetap ada di sana.
func TestRoleEdit_UnenforcedModulesHideKelolaOption(t *testing.T) {
	env, uid := setupRoles(t)
	req := rolesReq(http.MethodGet, "/w/test/roles/manager", nil, "manager")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleEditPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	selectBody := func(t *testing.T, obj string) string {
		t.Helper()
		marker := `name="level.` + obj + `"`
		start := strings.Index(body, marker)
		if start < 0 {
			t.Fatalf("select level.%s tak ditemukan", obj)
		}
		end := strings.Index(body[start:], "</select>")
		if end < 0 {
			t.Fatalf("penutup </select> level.%s tak ditemukan", obj)
		}
		return body[start : start+end]
	}

	for _, obj := range []string{
		"crm:dashboard", "crm:subscriptions", "crm:activities",
		"crm:reports_sales", "crm:reports_cs", "crm:reports_support", "crm:reports_subscriptions",
	} {
		seg := selectBody(t, obj)
		if strings.Contains(seg, `value="write"`) {
			t.Errorf("select level.%s tak boleh punya opsi Kelola (value=\"write\")", obj)
		}
	}

	// Manager punya grant default crm:subscriptions write (business_defaults.go)
	// — setelah direndahkan ke "read", opsi "read" harus tetap terpilih (bukti
	// moduleLevel bekerja, bukan cuma opsi write yang hilang tanpa fallback).
	subsSeg := selectBody(t, "crm:subscriptions")
	if !strings.Contains(subsSeg, `value="read"`) {
		t.Error("select level.crm:subscriptions harus tetap punya opsi read")
	}

	// Pembanding positif: modul WriteEnforced=true (Accounts) tetap punya "Kelola".
	accSeg := selectBody(t, "crm:accounts")
	if !strings.Contains(accSeg, `value="write"`) {
		t.Error("select level.crm:accounts harus tetap punya opsi Kelola (WriteEnforced=true)")
	}
}

// TestRoleEdit_RenewalsChurnReachabilityHint: audit 2026-09 — baris Renewals &
// Churn / Cancellations menampilkan keterangan kecil "Butuh Active
// Subscriptions ≥ Lihat juga" (moduleHints, role_edit.go), sebab izin
// crm:renewals/crm:churn TERSIMPAN SAH tapi halamannya (SubscriptionDetail/
// SubscriptionRenewals/SubscriptionChurnList) digerbangi canViewSubscriptions
// ("crm:subscriptions" read) — bukan objek modul ini sendiri. Manager punya
// grant default crm:renewals write & crm:churn write (business_defaults.go),
// jadi tanpa hint ini admin penyunting takkan menyadari mengapa memberi
// "Kelola" pada dua baris itu saja tak cukup. Baris Accounts (tak ada di
// moduleHints) jadi pembanding negatif — hint tak boleh bocor ke baris lain.
func TestRoleEdit_RenewalsChurnReachabilityHint(t *testing.T) {
	env, uid := setupRoles(t)
	req := rolesReq(http.MethodGet, "/w/test/roles/manager", nil, "manager")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleEditPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()

	rowBody := func(t *testing.T, marker string) string {
		t.Helper()
		pos := strings.Index(body, marker)
		if pos < 0 {
			t.Fatalf("penanda %q tak ditemukan", marker)
		}
		// <p> hint ada di kolom Label, mendahului kolom Akses tempat marker
		// (name="level.X") berada dalam <tr> yang sama — cari MUNDUR ke <tr
		// pembuka baris ini dulu, baru MAJU ke </tr> penutupnya.
		start := strings.LastIndex(body[:pos], "<tr")
		if start < 0 {
			t.Fatalf("pembuka <tr sebelum %q tak ditemukan", marker)
		}
		end := strings.Index(body[pos:], "</tr>")
		if end < 0 {
			t.Fatalf("penutup </tr> setelah %q tak ditemukan", marker)
		}
		return body[start : pos+end]
	}

	const hint = "Butuh Active Subscriptions"
	for _, obj := range []string{"crm:renewals", "crm:churn"} {
		row := rowBody(t, `name="level.`+obj+`"`)
		if !strings.Contains(row, hint) {
			t.Errorf("baris %s harus memuat keterangan reachability %q", obj, hint)
		}
	}

	// Pembanding negatif: baris Accounts tak punya entri di moduleHints, hint
	// tak boleh muncul di baris ini.
	accRow := rowBody(t, `name="level.crm:accounts"`)
	if strings.Contains(accRow, hint) {
		t.Error("baris Accounts tak boleh memuat keterangan reachability Renewals/Churn")
	}
}

// TestRoleEdit_SystemLocked: peran sistem (admin) → keterangan terkunci, TANPA
// tombol simpan — admin diwakili glob crm:* yang tak terpetakan ke matriks.
func TestRoleEdit_SystemLocked(t *testing.T) {
	env, uid := setupRoles(t)
	req := rolesReq(http.MethodGet, "/w/test/roles/admin", nil, "admin")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleEditPage)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "akses penuh") {
		t.Error("peran sistem harus tampil keterangan akses penuh")
	}
	if strings.Contains(body, "Simpan Perubahan") {
		t.Error("peran sistem tak boleh punya tombol simpan")
	}
}
