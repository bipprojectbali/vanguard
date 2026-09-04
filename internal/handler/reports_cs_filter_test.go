package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"go_starter/internal/db"
)

// reports_cs_filter_test.go — filter Periode + Segmen Customer Success Report
// (BL-50) di sisi handler. Empat jaminan inti (pola BL-49 Sales):
//
//   - Periode memotong KOLOM YANG BENAR per panel: Retention/Churn by
//     cancellation_date (BUKAN created_at), Onboarding by kickoff_date —
//     dibuktikan dgn rentang Kustom deterministik (tak bergantung "hari ini").
//     Health/Adopsi/Retention Rate = SNAPSHOT (active tak terpengaruh Periode).
//   - Segmen (band kesehatan) menyempit DI ATAS F3: csm own-scope memilih band
//     yang hanya dimiliki desa CSM lain → NOL (F3 menang, tak membocorkan desa
//     di luar binaan).
//   - CSV tersaring IDENTIK HTML (satu sumber angka: filter dioper ke kedua
//     jalur lewat reportsCSData).
//   - Param kosong = perilaku BL-45 (tanpa saring).
//
// Koneksi test = superuser (bypass RLS); backdate cancellation_date ditulis
// mentah lewat pool. Helper reuse reports_cs_test.go (seedChurnedSub, seedCSRow,
// dll) & reports_sales_panels_test.go (ymd).

// reportsCSBodyQ = ReportsCS dengan query string filter mentah (mis.
// "?period=custom&start=2026-03-01&end=2026-03-31").
func (e *testEnv) reportsCSBodyQ(t *testing.T, uid int64, tenantRole, bizRole, query string) (int, string) {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/reports/customer-success"+query, nil, "")
	rec := e.runAccount(uid, tenantRole, bizRole, req, e.h.ReportsCS)
	return rec.Code, rec.Body.String()
}

// exportCSPanelCSVQ = ReportsCSExport dengan query string mentah (panel+filter).
func (e *testEnv) exportCSPanelCSVQ(t *testing.T, uid int64, bizRole, query string) (int, string) {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/reports/customer-success/export"+query, nil, "")
	rec := e.runAccount(uid, "owner", bizRole, req, e.h.ReportsCSExport)
	return rec.Code, rec.Body.String()
}

// backdateCancellationDate menulis cancellation_date langganan ke tanggal
// tertentu (default seedChurnedSub = now()); pool = superuser, bypass RLS.
func (e *testEnv) backdateCancellationDate(t *testing.T, subID int64, at time.Time) {
	t.Helper()
	if _, err := e.h.Pool.Exec(t.Context(),
		`UPDATE subscriptions SET cancellation_date = $1 WHERE id = $2`, at, subID); err != nil {
		t.Fatalf("backdate cancellation_date sub %d: %v", subID, err)
	}
}

// --- Periode: kolom tanggal yang benar per panel -----------------------------

// TestReportsCSFilter_RetentionByCancellationDate: Retention Rate memotong
// cancellation_date pada churned (BUKAN created_at). 3 aktif (snapshot) + 1
// churn dgn cancellation_date Maret (created_at default = sekarang, Sep).
// Jendela Maret memuat churn (cancellation) → churned=1 → 75%; jendela September
// menutup created_at TAPI BUKAN cancellation → churned=0 → 100%. Jika ia salah
// memakai created_at, angka-angka ini terbalik — itu yang dicegah.
func TestReportsCSFilter_RetentionByCancellationDate(t *testing.T) {
	env, uid := setupAccounts(t)
	plan := env.seedPlan(t, "Paket CS Filter", "PLAN-CSF-1", "1000000")
	// idx_subs_one_active: 1 Active per desa → sebar 3 aktif ke 3 desa.
	for _, nm := range []string{"Desa Aktif F1", "Desa Aktif F2", "Desa Aktif F3"} {
		a := env.seedAccount(t, nm, &uid, nil, nil)
		env.seedSubscription(t, a.ID, plan, &uid, "Active", "1000000", "12000000")
	}
	churnAcc := env.seedAccount(t, "Desa Churn Periode", &uid, nil, nil)
	sub := env.seedChurnedSub(t, churnAcc.ID, plan, &uid, "Voluntary", "500000")
	env.backdateCancellationDate(t, sub.ID, ymd(2026, 3, 15))

	const label = "Retention Rate"
	_, mar := env.reportsCSBodyQ(t, uid, "owner", "admin", "?period=custom&start=2026-03-01&end=2026-03-31")
	if got := dashboardKPIValue(t, mar, label); got != "75,0%" {
		t.Errorf("jendela Maret memuat churn (cancellation_date): %s = %q, want \"75,0%%\"", label, got)
	}
	_, sep := env.reportsCSBodyQ(t, uid, "owner", "admin", "?period=custom&start=2026-09-01&end=2026-09-30")
	if got := dashboardKPIValue(t, sep, label); got != "100,0%" {
		t.Errorf("jendela September mengecualikan churn (bukan created_at): %s = %q, want \"100,0%%\"", label, got)
	}
}

// TestReportsCSFilter_OnboardingByKickoffDate: panel Onboarding memotong
// kickoff_date. Desa onboarding selesai kickoff Januari (durasi 30 hari):
// jendela Januari menampilkan "30,0 hari"; jendela September mengecualikannya.
func TestReportsCSFilter_OnboardingByKickoffDate(t *testing.T) {
	env, uid := setupAccounts(t)
	done := env.seedAccount(t, "Desa Onboard Periode", &uid, nil, nil)
	env.seedCSRow(t, db.CreateCustomerSuccessParams{
		AccountID:        done.ID,
		OnboardingStatus: ptr("Completed"),
		KickoffDate:      csDate(2026, 1, 1),
		TargetGoLiveDate: csDate(2026, 3, 1),
		ActualGoLiveDate: csDate(2026, 1, 31),
	})

	const dur = "30,0 hari"
	_, jan := env.reportsCSBodyQ(t, uid, "owner", "admin", "?period=custom&start=2026-01-01&end=2026-01-31")
	if !strings.Contains(jan, dur) {
		t.Errorf("jendela Januari (kickoff_date) harus menampilkan %q, body:\n%s", dur, jan)
	}
	_, sep := env.reportsCSBodyQ(t, uid, "owner", "admin", "?period=custom&start=2026-09-01&end=2026-09-30")
	if strings.Contains(sep, dur) {
		t.Errorf("jendela September mengecualikan kickoff Januari → %q tak boleh muncul, body:\n%s", dur, sep)
	}
}

// --- Segmen: menyempit DI ATAS F3 --------------------------------------------

// TestReportsCSFilter_SegmentNarrowsHealthBand: Segmen=band menyaring Health KPI
// ke desa dalam band itu. 1 sehat (90) + 1 kritis (30); tanpa segmen avg 60,
// segmen=healthy → 90, segmen=critical → 30.
func TestReportsCSFilter_SegmentNarrowsHealthBand(t *testing.T) {
	env, uid := setupAccounts(t)
	a1 := env.seedAccount(t, "Desa Seg Sehat", &uid, nil, nil)
	env.seedHealthScore(t, a1.ID, 90, "Healthy")
	a2 := env.seedAccount(t, "Desa Seg Kritis", &uid, nil, nil)
	env.seedHealthScore(t, a2.ID, 30, "Critical")

	const label = "Rata Health Score"
	_, all := env.reportsCSBodyQ(t, uid, "owner", "admin", "")
	if got := dashboardKPIValue(t, all, label); got != "60,0" {
		t.Errorf("tanpa segmen: %s = %q, want \"60,0\"", label, got)
	}
	_, healthy := env.reportsCSBodyQ(t, uid, "owner", "admin", "?segment=healthy")
	if got := dashboardKPIValue(t, healthy, label); got != "90,0" {
		t.Errorf("segmen=healthy: %s = %q, want \"90,0\"", label, got)
	}
	_, crit := env.reportsCSBodyQ(t, uid, "owner", "admin", "?segment=critical")
	if got := dashboardKPIValue(t, crit, label); got != "30,0" {
		t.Errorf("segmen=critical: %s = %q, want \"30,0\"", label, got)
	}
}

// TestReportsCSFilter_SegmentDoesNotExpand: Segmen di-AND DI ATAS F3 — csm
// own-scope memilih band yang hanya dimiliki desa CSM LAIN tak melebarkan
// cakupan (F3 menang). Desa binaan sendiri = Sehat (90); desa CSM lain =
// Kritis (20). csm segmen=critical → NOL berskor ("—"), bukan desa CSM lain.
func TestReportsCSFilter_SegmentDoesNotExpand(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "seg-expand-cs@local", "member", 0).ID
	own := env.seedAccount(t, "Desa Binaanku Seg", nil, &uid, nil)
	env.seedHealthScore(t, own.ID, 90, "Healthy")
	otherAcc := env.seedAccount(t, "Desa Binaan Lain Seg", nil, &other, nil)
	env.seedHealthScore(t, otherAcc.ID, 20, "Critical")

	const label = "Rata Health Score"
	// Band Kritis hanya ada di desa CSM lain → F3 menyembunyikannya → "—".
	_, crit := env.reportsCSBodyQ(t, uid, "member", "csm", "?segment=critical")
	if got := dashboardKPIValue(t, crit, label); got != "—" {
		t.Errorf("csm segmen=critical (band hanya di desa lain): %s = %q, want \"—\" (F3 menang)", label, got)
	}
	// Band Sehat = desa binaannya → tampil (segmen menyaring, tak melebarkan).
	_, healthy := env.reportsCSBodyQ(t, uid, "member", "csm", "?segment=healthy")
	if got := dashboardKPIValue(t, healthy, label); got != "90,0" {
		t.Errorf("csm segmen=healthy (desa binaannya): %s = %q, want \"90,0\"", label, got)
	}
}

// --- CSV identik HTML --------------------------------------------------------

// TestReportsCSFilter_CSVMatchesHTML: CSV churn tersaring dengan Periode yang
// SAMA seperti HTML (satu sumber angka). Churn cancellation Maret: jendela Maret
// memuatnya di CSV & HTML; jendela September mengecualikannya di keduanya.
func TestReportsCSFilter_CSVMatchesHTML(t *testing.T) {
	env, uid := setupAccounts(t)
	plan := env.seedPlan(t, "Paket CSV CS", "PLAN-CSVCS-1", "1000000")
	acc := env.seedAccount(t, "Desa CSV Churn", &uid, nil, nil)
	sub := env.seedChurnedSub(t, acc.ID, plan, &uid, "Voluntary", "500000")
	env.backdateCancellationDate(t, sub.ID, ymd(2026, 3, 15))

	// Jendela Maret memuat churn → CSV & HTML sama-sama menampilkannya.
	const marWin = "period=custom&start=2026-03-01&end=2026-03-31"
	code, csvMar := env.exportCSPanelCSVQ(t, uid, "admin", "?panel=retention&"+marWin)
	if code != http.StatusOK {
		t.Fatalf("CSV status = %d, want 200", code)
	}
	if !strings.Contains(csvMar, "Budget") || !strings.Contains(csvMar, "Rp 500.000") {
		t.Errorf("CSV Maret harus memuat churn in-window (Budget/Rp 500.000), body:\n%s", csvMar)
	}
	_, htmlMar := env.reportsCSBodyQ(t, uid, "owner", "admin", "?"+marWin)
	if !strings.Contains(htmlMar, "Rp 500.000") {
		t.Errorf("HTML Maret harus memuat Nilai Hilang Rp 500.000, body:\n%s", htmlMar)
	}

	// Jendela September mengecualikan churn → keduanya kosong.
	const sepWin = "period=custom&start=2026-09-01&end=2026-09-30"
	_, csvSep := env.exportCSPanelCSVQ(t, uid, "admin", "?panel=retention&"+sepWin)
	if strings.Contains(csvSep, "Budget") {
		t.Errorf("CSV September tak boleh memuat churn out-window, body:\n%s", csvSep)
	}
	_, htmlSep := env.reportsCSBodyQ(t, uid, "owner", "admin", "?"+sepWin)
	if !strings.Contains(htmlSep, "Belum ada langganan churn") {
		t.Errorf("HTML September harus kosong (pesan panel), body:\n%s", htmlSep)
	}
}

// --- Param kosong = BL-45 ----------------------------------------------------

// TestReportsCSFilter_EmptyParamsNoFilter: tanpa query, retensi lintas seluruh
// waktu (perilaku BL-45 tak berubah): 3 aktif + 1 churn (cancellation apa pun)
// → 75%.
func TestReportsCSFilter_EmptyParamsNoFilter(t *testing.T) {
	env, uid := setupAccounts(t)
	plan := env.seedPlan(t, "Paket Empty CS", "PLAN-EMPCS-1", "1000000")
	for _, nm := range []string{"Desa Empty A1", "Desa Empty A2", "Desa Empty A3"} {
		a := env.seedAccount(t, nm, &uid, nil, nil)
		env.seedSubscription(t, a.ID, plan, &uid, "Active", "1000000", "12000000")
	}
	churnAcc := env.seedAccount(t, "Desa Empty Churn", &uid, nil, nil)
	sub := env.seedChurnedSub(t, churnAcc.ID, plan, &uid, "Voluntary", "500000")
	env.backdateCancellationDate(t, sub.ID, ymd(2025, 1, 1)) // jauh di masa lampau

	_, body := env.reportsCSBodyQ(t, uid, "owner", "admin", "")
	if got := dashboardKPIValue(t, body, "Retention Rate"); got != "75,0%" {
		t.Errorf("tanpa filter: Retention Rate = %q, want \"75,0%%\" (lintas seluruh waktu)", got)
	}
}

// --- Dropdown filter selalu dirender -----------------------------------------

// TestReportsCSFilter_ControlsRendered: dropdown Periode & Segmen selalu ada
// (enum tetap, tak bergantung scope — beda dgn owner Sales). Catatan snapshot
// hadir untuk memperjelas Periode tak menyentuh Health/Adopsi/Retention Rate.
func TestReportsCSFilter_ControlsRendered(t *testing.T) {
	env, uid := setupAccounts(t)
	for _, role := range []string{"admin", "csm"} {
		t.Run("role="+role, func(t *testing.T) {
			tenantRole := "owner"
			if role == "csm" {
				tenantRole = "member"
			}
			_, body := env.reportsCSBodyQ(t, uid, tenantRole, role, "")
			for _, want := range []string{"Bulan Ini", "Semua Segmen", "Sehat (80–100)", "Kritis ("} {
				if !strings.Contains(body, want) {
					t.Errorf("role %q: filter harus memuat %q, body:\n%s", role, want, body)
				}
			}
			if !strings.Contains(body, "menampilkan kondisi terkini") {
				t.Errorf("role %q: catatan snapshot Periode harus hadir", role)
			}
		})
	}
}
