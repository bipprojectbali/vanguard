package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// reports_sales_test.go — Sales Report (Modul 8 M8-1) di sisi handler. Tiga
// sumbu dijaga:
//
//   - F2 (gerbang): tanpa izin crm:reports read → 403 + penjelasan (bukan
//     fail-soft seperti Beranda — Report BUKAN halaman default semua anggota).
//   - Agregasi: ReportPipelineByStage memuat SEMUA stage termasuk Closed
//     Won/Lost (beda sengaja dari chart Beranda yang mengecualikan keduanya).
//   - F3 (ownership): sales own-scope hanya menghitung deal miliknya sendiri;
//     manager all-scope lintas-owner. Dibuktikan lewat OpenCount (deal biasa,
//     tak ada masking F4 di Sales Report — nilai apa adanya bagi siapa saja
//     yang lolos F2).
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS
// sesungguhnya diuji di rls_test.go. Setup/helper reuse accounts_test.go.

// seedReportDeal menaruh satu deal dengan stage & amount tertentu — jangkar
// agregasi ReportPipelineByStage tanpa merangkai create per-stage manual.
func (e *testEnv) seedReportDeal(t *testing.T, accountID int64, owner *int64, stage, amount string) db.Deal {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityDeal)
	if err != nil {
		t.Fatalf("generate deal code: %v", err)
	}
	d, err := e.q.CreateDeal(t.Context(), db.CreateDealParams{
		TenantID:   e.tenantID,
		EntityCode: &code,
		DealName:   "Deal Report",
		AccountID:  accountID,
		DealOwner:  owner,
		Stage:      stage,
		Amount:     numFrom(t, amount),
		CreatedBy:  owner,
	})
	if err != nil {
		t.Fatalf("seed report deal: %v", err)
	}
	return d
}

// reportsSalesBody menjalankan ReportsSales sebagai (uid, tenantRole,
// businessRole) & mengembalikan (status, body).
func (e *testEnv) reportsSalesBody(t *testing.T, uid int64, tenantRole, businessRole string) (int, string) {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/reports/sales", nil, "")
	rec := e.runAccount(uid, tenantRole, businessRole, req, e.h.ReportsSales)
	return rec.Code, rec.Body.String()
}

// --- F2: gerbang ---------------------------------------------------------

// TestReportsSales_GateRead: anggota tanpa business_role (izin crm:reports
// tak dimiliki) → 403 + penjelasan, bukan fail-soft seperti Beranda.
func TestReportsSales_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)

	code, body := env.reportsSalesBody(t, uid, "member", "")
	if code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", code)
	}
	if !strings.Contains(body, "Sales Report") {
		t.Errorf("body 403 harus menyebut judul halaman, got:\n%s", body)
	}
}

// TestReportsSales_GateRead_AllowedRoles: sales/manager/admin (pemegang
// crm:reports read) → 200.
func TestReportsSales_GateRead_AllowedRoles(t *testing.T) {
	env, uid := setupAccounts(t)
	for _, role := range []string{"sales", "manager", "admin"} {
		t.Run("role="+role, func(t *testing.T) {
			code, _ := env.reportsSalesBody(t, uid, "owner", role)
			if code != http.StatusOK {
				t.Errorf("role %q: status = %d, want 200", role, code)
			}
		})
	}
}

// --- agregasi --------------------------------------------------------------

// TestReportsSales_IncludesClosedStages: tabel per-stage memuat Closed
// Won/Lost — beda sengaja dari chart pipeline Beranda (yang mengecualikan
// keduanya untuk funnel).
func TestReportsSales_IncludesClosedStages(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Report", &uid, nil, nil)
	env.seedReportDeal(t, acc.ID, &uid, "Prospecting", "1000000")
	env.seedReportDeal(t, acc.ID, &uid, "Closed Won", "2000000")
	env.seedReportDeal(t, acc.ID, &uid, "Closed Lost", "3000000")

	_, body := env.reportsSalesBody(t, uid, "owner", "admin")
	for _, stage := range []string{"Prospecting", "Closed Won", "Closed Lost"} {
		if !strings.Contains(body, ">"+stage+"<") {
			t.Errorf("tabel harus memuat stage %q, body:\n%s", stage, body)
		}
	}
	if !strings.Contains(body, "Rp 2.000.000") {
		t.Errorf("nilai stage Closed Won harus tampil, body:\n%s", body)
	}
}

// --- F3: ownership -----------------------------------------------------------

// TestReportsSales_ScopedByOwnership: sales (own-scope) hanya melihat deal
// miliknya sendiri di kartu Pipeline Terbuka; manager (all-scope) lintas-owner.
func TestReportsSales_ScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "otherowner-report@local", "member", 0).ID
	accMine := env.seedAccount(t, "Desa Milikku Report", &uid, nil, nil)
	accOther := env.seedAccount(t, "Desa Orang Report", &other, nil, nil)

	env.seedReportDeal(t, accMine.ID, &uid, "Prospecting", "1000000")
	env.seedReportDeal(t, accOther.ID, &other, "Prospecting", "5000000")

	const label = "Pipeline Terbuka"
	_, own := env.reportsSalesBody(t, uid, "owner", "sales")
	if got := dashboardKPIValue(t, own, label); got != "1" {
		t.Errorf("sales own-scope: pipeline terbuka = %q, want \"1\" (hanya miliknya)", got)
	}

	_, all := env.reportsSalesBody(t, uid, "owner", "manager")
	if got := dashboardKPIValue(t, all, label); got != "2" {
		t.Errorf("manager all-scope: pipeline terbuka = %q, want \"2\" (lintas-owner)", got)
	}
}

// --- Export CSV --------------------------------------------------------------

// TestReportsSales_Export: status 200, Content-Type text/csv, baris CSV
// sesuai stage yang diseed.
func TestReportsSales_Export(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Export", &uid, nil, nil)
	env.seedReportDeal(t, acc.ID, &uid, "Demo", "7500000")

	req := accountsReq(http.MethodGet, "/w/test/reports/sales/export", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.ReportsSalesExport)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Demo,1,7500000") {
		t.Errorf("baris CSV Demo tak sesuai, body:\n%s", body)
	}
}

// TestReportsSales_Export_GateRead: export tanpa izin → 403 (bukan CSV bocor).
func TestReportsSales_Export_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)
	req := accountsReq(http.MethodGet, "/w/test/reports/sales/export", nil, "")
	rec := env.runAccount(uid, "member", "", req, env.h.ReportsSalesExport)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}
