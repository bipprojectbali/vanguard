package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"go_starter/internal/authz"
	"go_starter/internal/codes"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// dashboard_sales_test.go — komposisi section domain "Sales" di Beranda
// (BL-59a) + perampingan lintas-domain (BL-98). Setup/helper reuse
// dashboard_test.go (seedDashboardSub/dashboardBody/dashboardKPIValue) +
// accounts_test.go (setupAccounts/accountsReq/runAccountScope).
//
// Beranda redesain (BL-59) mengomposisi section per-domain berdasar kapabilitas
// role: domain "Sales" hanya muncul bila role punya ≥1 dari crm:deals /
// crm:sales_activity / crm:leads (read). Selaras F2/F3 — role KUSTOM otomatis
// dapat section sesuai modulnya. Tiga sumbu diuji: visibilitas per-role bawaan,
// role kustom (union parsial), dan F3 (angka section menghormati data_scope).
//
// BL-98 (arah A): tiap section domain maksimum 2 KPI penting (saat itu TANPA
// chart domain — satu-satunya chart Beranda = donut health GLOBAL), + tautan
// "Lihat Laporan →" ke halaman Report domain — dirender HANYA bila role
// ber-crm:reports (jangan pernah menautkan halaman yang akan 403).
//
// BL-141 (membalik BL-98 KHUSUS Sales): 3 chart inline (pipeline/leads/
// win-loss, id BARU chart-sales-*) ditambah kembali di bawah crm:deals — lihat
// dashboard_sales_charts_test.go. Test di bawah HANYA memeriksa id chart era
// pre-BL-98 (chart-pipeline dst, TANPA "-sales-") yang sengaja tak pernah
// dipakai lagi — bukan klaim "nol chart" secara umum.

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
// (heading tak berdiri kosong). Chart BL-141 (chart-sales-*) diuji terpisah di
// dashboard_sales_charts_test.go; di sini hanya memastikan id LAMA era
// pre-BL-98 (chart-pipeline, tanpa "-sales-") tak pernah muncul lagi.
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

// TestDashboardBL98_AdminSlimSectionsAndLinks: admin (semua kapabilitas) melihat
// keempat section ramping — tepat 2 KPI terpilih per-domain + tautan Report
// tiap domain. Chart di sini HANYA memeriksa id LAMA era pre-BL-98 tak pernah
// kembali (chart-pipeline dst, tanpa "-sales-"/"-sub-"/"-cs-") — chart BARU
// BL-140..143 (chart-sales-*, dst.) sengaja TIDAK dicek di sini, diuji di file
// _charts_test.go masing-masing domain.
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

	// Donut health global tetap ada.
	if !strings.Contains(body, "chart-health") {
		t.Errorf("donut health global harus tetap ada, body:\n%s", body)
	}
	// Id chart LAMA era pre-BL-98 (tanpa "-sales-"/"-sub-"/"-cs-") tak pernah
	// kembali — BL-140..143 sengaja pakai id BARU (chart-sales-pipeline dst.),
	// bukan menghidupkan kembali fungsi *ChartOption satu-satu lama.
	for _, chart := range []string{
		"chart-pipeline", "chart-leads", "chart-mrr-movement",
		"chart-revenue-plan", "chart-onboarding", "chart-tickets-priority",
		"chart-agent-workload",
	} {
		if strings.Contains(body, chart) {
			t.Errorf("id chart LAMA %q (pre-BL-98) tak boleh muncul lagi", chart)
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
