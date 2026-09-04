package handler

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// reports_sales_filter_test.go — filter Periode + Tim(owner) Sales Report
// (BL-49) di sisi handler. Empat jaminan inti:
//
//   - Periode memotong KOLOM YANG BENAR per panel: Pipeline/KPI by created_at,
//     Forecast by expected_close_date (bukan created_at) — dibuktikan dengan
//     rentang Kustom deterministik (tak bergantung "hari ini").
//   - Owner(Tim) menyempit DI ATAS F3: manager scope_all bisa memilih owner
//     mana pun; sales own-scope memilih owner lain → NOL (is_own menang, tak
//     pernah membocorkan deal owner lain).
//   - CSV tersaring IDENTIK HTML (satu sumber angka: filter dioper ke kedua
//     jalur).
//   - Param kosong = perilaku BL-43 (tanpa saring).
//
// Koneksi test = superuser (bypass RLS); backdate created_at ditulis mentah
// lewat pool. Helper reuse reports_sales_test.go & reports_sales_panels_test.go.

// reportsSalesBodyQ = ReportsSales dengan query string filter mentah (mis.
// "?period=custom&start=2026-09-01&end=2026-09-30").
func (e *testEnv) reportsSalesBodyQ(t *testing.T, uid int64, tenantRole, bizRole, query string) (int, string) {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/reports/sales"+query, nil, "")
	rec := e.runAccount(uid, tenantRole, bizRole, req, e.h.ReportsSales)
	return rec.Code, rec.Body.String()
}

// exportPanelCSVQ = ReportsSalesExport dengan query string mentah (panel+filter).
func (e *testEnv) exportPanelCSVQ(t *testing.T, uid int64, bizRole, query string) (int, string) {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/reports/sales/export"+query, nil, "")
	rec := e.runAccount(uid, "owner", bizRole, req, e.h.ReportsSalesExport)
	return rec.Code, rec.Body.String()
}

// backdateDealCreatedAt menulis created_at deal ke masa lampau (default DB =
// now()); pool = superuser, bypass RLS.
func (e *testEnv) backdateDealCreatedAt(t *testing.T, dealID int64, at time.Time) {
	t.Helper()
	if _, err := e.h.Pool.Exec(t.Context(),
		`UPDATE deals SET created_at = $1 WHERE id = $2`, at, dealID); err != nil {
		t.Fatalf("backdate deal %d: %v", dealID, err)
	}
}

// --- Periode: kolom tanggal yang benar per panel -----------------------------

// TestReportsSalesFilter_PipelineByCreatedAt: panel Pipeline/KPI memotong
// created_at. Deal dibuat created_at Maret; jendela Maret memuatnya, jendela
// September tidak.
func TestReportsSalesFilter_PipelineByCreatedAt(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Periode Pipeline", &uid, nil, nil)
	d := env.seedReportDeal(t, acc.ID, &uid, "Prospecting", "3000000")
	env.backdateDealCreatedAt(t, d.ID, ymd(2026, 3, 10))

	const label = "Pipeline Terbuka"
	_, in := env.reportsSalesBodyQ(t, uid, "owner", "admin", "?period=custom&start=2026-03-01&end=2026-03-31")
	if got := dashboardKPIValue(t, in, label); got != "1" {
		t.Errorf("jendela Maret memuat deal created_at Maret: %s = %q, want \"1\"", label, got)
	}
	_, out := env.reportsSalesBodyQ(t, uid, "owner", "admin", "?period=custom&start=2026-09-01&end=2026-09-30")
	if got := dashboardKPIValue(t, out, label); got != "0" {
		t.Errorf("jendela September mengecualikan deal created_at Maret: %s = %q, want \"0\"", label, got)
	}
}

// TestReportsSalesFilter_ForecastByExpectedClose: panel Forecast memotong
// expected_close_date, BUKAN created_at (persyaratan inti BL-49). Deal
// created_at Januari, expected_close_date September: jendela September
// menampilkannya; jendela Januari (menutup created_at, bukan expected_close)
// TIDAK — bukti filter memakai kolom yang tepat.
func TestReportsSalesFilter_ForecastByExpectedClose(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Periode Forecast", &uid, nil, nil)
	d := env.seedForecastDeal(t, acc.ID, &uid, "Prospecting", "20000000", 50, ymd(2026, 9, 10))
	env.backdateDealCreatedAt(t, d.ID, ymd(2026, 1, 5))

	// Diskriminator = pesan kosong panel Forecast (spesifik panel, tak
	// terpolusi echo tanggal Kustom di input). "2026-09" tak dipakai: input
	// date Kustom meng-echo-nya ke value attribute apa pun isinya.
	const fcEmpty = "Belum ada deal terbuka dengan tanggal perkiraan tutup"

	// Jendela September menutup expected_close_date (Sep) → Forecast terisi.
	_, sep := env.reportsSalesBodyQ(t, uid, "owner", "admin", "?period=custom&start=2026-09-01&end=2026-09-30")
	if strings.Contains(sep, fcEmpty) {
		t.Errorf("jendela September (expected_close_date) harus mengisi Forecast, malah kosong, body:\n%s", sep)
	}
	// Jendela Januari menutup created_at (Jan) TAPI BUKAN expected_close_date
	// (Sep) → Forecast HARUS kosong. Jika ia salah memakai created_at, panel
	// terisi dan pesan kosong hilang — itu yang kita cegah.
	_, jan := env.reportsSalesBodyQ(t, uid, "owner", "admin", "?period=custom&start=2026-01-01&end=2026-01-31")
	if !strings.Contains(jan, fcEmpty) {
		t.Errorf("jendela Januari menutup created_at, TAPI Forecast pakai expected_close_date → HARUS kosong, body:\n%s", jan)
	}
}

// --- Tim(owner): menyempit DI ATAS F3 ----------------------------------------

// TestReportsSalesFilter_OwnerNarrowsManager: manager scope_all bisa menyaring
// ke owner tertentu. Dua deal (uid + other); tanpa filter = 2, filter ke other
// = 1.
func TestReportsSalesFilter_OwnerNarrowsManager(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "owner-narrow@local", "member", 0).ID
	accMine := env.seedAccount(t, "Desa Mine Narrow", &uid, nil, nil)
	accOther := env.seedAccount(t, "Desa Other Narrow", &other, nil, nil)
	env.seedReportDeal(t, accMine.ID, &uid, "Prospecting", "1000000")
	env.seedReportDeal(t, accOther.ID, &other, "Prospecting", "1000000")

	const label = "Pipeline Terbuka"
	_, all := env.reportsSalesBodyQ(t, uid, "owner", "manager", "")
	if got := dashboardKPIValue(t, all, label); got != "2" {
		t.Errorf("manager tanpa filter: %s = %q, want \"2\"", label, got)
	}
	_, filt := env.reportsSalesBodyQ(t, uid, "owner", "manager", "?owner="+strconv.FormatInt(other, 10))
	if got := dashboardKPIValue(t, filt, label); got != "1" {
		t.Errorf("manager filter owner=other: %s = %q, want \"1\"", label, got)
	}
}

// TestReportsSalesFilter_OwnerDoesNotExpandSales: sales own-scope memilih owner
// lain TAK melebarkan cakupan — F3 menang (hasil NOL, bukan deal owner lain).
// Memilih dirinya sendiri = deal miliknya.
func TestReportsSalesFilter_OwnerDoesNotExpandSales(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "owner-expand@local", "member", 0).ID
	accMine := env.seedAccount(t, "Desa Mine Expand", &uid, nil, nil)
	accOther := env.seedAccount(t, "Desa Other Expand", &other, nil, nil)
	env.seedReportDeal(t, accMine.ID, &uid, "Prospecting", "1000000")
	env.seedReportDeal(t, accOther.ID, &other, "Prospecting", "1000000")

	const label = "Pipeline Terbuka"
	_, foreign := env.reportsSalesBodyQ(t, uid, "owner", "sales", "?owner="+strconv.FormatInt(other, 10))
	if got := dashboardKPIValue(t, foreign, label); got != "0" {
		t.Errorf("sales own-scope filter owner=other: %s = %q, want \"0\" (F3 menang, tak bocor)", label, got)
	}
	_, own := env.reportsSalesBodyQ(t, uid, "owner", "sales", "?owner="+strconv.FormatInt(uid, 10))
	if got := dashboardKPIValue(t, own, label); got != "1" {
		t.Errorf("sales filter owner=dirinya: %s = %q, want \"1\"", label, got)
	}
}

// --- CSV identik HTML --------------------------------------------------------

// TestReportsSalesFilter_CSVMatchesHTML: CSV per-panel tersaring dengan filter
// yang SAMA seperti HTML (satu sumber angka). Deal in-window (Maret) tampil di
// keduanya; deal out-window (Agustus) absen di keduanya.
func TestReportsSalesFilter_CSVMatchesHTML(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa CSV Filter", &uid, nil, nil)
	din := env.seedReportDeal(t, acc.ID, &uid, "Demo", "5000000")
	dout := env.seedReportDeal(t, acc.ID, &uid, "Proposal", "9000000")
	env.backdateDealCreatedAt(t, din.ID, ymd(2026, 3, 10))
	env.backdateDealCreatedAt(t, dout.ID, ymd(2026, 8, 10))

	const window = "period=custom&start=2026-03-01&end=2026-03-31"

	code, csv := env.exportPanelCSVQ(t, uid, "admin", "?panel=pipeline&"+window)
	if code != http.StatusOK {
		t.Fatalf("CSV status = %d, want 200", code)
	}
	if !strings.Contains(csv, "Demo,1,5000000") {
		t.Errorf("CSV harus memuat deal in-window (Demo), body:\n%s", csv)
	}
	if strings.Contains(csv, "9000000") {
		t.Errorf("CSV tak boleh memuat deal out-window (Proposal 9jt), body:\n%s", csv)
	}

	_, html := env.reportsSalesBodyQ(t, uid, "owner", "admin", "?"+window)
	if !strings.Contains(html, "Rp 5.000.000") {
		t.Errorf("HTML harus memuat deal in-window (Rp 5.000.000), body:\n%s", html)
	}
	if strings.Contains(html, "Rp 9.000.000") {
		t.Errorf("HTML tak boleh memuat deal out-window (Rp 9.000.000), body:\n%s", html)
	}
}

// --- Param kosong = BL-43 ----------------------------------------------------

// TestReportsSalesFilter_EmptyParamsNoFilter: tanpa query, semua data dalam
// cakupan terhitung (perilaku BL-43 tak berubah).
func TestReportsSalesFilter_EmptyParamsNoFilter(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Tanpa Filter", &uid, nil, nil)
	d1 := env.seedReportDeal(t, acc.ID, &uid, "Prospecting", "1000000")
	env.seedReportDeal(t, acc.ID, &uid, "Prospecting", "1000000")
	env.backdateDealCreatedAt(t, d1.ID, ymd(2025, 1, 1)) // jauh di masa lampau

	_, body := env.reportsSalesBodyQ(t, uid, "owner", "admin", "")
	if got := dashboardKPIValue(t, body, "Pipeline Terbuka"); got != "2" {
		t.Errorf("tanpa filter: Pipeline Terbuka = %q, want \"2\" (lintas seluruh waktu)", got)
	}
}

// --- Dropdown Tim hanya untuk scope_all --------------------------------------

// TestReportsSalesFilter_OwnerDropdownScopeOnly: dropdown Tim dirender untuk
// pemakai scope_all (manager) tapi disembunyikan untuk own-scope (sales) —
// dropdown owner menyesatkan bila F3 mengunci pemakai ke dirinya.
func TestReportsSalesFilter_OwnerDropdownScopeOnly(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Dropdown", &uid, nil, nil)
	env.seedReportDeal(t, acc.ID, &uid, "Prospecting", "1000000")

	_, mgr := env.reportsSalesBodyQ(t, uid, "owner", "manager", "")
	if !strings.Contains(mgr, "Semua Tim") {
		t.Errorf("manager scope_all harus melihat dropdown Tim, body:\n%s", mgr)
	}
	_, sales := env.reportsSalesBodyQ(t, uid, "owner", "sales", "")
	if strings.Contains(sales, "Semua Tim") {
		t.Errorf("sales own-scope TAK boleh melihat dropdown Tim, body:\n%s", sales)
	}
	// Dropdown Periode tetap ada untuk keduanya.
	if !strings.Contains(sales, "Bulan Ini") {
		t.Errorf("dropdown Periode harus ada untuk semua pemakai, body:\n%s", sales)
	}
}
