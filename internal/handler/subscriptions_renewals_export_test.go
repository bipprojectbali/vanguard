package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// subscriptions_renewals_export_test.go — ekspor CSV dasbor Renewals (BL-94) +
// render header dasbor (KPI/banner/tombol/subtitle/badge derivasi). CSV memakai
// jalur ListRenewals SAMA dgn tabel → gerbang F2 read, F3 ownership, dan filter
// jendela diuji identik pola churn export (BL-92).

// TestSubscriptionRenewalsExport_CSV: content-type text/csv + header kolom + baris
// ter-scope. Sales (own-scope) hanya dapat miliknya.
func TestSubscriptionRenewalsExport_CSV(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other@local", "member", 0).ID
	now := time.Now().In(appTZ)
	mine := env.seedAccount(t, "Desa Mine", &uid, nil, nil)
	theirs := env.seedAccount(t, "Desa Theirs", &other, nil, nil)
	pM := env.seedPlan(t, "Plan Mine", "PL-MINE", "1000000")
	pT := env.seedPlan(t, "Plan Theirs", "PL-THR", "1000000")
	env.seedRenewalSub(t, mine.ID, pM, &uid, "Active", now.AddDate(0, 0, 10), "", "Manual")
	env.seedRenewalSub(t, theirs.ID, pT, &other, "Active", now.AddDate(0, 0, 10), "", "Manual")

	req := accountsReq(http.MethodGet, "/w/test/subscriptions/renewals/export?window=due", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.SubscriptionRenewalsExport)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type = %q, want text/csv", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa,Paket,Tgl Perpanjang,Sisa Hari,Jenis,Status,Nilai Sebelumnya,MRR Kini") {
		t.Errorf("header CSV hilang/berubah\n%s", body)
	}
	if !strings.Contains(body, "Desa Mine") {
		t.Error("CSV sales harus memuat renewal miliknya (Desa Mine)")
	}
	if strings.Contains(body, "Desa Theirs") {
		t.Error("CSV sales TAK boleh memuat renewal milik anggota lain (Desa Theirs)")
	}
}

// TestSubscriptionRenewalsExport_GateAndWindow: gerbang read sama (support 403);
// ?window= menyaring baris CSV persis tabel (grace TAK muncul di window due).
func TestSubscriptionRenewalsExport_GateAndWindow(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now().In(appTZ)
	due := env.seedAccount(t, "Desa DueX", &uid, nil, nil)
	grace := env.seedAccount(t, "Desa GraceX", &uid, nil, nil)
	pDue := env.seedPlan(t, "Plan DueX", "PL-DUEX", "1000000")
	pGrace := env.seedPlan(t, "Plan GraceX", "PL-GRCX", "1000000")
	env.seedRenewalSub(t, due.ID, pDue, &uid, "Active", now.AddDate(0, 0, 10), "", "Manual")
	env.seedRenewalSub(t, grace.ID, pGrace, &uid, "Active", now.AddDate(0, 0, -5), "", "Manual")

	// Support tanpa objek crm:subscriptions → 403.
	reqDeny := accountsReq(http.MethodGet, "/w/test/subscriptions/renewals/export", nil, "")
	if rec := env.runAccount(uid, "owner", "support", reqDeny, env.h.SubscriptionRenewalsExport); rec.Code != http.StatusForbidden {
		t.Errorf("support harus 403, got %d", rec.Code)
	}

	// window=due → hanya Desa DueX di CSV.
	reqDue := accountsReq(http.MethodGet, "/w/test/subscriptions/renewals/export?window=due", nil, "")
	rec := env.runAccount(uid, "owner", "manager", reqDue, env.h.SubscriptionRenewalsExport)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa DueX") || strings.Contains(body, "Desa GraceX") {
		t.Error("CSV window=due harus hanya Desa DueX")
	}
}

// TestSubscriptionRenewals_DashboardHeader: dasbor merender elemen BL-94 —
// subtitle, banner info netral, tombol Ekspor CSV, 4 label KPI, dan badge Status
// derivasi (bukan lifecycle). due (Active +10 hari) → badge "Akan Jatuh Tempo".
func TestSubscriptionRenewals_DashboardHeader(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now().In(appTZ)
	due := env.seedAccount(t, "Desa Header", &uid, nil, nil)
	pDue := env.seedPlan(t, "Plan Header", "PL-HDR", "1000000")
	env.seedRenewalSub(t, due.ID, pDue, &uid, "Active", now.AddDate(0, 0, 10), "", "Manual")

	rec := env.runAccount(uid, "owner", "manager", renewalsReq("due"), env.h.SubscriptionRenewals)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	wants := []string{
		"Langganan mendekati / melewati tanggal perpanjangan", // subtitle
		"Ekspor CSV", // tombol ekspor
		"/subscriptions/renewals/export?window=due", // href ekspor ikut jendela
		"Akan Jatuh Tempo (30 Hari)",                // KPI 1
		"Masa Tenggang",                             // KPI 2
		"Diperpanjang",                              // KPI 3
		"Renewal Rate",                              // KPI 4
		"badge badge-warning",                       // badge derivasi due
	}
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("dasbor renewals harus memuat %q", w)
		}
	}
}
