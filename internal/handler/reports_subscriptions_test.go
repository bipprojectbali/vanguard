package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"go_starter/internal/codes"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// reports_subscriptions_test.go — Subscription Report 8.4 (Modul 8, BL-47) di
// sisi handler. Sumbu yang dijaga:
//
//   - F2 (gerbang): tanpa izin crm:reports read → 403 + penjelasan.
//   - Panel: kelima panel (MRR/ARR · Renewal · Churn · Revenue by Plan · Aging)
//     + 4 KPI benar-benar dirender; label churn Indonesia terpetakan.
//   - F3 (ownership): sales own-scope hanya mengagregasi langganan miliknya;
//     manager all-scope lintas-owner (dibuktikan lewat nama paket panel 4).
//   - F4 (masking): builder memasking nilai Rp bagi role tanpa akses ARR.
//   - Dilewatkan: elemen yang sengaja tak dibangun (delta "vs bulan lalu") tak
//     muncul di HTML.
//   - Export: CSV per-panel 200 + text/csv; gerbang F2 sama.
//
// Setup/helper reuse accounts_test.go + seedRenewalSub/seedChurnedSub/seedPlan
// (package sama). seedActiveSubStart (bawah) menambah start_date — dibutuhkan
// panel Aging & komponen MRR baru yang helper lama biarkan NULL.

// seedActiveSubStart menyeed langganan Active ber-start_date & MRR/ARR eksplisit
// (helper renewal/churn lama tak set start_date). end_date jauh di depan agar
// tak masuk jendela jatuh-tempo; dipakai panel Revenue-by-Plan, Aging, & MRR
// baru (start_date bulan ini tanpa previous_subscription_id).
func (e *testEnv) seedActiveSubStart(
	t *testing.T, accountID, planID int64, owner *int64, mrr string, start time.Time,
) db.Subscription {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntitySubscription)
	if err != nil {
		t.Fatalf("generate subscription code: %v", err)
	}
	s, err := e.q.CreateSubscription(t.Context(), db.CreateSubscriptionParams{
		TenantID:          e.tenantID,
		EntityCode:        &code,
		SubscriptionOwner: owner,
		AccountID:         accountID,
		PlanID:            planID,
		Status:            "Active",
		StartDate:         pgtype.Date{Time: start, Valid: true},
		EndDate:           pgtype.Date{Time: start.AddDate(1, 0, 0), Valid: true},
		AutoRenew:         false,
		Mrr:               numFrom(t, mrr),
		Arr:               numFrom(t, "6000000"),
		CreatedBy:         owner,
	})
	if err != nil {
		t.Fatalf("seed active subscription: %v", err)
	}
	return s
}

func reportsSubscriptionsReq() *http.Request {
	return accountsReq(http.MethodGet, "/w/test/reports/subscriptions", nil, "")
}

func reportsSubscriptionsExportReq(panelKey string) *http.Request {
	target := "/w/test/reports/subscriptions/export"
	if panelKey != "" {
		target += "?panel=" + panelKey
	}
	return accountsReq(http.MethodGet, target, nil, "")
}

// --- F2: gerbang -----------------------------------------------------------

// TestReportsSubscriptions_GateRead: anggota tanpa business_role (izin crm:
// reports tak dimiliki) → 403 + penjelasan.
func TestReportsSubscriptions_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)

	rec := env.runAccount(uid, "member", "", reportsSubscriptionsReq(), env.h.ReportsSubscriptions)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Subscription Report") {
		t.Errorf("body 403 harus menyebut judul halaman, got:\n%s", rec.Body.String())
	}
}

// TestReportsSubscriptions_GateRead_AllowedRoles: sales/manager/admin (pemegang
// crm:reports read) → 200.
func TestReportsSubscriptions_GateRead_AllowedRoles(t *testing.T) {
	env, uid := setupAccounts(t)
	for _, role := range []string{"sales", "manager", "admin"} {
		t.Run("role="+role, func(t *testing.T) {
			rec := env.runAccount(uid, "owner", role, reportsSubscriptionsReq(), env.h.ReportsSubscriptions)
			if rec.Code != http.StatusOK {
				t.Errorf("role %q: status = %d, want 200\n%s", role, rec.Code, rec.Body.String())
			}
		})
	}
}

// --- Panel: kelima panel dirender ------------------------------------------

// TestReportsSubscriptions_PanelsRender: seed data lintas panel, lalu pastikan
// judul kelima panel, label KPI, nama paket (bukan hardcode), & label churn
// Indonesia (terpetakan dari picklist Inggris) muncul.
func TestReportsSubscriptions_PanelsRender(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()

	active := env.seedAccount(t, "Desa Aktif", &uid, nil, nil)
	pActive := env.seedPlan(t, "Paket Emas", "PL-EMAS", "1000000")
	env.seedActiveSubStart(t, active.ID, pActive, &uid, "500000", now)

	due := env.seedAccount(t, "Desa Jatuh Tempo", &uid, nil, nil)
	pDue := env.seedPlan(t, "Paket Perak", "PL-PERAK", "1000000")
	env.seedRenewalSub(t, due.ID, pDue, &uid, "Active", now.AddDate(0, 0, 10), "", "Manual")

	gone := env.seedAccount(t, "Desa Berhenti", &uid, nil, nil)
	pGone := env.seedPlan(t, "Paket Perunggu", "PL-PRG", "1000000")
	env.seedChurnedSub(t, gone.ID, pGone, &uid, "Voluntary", "500000")

	rec := env.runAccount(uid, "owner", "manager", reportsSubscriptionsReq(), env.h.ReportsSubscriptions)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	for _, want := range []string{
		"MRR/ARR Report", "Renewal Report", "Churn Report", "Revenue by Plan",
		"Subscription Aging",         // judul panel
		"Renewal Rate", "Churn Rate", // KPI
		"Paket Emas",            // nama paket panel 4 (dari plans, bukan hardcode)
		"Anggaran tidak lanjut", // label churn Indonesia (Budget → …)
		"&lt; 6 bulan",          // bucket aging (start_date = now), &lt; = escape "<"
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body Subscription Report harus memuat %q", want)
		}
	}
}

// TestReportsSubscriptions_SkippedNotRendered: elemen yang SENGAJA dilewatkan
// BL-47 (delta "vs bulan lalu") tak boleh muncul di HTML.
func TestReportsSubscriptions_SkippedNotRendered(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Skip", &uid, nil, nil)
	plan := env.seedPlan(t, "Paket Skip", "PL-SKIP", "1000000")
	env.seedActiveSubStart(t, acc.ID, plan, &uid, "500000", time.Now())

	rec := env.runAccount(uid, "owner", "manager", reportsSubscriptionsReq(), env.h.ReportsSubscriptions)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "vs bulan lalu") {
		t.Error("delta 'vs bulan lalu' sengaja dilewatkan (tak ada snapshot MRR historis) — tak boleh dirender")
	}
}

// --- F3: ownership ----------------------------------------------------------

// TestReportsSubscriptions_ScopedByOwnership: sales (own-scope) hanya
// mengagregasi langganan miliknya (paket sendiri di Revenue-by-Plan); manager
// (all-scope) lintas-owner.
func TestReportsSubscriptions_ScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "otherreport@local", "member", 0).ID
	now := time.Now()

	mine := env.seedAccount(t, "Desa Milikku", &uid, nil, nil)
	theirs := env.seedAccount(t, "Desa Orang", &other, nil, nil)
	pMine := env.seedPlan(t, "Paket Milikku", "PL-MINE", "1000000")
	pTheirs := env.seedPlan(t, "Paket Orang", "PL-THEIRS", "1000000")
	env.seedActiveSubStart(t, mine.ID, pMine, &uid, "500000", now)
	env.seedActiveSubStart(t, theirs.ID, pTheirs, &other, "700000", now)

	rec := env.runAccount(uid, "owner", "sales", reportsSubscriptionsReq(), env.h.ReportsSubscriptions)
	body := rec.Body.String()
	if !strings.Contains(body, "Paket Milikku") {
		t.Error("sales harus mengagregasi paket miliknya (Paket Milikku)")
	}
	if strings.Contains(body, "Paket Orang") {
		t.Error("sales TAK boleh melihat paket milik anggota lain (Paket Orang)")
	}

	recM := env.runAccount(uid, "owner", "manager", reportsSubscriptionsReq(), env.h.ReportsSubscriptions)
	bodyM := recM.Body.String()
	if !strings.Contains(bodyM, "Paket Milikku") || !strings.Contains(bodyM, "Paket Orang") {
		t.Error("manager (all-scope) harus mengagregasi semua paket")
	}
}

// --- F4: masking (builder-level) -------------------------------------------

// TestReportsSubscriptions_MaskingF4: nilai Rp (Revenue-by-Plan & komponen MRR)
// tersamar bagi role tanpa akses ARR (canSeeARR=false, mis. "support") tetapi
// utuh bagi "admin". Diuji di level builder (pola reports_cs_test).
func TestReportsSubscriptions_MaskingF4(t *testing.T) {
	planRows := []db.ReportRevenueByPlanRow{
		{PlanName: "Paket Emas", VillageCount: 2, Mrr: numFrom(t, "1000000")},
	}
	masked := buildRevenueByPlan(planRows, "support")
	if len(masked) != 1 {
		t.Fatalf("buildRevenueByPlan(support) len = %d, want 1", len(masked))
	}
	if masked[0].MRR != flsHidden || masked[0].AvgPer != flsHidden {
		t.Errorf("MRR/AvgPer tersamar = %q/%q, want %q", masked[0].MRR, masked[0].AvgPer, flsHidden)
	}
	seen := buildRevenueByPlan(planRows, "admin")
	if !strings.Contains(seen[0].MRR, "1.000.000") {
		t.Errorf("admin harus melihat MRR Rp 1.000.000, got %q", seen[0].MRR)
	}
	if !strings.Contains(seen[0].AvgPer, "500.000") {
		t.Errorf("admin harus melihat Rata per Desa Rp 500.000, got %q", seen[0].AvgPer)
	}

	mrrRow := db.ReportSubMRRRow{
		NewMrr: numFrom(t, "500000"), NewCount: 1,
	}
	comps := buildMRRComponents(mrrRow, "support")
	if comps[0].Value != flsHidden {
		t.Errorf("komponen MRR support = %q, want %q (tersamar)", comps[0].Value, flsHidden)
	}
	compsAdmin := buildMRRComponents(mrrRow, "admin")
	if !strings.Contains(compsAdmin[0].Value, "500.000") {
		t.Errorf("admin harus melihat MRR Baru Rp 500.000, got %q", compsAdmin[0].Value)
	}
}

// --- Export CSV ------------------------------------------------------------

// TestReportsSubscriptions_Export_GateRead: export tanpa izin → 403 (bukan CSV
// bocor).
func TestReportsSubscriptions_Export_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)
	rec := env.runAccount(uid, "member", "", reportsSubscriptionsExportReq(""), env.h.ReportsSubscriptionsExport)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

// TestReportsSubscriptions_Export_Panels: tiap panel meng-export CSV 200 +
// Content-Type text/csv dengan header kolom yang benar.
func TestReportsSubscriptions_Export_Panels(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	acc := env.seedAccount(t, "Desa Export", &uid, nil, nil)
	plan := env.seedPlan(t, "Paket Export", "PL-EXP", "1000000")
	env.seedActiveSubStart(t, acc.ID, plan, &uid, "500000", now)
	gone := env.seedAccount(t, "Desa Export Churn", &uid, nil, nil)
	pGone := env.seedPlan(t, "Paket Export Churn", "PL-EXC", "1000000")
	env.seedChurnedSub(t, gone.ID, pGone, &uid, "Voluntary", "500000")

	cases := []struct {
		panel  string
		header string
	}{
		{"mrr", "Komponen MRR"},
		{"renewal", "Jatuh Tempo"},
		{"churn", "Nilai Hilang"},
		{"plan", "Rata per Desa"},
		{"aging", "Kelompok Umur"},
	}
	for _, c := range cases {
		t.Run("panel="+c.panel, func(t *testing.T) {
			rec := env.runAccount(uid, "owner", "admin", reportsSubscriptionsExportReq(c.panel), env.h.ReportsSubscriptionsExport)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", rec.Code)
			}
			if ct := rec.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
				t.Errorf("Content-Type = %q", ct)
			}
			if !strings.Contains(rec.Body.String(), c.header) {
				t.Errorf("CSV panel %q harus memuat header %q, body:\n%s", c.panel, c.header, rec.Body.String())
			}
		})
	}
}
