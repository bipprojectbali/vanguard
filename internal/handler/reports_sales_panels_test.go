package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// reports_sales_panels_test.go — 5 panel Sales Report 8.1 (BL-43) di sisi
// handler: agregasi tiap panel benar, F3 ownership menyaring panel Leads &
// Activity, F4 maskARR tetap aktif di CSV panel Forecast, dan CSV per-panel
// (?panel=forecast|winloss|funnel|activity) memuat kolom & baris yang tepat.
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS
// diuji di rls_test.go. Helper reuse accounts_test.go/helper_test.go.

func ymd(y int, m time.Month, day int) time.Time {
	return time.Date(y, m, day, 0, 0, 0, 0, time.UTC)
}

// seedForecastDeal = deal dengan probability + expected_close_date (jangkar
// panel Pipeline weighted & panel Forecast bucket).
func (e *testEnv) seedForecastDeal(t *testing.T, accID int64, owner *int64, stage, amount string, prob int16, close time.Time) db.Deal {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityDeal)
	if err != nil {
		t.Fatalf("generate deal code: %v", err)
	}
	p := prob
	d, err := e.q.CreateDeal(t.Context(), db.CreateDealParams{
		TenantID:          e.tenantID,
		EntityCode:        &code,
		DealName:          "Deal FC " + stage,
		AccountID:         accID,
		DealOwner:         owner,
		Stage:             stage,
		Amount:            numFrom(t, amount),
		Probability:       &p,
		ExpectedCloseDate: pgtype.Date{Time: close, Valid: true},
		CreatedBy:         owner,
	})
	if err != nil {
		t.Fatalf("seed forecast deal: %v", err)
	}
	return d
}

// setDealLostReason menempel win_loss_reason (teks bebas 00009) pada deal Closed
// Lost — kolom tak ada di CreateDeal, ditulis mentah lewat pool.
func (e *testEnv) setDealLostReason(t *testing.T, dealID int64, reason string) {
	t.Helper()
	if _, err := e.h.Pool.Exec(t.Context(),
		`UPDATE deals SET win_loss_reason = $1 WHERE id = $2`, reason, dealID); err != nil {
		t.Fatalf("set lost reason: %v", err)
	}
}

// seedLeadStatus = satu lead dengan status tertentu milik owner (jangkar funnel).
func (e *testEnv) seedLeadStatus(t *testing.T, name string, owner int64, status string) db.Lead {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityLead)
	if err != nil {
		t.Fatalf("generate lead code: %v", err)
	}
	l, err := e.q.CreateLead(t.Context(), db.CreateLeadParams{
		TenantID:   e.tenantID,
		EntityCode: &code,
		LeadName:   name,
		LeadOwner:  &owner,
		LeadStatus: status,
		CreatedBy:  &owner,
	})
	if err != nil {
		t.Fatalf("seed lead %s: %v", name, err)
	}
	return l
}

// markLeadConverted menandai lead terkonversi ke deal (converted + deal_id +
// converted_at + status) — CreateLead tak menerima kolom konversi.
func (e *testEnv) markLeadConverted(t *testing.T, leadID, dealID int64) {
	t.Helper()
	if _, err := e.h.Pool.Exec(t.Context(),
		`UPDATE leads SET converted = true, converted_deal_id = $1, converted_at = now(), lead_status = 'Converted' WHERE id = $2`,
		dealID, leadID); err != nil {
		t.Fatalf("mark lead converted: %v", err)
	}
}

// --- Panel 1: Pipeline (weighted + probability) ------------------------------

// TestReportsSales_PipelinePanel_WeightedProbability: kolom Probability &
// Weighted dihitung dari amount×probability/100. Deal 10jt @50% → weighted 5jt.
func TestReportsSales_PipelinePanel_WeightedProbability(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Weighted", &uid, nil, nil)
	env.seedForecastDeal(t, acc.ID, &uid, "Prospecting", "10000000", 50, ymd(2026, 9, 10))

	_, body := env.reportsSalesBody(t, uid, "owner", "admin")
	if !strings.Contains(body, "Rp 5.000.000") {
		t.Errorf("weighted 10jt×50%% = Rp 5.000.000 harus tampil, body:\n%s", body)
	}
	if !strings.Contains(body, "50%") {
		t.Errorf("probability 50%% harus tampil, body:\n%s", body)
	}
}

// --- Panel 2: Forecast (bucket bulan, exclude Closed) ------------------------

// TestReportsSales_ForecastPanel_Buckets: forecast dikelompokkan per bulan dari
// expected_close_date, nilai tertimbang; deal Closed Won DIKECUALIKAN.
func TestReportsSales_ForecastPanel_Buckets(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Forecast", &uid, nil, nil)
	env.seedForecastDeal(t, acc.ID, &uid, "Prospecting", "20000000", 50, ymd(2026, 5, 15))  // weighted 10jt → Mei
	env.seedForecastDeal(t, acc.ID, &uid, "Qualification", "8000000", 25, ymd(2026, 9, 10)) // weighted 2jt → Sep
	env.seedForecastDeal(t, acc.ID, &uid, "Closed Won", "40000000", 100, ymd(2026, 7, 1))   // dikecualikan

	_, body := env.reportsSalesBody(t, uid, "owner", "admin")
	for _, want := range []string{"Mei 2026", "Sep 2026", "Rp 10.000.000", "Rp 2.000.000"} {
		if !strings.Contains(body, want) {
			t.Errorf("forecast harus memuat %q, body:\n%s", want, body)
		}
	}
	if strings.Contains(body, "Jul 2026") {
		t.Errorf("forecast harus mengecualikan deal Closed Won (Jul 2026), body:\n%s", body)
	}
}

// --- Panel 3: Win/Loss (persen + alasan) -------------------------------------

// TestReportsSales_WinLossPanel: kartu Menang/Kalah dari won/lost count; tabel
// Alasan Kalah GROUP BY win_loss_reason dengan Porsi% dihitung di Go.
func TestReportsSales_WinLossPanel(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa WinLoss", &uid, nil, nil)
	env.seedReportDeal(t, acc.ID, &uid, "Closed Won", "1000000")
	l1 := env.seedReportDeal(t, acc.ID, &uid, "Closed Lost", "1000000")
	l2 := env.seedReportDeal(t, acc.ID, &uid, "Closed Lost", "1000000")
	l3 := env.seedReportDeal(t, acc.ID, &uid, "Closed Lost", "1000000")
	env.setDealLostReason(t, l1.ID, "Harga Terlalu Tinggi")
	env.setDealLostReason(t, l2.ID, "Harga Terlalu Tinggi")
	env.setDealLostReason(t, l3.ID, "Kurang Fitur")

	_, body := env.reportsSalesBody(t, uid, "owner", "admin")
	// Menang 1/4 = 25%, Kalah 3/4 = 75%.
	for _, want := range []string{"25%", "75%", "Harga Terlalu Tinggi", "66%", "Kurang Fitur", "33%"} {
		if !strings.Contains(body, want) {
			t.Errorf("panel Win/Loss harus memuat %q, body:\n%s", want, body)
		}
	}
}

// --- Panel 4: Lead Conversion (funnel) ---------------------------------------

// TestReportsSales_FunnelPanel: corong Lead→Terkualifikasi→Jadi Deal→Menang.
// 4 lead (2 Qualified + 1 Converted terkualifikasi = 3), 1 terkonversi deal,
// 1 deal menang.
func TestReportsSales_FunnelPanel(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Funnel", &uid, nil, nil)
	env.seedLeadStatus(t, "Lead A", uid, "Qualified")
	env.seedLeadStatus(t, "Lead B", uid, "Qualified")
	conv := env.seedLeadStatus(t, "Lead C", uid, "Contacted")
	env.seedLeadStatus(t, "Lead D", uid, "New")
	won := env.seedReportDeal(t, acc.ID, &uid, "Closed Won", "1000000")
	env.markLeadConverted(t, conv.ID, won.ID)

	_, body := env.reportsSalesBody(t, uid, "owner", "admin")
	// Lead Masuk 4 (100%), Terkualifikasi 3 (75%), Jadi Deal 1 (25%), Menang 1 (25%).
	for _, want := range []string{"4 · 100%", "3 · 75%", "1 · 25%"} {
		if !strings.Contains(body, want) {
			t.Errorf("funnel harus memuat %q, body:\n%s", want, body)
		}
	}
}

// --- Panel 5: Sales Activity (per-owner + Aktivitas/Deal) ---------------------

// TestReportsSales_ActivityPanel_PerDeal: aktivitas sales per-owner + Deal
// Menang → Aktivitas/Deal. 6 aktivitas, 2 deal menang → 3.0 per deal.
func TestReportsSales_ActivityPanel_PerDeal(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Activity", &uid, nil, nil)
	env.seedSalesActivity(t, "call", "account", acc.ID, "C1", &uid)
	env.seedSalesActivity(t, "call", "account", acc.ID, "C2", &uid)
	env.seedSalesActivity(t, "call", "account", acc.ID, "C3", &uid)
	env.seedSalesActivity(t, "email", "account", acc.ID, "E1", &uid)
	env.seedSalesActivity(t, "email", "account", acc.ID, "E2", &uid)
	env.seedSalesActivity(t, "meeting", "account", acc.ID, "M1", &uid)
	env.seedReportDeal(t, acc.ID, &uid, "Closed Won", "1000000")
	env.seedReportDeal(t, acc.ID, &uid, "Closed Won", "1000000")

	_, body := env.reportsSalesBody(t, uid, "owner", "admin")
	if strings.Contains(body, "Belum ada aktivitas sales") {
		t.Fatalf("panel aktivitas tak boleh kosong, body:\n%s", body)
	}
	if !strings.Contains(body, "3.0") {
		t.Errorf("Aktivitas/Deal = 6/2 = 3.0 harus tampil, body:\n%s", body)
	}
}

// --- F3: ownership ------------------------------------------------------------

// TestReportsSales_FunnelPanel_ScopedByOwnership: funnel sisi Leads disaring
// data_scope. Sales own-scope hanya menghitung lead miliknya; manager lintas.
func TestReportsSales_FunnelPanel_ScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "otherlead-report@local", "member", 0).ID
	env.seedLeadStatus(t, "Lead Mine", uid, "New")
	env.seedLeadStatus(t, "Lead O1", other, "New")
	env.seedLeadStatus(t, "Lead O2", other, "New")
	env.seedLeadStatus(t, "Lead O3", other, "New")

	_, own := env.reportsSalesBody(t, uid, "owner", "sales")
	if !strings.Contains(own, "1 · 100%") {
		t.Errorf("sales own-scope: Lead Masuk = 1, body:\n%s", own)
	}
	_, all := env.reportsSalesBody(t, uid, "owner", "manager")
	if !strings.Contains(all, "4 · 100%") {
		t.Errorf("manager all-scope: Lead Masuk = 4, body:\n%s", all)
	}
}

// TestReportsSales_ActivityPanel_ScopedByOwnership: panel Activity disaring
// data_scope aktivitas. Sales hanya melihat barisnya, bukan owner lain.
func TestReportsSales_ActivityPanel_ScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "otheract-report@local", "member", 0).ID
	accMine := env.seedAccount(t, "Desa Mine Act", &uid, nil, nil)
	accOther := env.seedAccount(t, "Desa Other Act", &other, nil, nil)
	env.seedSalesActivity(t, "call", "account", accMine.ID, "Act-Mine", &uid)
	env.seedSalesActivity(t, "call", "account", accOther.ID, "Act-Other", &other)

	_, own := env.reportsSalesBody(t, uid, "owner", "sales")
	if strings.Contains(own, "otheract-report@local") {
		t.Errorf("sales own-scope tak boleh melihat owner lain, body:\n%s", own)
	}
	if !strings.Contains(own, "test@local") {
		t.Errorf("sales harus melihat barisnya sendiri, body:\n%s", own)
	}
	_, all := env.reportsSalesBody(t, uid, "owner", "manager")
	if !strings.Contains(all, "otheract-report@local") {
		t.Errorf("manager all-scope harus melihat owner lain, body:\n%s", all)
	}
}

// --- CSV per-panel ------------------------------------------------------------

// exportPanelCSV menjalankan ReportsSalesExport?panel=… → (status, body).
func (e *testEnv) exportPanelCSV(t *testing.T, uid int64, bizRole, panelKey string) (int, string) {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/reports/sales/export?panel="+panelKey, nil, "")
	rec := e.runAccount(uid, "owner", bizRole, req, e.h.ReportsSalesExport)
	return rec.Code, rec.Body.String()
}

// TestReportsSales_Export_Panels: tiap ?panel= menghasilkan header & baris CSV
// yang sesuai agregasi panelnya.
func TestReportsSales_Export_Panels(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa CSV", &uid, nil, nil)
	env.seedForecastDeal(t, acc.ID, &uid, "Prospecting", "20000000", 50, ymd(2026, 9, 10)) // weighted 10jt
	lost := env.seedReportDeal(t, acc.ID, &uid, "Closed Lost", "1000000")
	env.setDealLostReason(t, lost.ID, "Harga")
	env.seedLeadStatus(t, "L1", uid, "Qualified")
	env.seedReportDeal(t, acc.ID, &uid, "Closed Won", "1000000")
	env.seedSalesActivity(t, "call", "account", acc.ID, "C", &uid)

	cases := []struct{ panel, header, row string }{
		{"forecast", "Periode,Forecast", "2026-09,10000000"},
		{"winloss", "Alasan Kalah,Jumlah Deal,Porsi", "Harga,1,100%"},
		{"funnel", "Tahap,Jumlah,Porsi", "Lead Masuk,1,100%"},
		{"activity", "Sales,Panggilan,Email,Meeting,Total,Deal Menang,Aktivitas/Deal", "test@local,1,0,0,1,1,1.0"},
	}
	for _, c := range cases {
		t.Run("panel="+c.panel, func(t *testing.T) {
			code, body := env.exportPanelCSV(t, uid, "admin", c.panel)
			if code != http.StatusOK {
				t.Fatalf("status = %d, want 200", code)
			}
			if !strings.Contains(body, c.header) {
				t.Errorf("header %q tak ada, body:\n%s", c.header, body)
			}
			if !strings.Contains(body, c.row) {
				t.Errorf("baris %q tak ada, body:\n%s", c.row, body)
			}
		})
	}
}

// TestReportsSales_ForecastCSV_NoLeak_Support: Support (data_scope='none' atas
// deals) → CSV forecast tak memuat nilai tertimbang mentah (F3 nol baris +
// maskARR pertahanan-berlapis).
func TestReportsSales_ForecastCSV_NoLeak_Support(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa FC Support", &uid, nil, nil)
	env.seedForecastDeal(t, acc.ID, &uid, "Prospecting", "20000000", 50, ymd(2026, 9, 10))

	code, body := env.exportPanelCSV(t, uid, "support", "forecast")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200", code)
	}
	if strings.Contains(body, "10000000") {
		t.Errorf("support TAK BOLEH melihat nilai forecast mentah, body:\n%s", body)
	}
}
