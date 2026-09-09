package handler

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"go_starter/internal/authz"
	"go_starter/internal/codes"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// dashboard_test.go — Beranda ruang kerja (Modul 1, tasks.md M1-3) di sisi
// handler. Empat sumbu dijaga:
//
//   - Agregasi: ARR total hanya menjumlah subscription Active; pipeline
//     mengecualikan Closed Won/Lost; renewal jatuh tempo hanya jendela 30 hari.
//   - F3 (ownership): sales/csm own-scope hanya menghitung baris miliknya
//     sendiri; admin/manager (all-scope) menghitung lintas-owner. Dibuktikan
//     lewat renewal COUNT (bukan ARR — ARR tersamar F4 utk sales/csm).
//   - F4 (masking ARR): admin/manager melihat ARR asli; sales/csm/support
//     melihat penanda flsHidden ("•••") — kebijakan subscriptions, BUKAN deals.
//   - F2 (gerbang) fail-soft: anggota tanpa business_role jatuh ke Placeholder
//     biasa (200, bukan 403) — Beranda tetap terbuka utk semua anggota workspace.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS
// sesungguhnya diuji di rls_test.go. Setup/helper reuse accounts_test.go
// (setupAccounts/accountsReq/runAccount) + seedPlan (sales_quotes_test.go).

// seedDashboardSub menaruh satu subscription langsung lewat pool dengan status/
// ARR/end_date terkontrol — untuk menguji agregasi tanpa merangkai create.
// endDate nil → NULL (tak masuk hitungan renewal manapun).
func (e *testEnv) seedDashboardSub(
	t *testing.T, accountID, planID int64, owner *int64, status, arr string, endDate *time.Time,
) db.Subscription {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntitySubscription)
	if err != nil {
		t.Fatalf("generate subscription code: %v", err)
	}
	end := pgtype.Date{}
	if endDate != nil {
		end = pgtype.Date{Time: *endDate, Valid: true}
	}
	s, err := e.q.CreateSubscription(t.Context(), db.CreateSubscriptionParams{
		TenantID:          e.tenantID,
		EntityCode:        &code,
		SubscriptionOwner: owner,
		AccountID:         accountID,
		PlanID:            &planID,
		Status:            status,
		EndDate:           end,
		AutoRenew:         false,
		Mrr:               numFrom(t, "500000"),
		Arr:               numFrom(t, arr),
		CreatedBy:         owner,
	})
	if err != nil {
		t.Fatalf("seed dashboard subscription: %v", err)
	}
	// BL-88 PR2b: langganan nyata punya ≥1 subscription_items (invarian 1-Active &
	// agregasi laporan kini via item). Seed 1 item cermin parent; parent_active
	// diturunkan trigger dari status (Active → aktif → menegakkan idx item).
	lineNo := int16(1)
	if _, err := e.q.AddSubscriptionItem(t.Context(), db.AddSubscriptionItemParams{
		SubscriptionID: s.ID,
		TenantID:       e.tenantID,
		PlanID:         &planID,
		Quantity:       1,
		UnitPrice:      numFrom(t, "500000"),
		Subtotal:       numFrom(t, "500000"),
		Mrr:            numFrom(t, "500000"),
		Arr:            numFrom(t, arr),
		LineNo:         &lineNo,
	}); err != nil {
		t.Fatalf("seed dashboard subscription item: %v", err)
	}
	return s
}

// dashboardBody menjalankan WorkspaceHome sebagai (uid, tenantRole, businessRole)
// & mengembalikan body HTML mentah.
func (e *testEnv) dashboardBody(t *testing.T, uid int64, tenantRole, businessRole string) string {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/", nil, "")
	rec := e.runAccount(uid, tenantRole, businessRole, req, e.h.WorkspaceHome)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// dashboardKPIValue mengekstrak nilai kartu KPI berdasar label (pola pasti
// dashboardKPICard: <p>label</p><p class="...">value</p>) — presisi & tak
// rentan tabrakan digit dgn substring-match longgar di tempat lain di body.
func dashboardKPIValue(t *testing.T, body, label string) string {
	t.Helper()
	re := regexp.MustCompile(`>` + regexp.QuoteMeta(label) + `</p><p class="[^"]*">([^<]*)</p>`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("kartu KPI %q tak ditemukan di body:\n%s", label, body)
	}
	return m[1]
}

// --- agregasi ---------------------------------------------------------------

// TestDashboard_ARRTotalSumsActiveOnly: ARR total hanya menjumlah subscription
// berstatus Active; Trial tak ikut terhitung.
func TestDashboard_ARRTotalSumsActiveOnly(t *testing.T) {
	env, uid := setupAccounts(t)
	planA := env.seedPlan(t, "Paket Aktif", "PLAN-ARR-A", "1000000")
	planB := env.seedPlan(t, "Paket Trial", "PLAN-ARR-B", "1000000")
	acc := env.seedAccount(t, "Desa ARR", &uid, nil, nil)
	env.seedDashboardSub(t, acc.ID, planA, &uid, "Active", "6000000", nil)
	env.seedDashboardSub(t, acc.ID, planB, &uid, "Trial", "9999999", nil)

	body := env.dashboardBody(t, uid, "owner", "admin")
	if !strings.Contains(body, "Rp 6.000.000") {
		t.Errorf("ARR total harus Rp 6.000.000 (hanya Active), body:\n%s", body)
	}
	if strings.Contains(body, "9.999.999") {
		t.Error("ARR subscription Trial tak boleh ikut terhitung")
	}
}

// TestDashboard_RenewalsDueWindow: hanya subscription Active/PendingApproval
// dengan end_date dalam 30 hari ke depan yang terhitung — di luar jendela atau
// status lain tak ikut.
func TestDashboard_RenewalsDueWindow(t *testing.T) {
	env, uid := setupAccounts(t)
	planIn := env.seedPlan(t, "Paket Jatuh Tempo", "PLAN-DUE-IN", "1000000")
	planOut := env.seedPlan(t, "Paket Jauh", "PLAN-DUE-OUT", "1000000")
	planWrong := env.seedPlan(t, "Paket Salah Status", "PLAN-DUE-WS", "1000000")
	acc := env.seedAccount(t, "Desa Renewal", &uid, nil, nil)

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	within := today.AddDate(0, 0, 15)
	outside := today.AddDate(0, 0, 45)

	env.seedDashboardSub(t, acc.ID, planIn, &uid, "Active", "1200000", &within)
	env.seedDashboardSub(t, acc.ID, planOut, &uid, "Active", "9990000", &outside)
	env.seedDashboardSub(t, acc.ID, planWrong, &uid, "Trial", "8880000", &within)

	body := env.dashboardBody(t, uid, "owner", "admin")
	if !strings.Contains(body, "Rp 1.200.000") {
		t.Errorf("ARR renewal jatuh tempo harus Rp 1.200.000 (hanya dalam jendela), body:\n%s", body)
	}
	if strings.Contains(body, "9.990.000") || strings.Contains(body, "8.880.000") {
		t.Error("subscription di luar jendela/status salah tak boleh ikut ARR jatuh tempo")
	}
}

// --- F3: ownership -----------------------------------------------------------

// TestDashboard_RenewalsScopedByOwnership: sales (own-scope) hanya menghitung
// renewal jatuh tempo miliknya sendiri; manager (all-scope) menghitung lintas-
// owner. Dipakai renewal COUNT (bukan ARR) sebagai bukti F3 — ARR pada peran
// sales sengaja tersamar F4 (lihat TestDashboard_ARRMaskedForNonManager), jadi
// tak bisa dipakai membuktikan cakupan kepemilikan.
func TestDashboard_RenewalsScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "otherowner@local", "member", 0).ID
	planMine := env.seedPlan(t, "Paket Milikku", "PLAN-OWN-MINE", "1000000")
	planOther := env.seedPlan(t, "Paket Orang", "PLAN-OWN-OTHER", "1000000")
	accMine := env.seedAccount(t, "Desa Milikku", &uid, nil, nil)
	accOther := env.seedAccount(t, "Desa Orang", &other, nil, nil)

	within := time.Now().UTC().AddDate(0, 0, 10)
	env.seedDashboardSub(t, accMine.ID, planMine, &uid, "Active", "3000000", &within)
	env.seedDashboardSub(t, accOther.ID, planOther, &other, "Active", "5000000", &within)

	const label = "Renewal Jatuh Tempo (30 hari)"
	own := env.dashboardBody(t, uid, "owner", "sales")
	if got := dashboardKPIValue(t, own, label); got != "1" {
		t.Errorf("sales own-scope: renewal jatuh tempo = %q, want \"1\" (hanya miliknya)", got)
	}

	all := env.dashboardBody(t, uid, "owner", "manager")
	if got := dashboardKPIValue(t, all, label); got != "2" {
		t.Errorf("manager all-scope: renewal jatuh tempo = %q, want \"2\" (lintas-owner)", got)
	}
}

// --- F4: masking ARR ----------------------------------------------------------

// TestDashboard_ARRMaskedForNonManager: admin/manager melihat ARR asli;
// sales/csm/support melihat flsHidden — kebijakan subscriptions (bukan deals).
func TestDashboard_ARRMaskedForNonManager(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket Mask", "PLAN-MASK", "1000000")
	acc := env.seedAccount(t, "Desa Mask", &uid, nil, nil)
	env.seedDashboardSub(t, acc.ID, planID, &uid, "Active", "4000000", nil)

	cases := []struct {
		role     string
		wantReal bool
		wantHide bool
	}{
		{"admin", true, false},
		{"manager", true, false},
		{"sales", false, true},
		{"csm", false, true},
		{"support", false, true},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			body := env.dashboardBody(t, uid, "owner", c.role)
			if c.wantReal && !strings.Contains(body, "Rp 4.000.000") {
				t.Errorf("role %q harus melihat ARR asli, body:\n%s", c.role, body)
			}
			if c.wantHide && !strings.Contains(body, flsHidden) {
				t.Errorf("role %q harus melihat ARR tersamar (%s)", c.role, flsHidden)
			}
			if c.wantHide && strings.Contains(body, "Rp 4.000.000") {
				t.Errorf("role %q TAK boleh melihat ARR asli", c.role)
			}
		})
	}
}

// --- F2: gerbang fail-soft ----------------------------------------------------

// TestDashboard_NoBusinessRoleShowsPendingApproval: anggota (member) tanpa
// business_role — mis. baru mendaftar/diundang — diarahkan ke halaman "menunggu
// approval admin" (Opsi A, BL-105), BUKAN Beranda dengan menu mati. Tetap 200
// (bukan 403) & tanpa kebocoran KPI. Menggantikan perilaku lama (jatuh ke
// Placeholder sapaan) yang membiarkan user di Beranda tanpa penjelasan.
func TestDashboard_NoBusinessRoleShowsPendingApproval(t *testing.T) {
	env, uid := setupAccounts(t)

	body := env.dashboardBody(t, uid, "member", "")
	if strings.Contains(body, "ARR Total") {
		t.Error("tanpa business_role TAK boleh melihat kartu KPI Beranda")
	}
	if !strings.Contains(body, "Menunggu approval admin") {
		t.Errorf("member tanpa peran harus melihat halaman menunggu approval, body:\n%s", body)
	}
	if strings.Contains(body, "Selamat datang di") {
		t.Error("member tanpa peran tak lagi melihat Beranda Placeholder (kini halaman tunggu)")
	}
}

// TestDashboard_ManagerNoBusinessRoleGetsOnboardNotPending: pengecualian sengaja
// dari BL-105 — owner/admin tanpa business_role BUKAN "menunggu": mereka bisa
// menetapkan peran sendiri lewat banner CRMOnboard di Beranda. Jadi mereka TAK
// boleh kena halaman tunggu.
func TestDashboard_ManagerNoBusinessRoleGetsOnboardNotPending(t *testing.T) {
	env, uid := setupAccounts(t)

	body := env.dashboardBody(t, uid, "owner", "")
	if strings.Contains(body, "Menunggu approval admin") {
		t.Errorf("pengelola tanpa peran TAK boleh kena halaman tunggu, body:\n%s", body)
	}
	if !strings.Contains(body, "Aktifkan CRM untuk Anda") {
		t.Errorf("pengelola tanpa peran harus melihat banner CRMOnboard, body:\n%s", body)
	}
}

// --- BL-59a: section domain Sales (komposisi per-izin) -----------------------
//
// Beranda redesain (BL-59) mengomposisi section per-domain berdasar kapabilitas
// role: domain "Sales" hanya muncul bila role punya ≥1 dari crm:deals /
// crm:sales_activity / crm:leads (read). Selaras F2/F3 — role KUSTOM otomatis
// dapat section sesuai modulnya. Tiga sumbu diuji: visibilitas per-role bawaan,
// role kustom (union parsial), dan F3 (angka section menghormati data_scope).

// seedClosingDeal menaruh satu deal TERBUKA yang expected_close_date-nya jatuh
// di bulan kalender berjalan — untuk KPI section Sales "Deal Tutup Bulan Ini".
func (e *testEnv) seedClosingDeal(t *testing.T, accountID int64, owner *int64) db.Deal {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityDeal)
	if err != nil {
		t.Fatalf("generate deal code: %v", err)
	}
	now := time.Now().UTC()
	first := pgtype.Date{
		Time:  time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
	d, err := e.q.CreateDeal(t.Context(), db.CreateDealParams{
		TenantID: e.tenantID, EntityCode: &code, DealName: "Deal Tutup",
		AccountID: accountID, DealOwner: owner, Stage: dealInitialStage,
		ExpectedCloseDate: first, CreatedBy: owner,
	})
	if err != nil {
		t.Fatalf("seed closing deal: %v", err)
	}
	return d
}

// TestDashboardSales_DomainVisibleByCapability: role dgn kapabilitas Sales inti
// (admin/manager/sales punya crm:deals) melihat heading section "Sales" + KPI
// Win Rate; role tanpanya (csm/support) TAK melihat section Sales sama sekali
// (heading tak berdiri kosong). BL-98: section domain kini TANPA chart apa pun
// (chart-pipeline dipindah ke Sales Report) — hanya KPI + tautan Report.
func TestDashboardSales_DomainVisibleByCapability(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Sales Dom", &uid, nil, nil)
	env.seedDeal(t, acc.ID, &uid)

	for _, role := range []string{"admin", "manager", "sales"} {
		t.Run(role+" melihat section Sales", func(t *testing.T) {
			body := env.dashboardBody(t, uid, "owner", role)
			if !strings.Contains(body, ">Sales</h2>") {
				t.Errorf("role %q harus melihat heading section Sales, body:\n%s", role, body)
			}
			if !strings.Contains(body, "Win Rate") {
				t.Errorf("role %q (crm:deals) harus melihat KPI Win Rate di section Sales", role)
			}
			// BL-98: chart domain dibuang seluruhnya.
			if strings.Contains(body, "chart-pipeline") {
				t.Errorf("role %q TAK boleh lagi melihat chart-pipeline (dipindah ke Report)", role)
			}
		})
	}

	for _, role := range []string{"csm", "support"} {
		t.Run(role+" tak melihat section Sales", func(t *testing.T) {
			body := env.dashboardBody(t, uid, "owner", role)
			if strings.Contains(body, ">Sales</h2>") {
				t.Errorf("role %q (tanpa kapabilitas Sales) TAK boleh melihat section Sales, body:\n%s", role, body)
			}
		})
	}
}

// TestDashboardSales_CustomRoleLeadsOnlyNoSection (BL-98): setelah section
// dirampingkan, KEDUA KPI Sales (Win Rate, Deal Tutup) bergantung crm:deals.
// Role KUSTOM dgn hanya crm:dashboard + crm:leads (TANPA crm:deals) karena itu
// TAK menyumbang KPI apa pun ke section Sales → section tak muncul (heading tak
// berdiri kosong). Komposisi per-kapabilitas tetap berlaku; hanya set butirnya
// yang menyusut.
func TestDashboardSales_CustomRoleLeadsOnlyNoSection(t *testing.T) {
	env, uid := setupAccounts(t)
	env.loadBusinessRolesWith(t,
		authz.BusinessPerm{Role: "leadsonly", Obj: "crm:dashboard", Act: "read"},
		authz.BusinessPerm{Role: "leadsonly", Obj: "crm:leads", Act: "read"},
	)
	acc := env.seedAccount(t, "Desa Custom", &uid, nil, nil)
	env.seedDeal(t, acc.ID, &uid) // deal ADA tapi role tak berhak melihatnya

	req := accountsReq(http.MethodGet, "/w/test/", nil, "")
	rec := env.runAccountScope(uid, "member", "leadsonly", "all", req, env.h.WorkspaceHome)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, ">Sales</h2>") {
		t.Errorf("role kustom crm:leads (tanpa crm:deals) TAK boleh melihat section Sales, body:\n%s", body)
	}
	if strings.Contains(body, "Win Rate") {
		t.Error("role kustom tanpa crm:deals TAK boleh melihat KPI Win Rate")
	}
	// Dashboard tetap terbuka (chart-health global) walau section Sales absen.
	if !strings.Contains(body, "chart-health") {
		t.Errorf("dashboard tetap harus render chart-health global, body:\n%s", body)
	}
}

// TestDashboardSales_DealsClosingScopedByOwnership (F3): KPI "Deal Tutup Bulan
// Ini" menghormati data_scope — sales (own) hanya menghitung deal miliknya;
// manager (all) menghitung lintas-owner. Sumber filter = DealsListFilterFor,
// sama dgn papan Kanban (bukan logic scope duplikat).
func TestDashboardSales_DealsClosingScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "otherdeal@local", "member", 0).ID
	accMine := env.seedAccount(t, "Desa Deal Mine", &uid, nil, nil)
	accOther := env.seedAccount(t, "Desa Deal Other", &other, nil, nil)
	env.seedClosingDeal(t, accMine.ID, &uid)
	env.seedClosingDeal(t, accOther.ID, &other)

	const label = "Deal Tutup Bulan Ini"
	own := env.dashboardBody(t, uid, "owner", "sales")
	if got := dashboardKPIValue(t, own, label); got != "1" {
		t.Errorf("sales own-scope: %q = %q, want \"1\" (hanya miliknya)", label, got)
	}
	all := env.dashboardBody(t, uid, "owner", "manager")
	if got := dashboardKPIValue(t, all, label); got != "2" {
		t.Errorf("manager all-scope: %q = %q, want \"2\" (lintas-owner)", label, got)
	}
}

// --- BL-98: Beranda ramping untuk role multi-domain -------------------------
//
// Keputusan (arah A): tiap section domain maksimum 2 KPI penting (tetap digate
// per-kapabilitas), TANPA chart domain (satu-satunya chart Beranda = donut health
// GLOBAL), + tautan "Lihat Laporan →" ke halaman Report domain — dirender HANYA
// bila role ber-crm:reports (jangan pernah menautkan halaman yang akan 403).

// TestDashboardBL98_AdminSlimSectionsAndLinks: admin (semua kapabilitas) melihat
// keempat section ramping — tepat 2 KPI terpilih per-domain, tautan Report tiap
// domain, dan NOL chart domain (hanya chart-health global bertahan).
func TestDashboardBL98_AdminSlimSectionsAndLinks(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa BL98", &uid, nil, nil)
	env.seedDeal(t, acc.ID, &uid) // section punya data (visibilitas tak bergantung data)

	body := env.dashboardBody(t, uid, "owner", "admin")

	// 2 KPI terpilih per-domain HADIR.
	for _, kpi := range []string{
		"Win Rate", "Deal Tutup Bulan Ini", // Sales
		"MRR", "Churn Rate", // Langganan
		"Desa Berisiko", "Adoption Rate", // Customer Success
		"Tiket Terbuka", "Kepatuhan SLA", // Support
	} {
		if !strings.Contains(body, kpi) {
			t.Errorf("admin harus melihat KPI ramping %q, body:\n%s", kpi, body)
		}
	}

	// KPI/butir yang DIPINDAH ke Report TIDAK boleh muncul di Beranda.
	for _, dropped := range []string{
		"Aktivitas Sales", "Langganan Aktif", "Renewal Rate",
		"Engagement Jatuh Tempo (7 hari)", "Terlambat / Langgar SLA",
		"Rata Waktu Penyelesaian",
	} {
		if strings.Contains(body, dropped) {
			t.Errorf("KPI %q sudah dipindah ke Report, TAK boleh muncul di Beranda", dropped)
		}
	}

	// Tautan "Lihat Laporan →" tiap domain (4 total) dengan href yang benar.
	if n := strings.Count(body, "Lihat Laporan →"); n != 4 {
		t.Errorf("harus ada 4 tautan 'Lihat Laporan →' (satu per domain), got %d", n)
	}
	for _, href := range []string{
		`href="/w/test/reports/sales"`,
		`href="/w/test/reports/subscriptions"`,
		`href="/w/test/reports/customer-success"`,
		`href="/w/test/reports/support"`,
	} {
		if !strings.Contains(body, href) {
			t.Errorf("tautan Report harus menuju %s, body:\n%s", href, body)
		}
	}

	// NOL chart domain — hanya donut health global bertahan.
	if !strings.Contains(body, "chart-health") {
		t.Errorf("donut health global harus tetap ada, body:\n%s", body)
	}
	for _, chart := range []string{
		"chart-pipeline", "chart-leads", "chart-mrr-movement",
		"chart-revenue-plan", "chart-onboarding", "chart-tickets-priority",
		"chart-agent-workload",
	} {
		if strings.Contains(body, chart) {
			t.Errorf("chart domain %q sudah dibuang dari Beranda (BL-98)", chart)
		}
	}
}

// TestDashboardBL98_ReportLinkGatedByReportsCap: role KUSTOM dgn kapabilitas
// domain (crm:deals) TAPI TANPA crm:reports tetap melihat section + 2 KPI-nya,
// namun TANPA tautan "Lihat Laporan →" — kita tak pernah menautkan halaman
// Report yang akan menolaknya 403.
func TestDashboardBL98_ReportLinkGatedByReportsCap(t *testing.T) {
	env, uid := setupAccounts(t)
	env.loadBusinessRolesWith(t,
		authz.BusinessPerm{Role: "dealsnoreports", Obj: "crm:dashboard", Act: "read"},
		authz.BusinessPerm{Role: "dealsnoreports", Obj: "crm:deals", Act: "read"},
	)
	acc := env.seedAccount(t, "Desa NoReports", &uid, nil, nil)
	env.seedDeal(t, acc.ID, &uid)

	req := accountsReq(http.MethodGet, "/w/test/", nil, "")
	rec := env.runAccountScope(uid, "member", "dealsnoreports", "all", req, env.h.WorkspaceHome)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	if !strings.Contains(body, ">Sales</h2>") || !strings.Contains(body, "Win Rate") {
		t.Errorf("role crm:deals harus tetap melihat section Sales + Win Rate, body:\n%s", body)
	}
	if strings.Contains(body, "Lihat Laporan →") {
		t.Errorf("role tanpa crm:reports TAK boleh melihat tautan 'Lihat Laporan →', body:\n%s", body)
	}
	if strings.Contains(body, `href="/w/test/reports/sales"`) {
		t.Error("role tanpa crm:reports TAK boleh punya tautan ke halaman Report")
	}
}
