package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// reports_subscriptions_filter_test.go — filter Periode + Paket Subscription
// Report (BL-52) di sisi handler. Jaminan inti (pola BL-49 Sales / BL-50 CS /
// BL-51 Support):
//
//   - Periode memotong KOLOM YANG BENAR per panel, BUKAN seragam: Renewal by
//     end_date & Churn by cancellation_date (dibuktikan satu langganan renewal
//     end_date Januari + satu churn cancellation_date Maret — hanya jendela yang
//     tepat memuat panel yang tepat; jika seragam, salah satu tak akan pernah
//     cocok).
//   - Panel SNAPSHOT (MRR/ARR berjalan, Revenue-by-Plan, Aging) = nilai SEKARANG
//     tanpa dimensi waktu → Periode TAK berlaku (tetap terisi di jendela lampau).
//   - Paket menyempit DI ATAS F3: aggregate menyaring; sales own-scope memilih
//     paket yang hanya dimiliki desa anggota LAIN → NOL (F3 menang).
//   - CSV tersaring IDENTIK HTML (satu sumber angka lewat reportsSubscriptionsData).
//   - Param kosong = perilaku BL-47 (tanpa saring).
//
// Koneksi test = superuser (bypass RLS); cancellation_date di-backdate mentah
// lewat pool (helper churn menyetel now()). Reuse helper reports_subscriptions_
// test.go (seedActiveSubStart), subscriptions_*_test.go (seedRenewalSub/
// seedChurnedSub), reports_sales_panels_test.go (ymd), dashboard_test.go
// (dashboardKPIValue — cocok utk reportStat: struktur >label</p><p>value</p>).

// reportsSubscriptionsBodyQ = ReportsSubscriptions dengan query string filter.
func (e *testEnv) reportsSubscriptionsBodyQ(t *testing.T, uid int64, tenantRole, bizRole, query string) (int, string) {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/reports/subscriptions"+query, nil, "")
	rec := e.runAccount(uid, tenantRole, bizRole, req, e.h.ReportsSubscriptions)
	return rec.Code, rec.Body.String()
}

// exportSubscriptionPanelCSVQ = ReportsSubscriptionsExport dengan query string.
func (e *testEnv) exportSubscriptionPanelCSVQ(t *testing.T, uid int64, bizRole, query string) (int, string) {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/reports/subscriptions/export"+query, nil, "")
	rec := e.runAccount(uid, "owner", bizRole, req, e.h.ReportsSubscriptionsExport)
	return rec.Code, rec.Body.String()
}

// backdateSubCancellation menyetel cancellation_date langganan (helper churn
// memakai now()); pool = superuser, bypass RLS.
func (e *testEnv) backdateSubCancellation(t *testing.T, subID int64, at time.Time) {
	t.Helper()
	if _, err := e.h.Pool.Exec(t.Context(),
		`UPDATE subscriptions SET cancellation_date = $1 WHERE id = $2`, at, subID); err != nil {
		t.Fatalf("backdate cancellation_date langganan %d: %v", subID, err)
	}
}

// --- Periode: kolom tanggal yang benar per panel -----------------------------

// TestReportsSubscriptionsFilter_RenewalVsChurnColumn: Renewal memotong end_date
// & Churn memotong cancellation_date — kolom BERBEDA (BUKAN seragam). Satu
// langganan renewal ber-end_date Januari + satu churn ber-cancellation_date
// Maret. Jendela Januari memuat baris renewal "Jan 2026" TAPI bukan breakdown
// churn; jendela Maret memuat breakdown churn ("Anggaran tidak lanjut") TAPI
// bukan renewal. Jika keduanya salah dipotong satu kolom seragam, salah satu
// panel tak akan pernah cocok di jendela lain — itu yang dicegah.
func TestReportsSubscriptionsFilter_RenewalVsChurnColumn(t *testing.T) {
	env, uid := setupAccounts(t)

	due := env.seedAccount(t, "Desa Jatuh Tempo Periode", &uid, nil, nil)
	pDue := env.seedPlan(t, "Paket Renewal", "PL-RNW", "1000000")
	env.seedRenewalSub(t, due.ID, pDue, &uid, "Active", ymd(2026, 1, 15), "", "Manual")

	gone := env.seedAccount(t, "Desa Berhenti Periode", &uid, nil, nil)
	pGone := env.seedPlan(t, "Paket Churn", "PL-CHR", "1000000")
	churned := env.seedChurnedSub(t, gone.ID, pGone, &uid, "Voluntary", "500000")
	env.backdateSubCancellation(t, churned.ID, ymd(2026, 3, 20))

	const churnLabel = "Anggaran tidak lanjut" // Budget → label Indonesia

	_, jan := env.reportsSubscriptionsBodyQ(t, uid, "owner", "admin", "?period=custom&start=2026-01-01&end=2026-01-31")
	if !strings.Contains(jan, "Jan 2026") {
		t.Errorf("jendela Januari harus memuat baris renewal end_date Jan 2026, body:\n%s", jan)
	}
	if strings.Contains(jan, churnLabel) {
		t.Errorf("jendela Januari tak boleh memuat breakdown churn (cancellation_date Maret), body:\n%s", jan)
	}

	_, mar := env.reportsSubscriptionsBodyQ(t, uid, "owner", "admin", "?period=custom&start=2026-03-01&end=2026-03-31")
	if !strings.Contains(mar, churnLabel) {
		t.Errorf("jendela Maret harus memuat breakdown churn cancellation_date Maret, body:\n%s", mar)
	}
	if strings.Contains(mar, "Jan 2026") {
		t.Errorf("jendela Maret tak boleh memuat renewal end_date Januari, body:\n%s", mar)
	}
}

// TestReportsSubscriptionsFilter_ChurnRateByCancellationDate: KPI Churn Rate
// (ReportRetention) memotong churned pada cancellation_date; active = SNAPSHOT.
// 1 aktif + 1 churn (cancellation Januari). Jendela Januari: 1/(1+1) = 50,0%;
// jendela September: churned di luar → 0/(1+0) = 0,0% (active tetap snapshot).
func TestReportsSubscriptionsFilter_ChurnRateByCancellationDate(t *testing.T) {
	env, uid := setupAccounts(t)

	active := env.seedAccount(t, "Desa Aktif Churn KPI", &uid, nil, nil)
	pAct := env.seedPlan(t, "Paket Aktif KPI", "PL-AKPI", "1000000")
	env.seedActiveSubStart(t, active.ID, pAct, &uid, "500000", todayInAppTZ())

	gone := env.seedAccount(t, "Desa Churn KPI", &uid, nil, nil)
	pGone := env.seedPlan(t, "Paket Churn KPI", "PL-CKPI", "1000000")
	churned := env.seedChurnedSub(t, gone.ID, pGone, &uid, "Voluntary", "500000")
	env.backdateSubCancellation(t, churned.ID, ymd(2026, 1, 15))

	const label = "Churn Rate"
	_, jan := env.reportsSubscriptionsBodyQ(t, uid, "owner", "admin", "?period=custom&start=2026-01-01&end=2026-01-31")
	if got := dashboardKPIValue(t, jan, label); got != "50,0%" {
		t.Errorf("jendela Januari (churn cancellation_date): %s = %q, want \"50,0%%\"", label, got)
	}
	_, sep := env.reportsSubscriptionsBodyQ(t, uid, "owner", "admin", "?period=custom&start=2026-09-01&end=2026-09-30")
	if got := dashboardKPIValue(t, sep, label); got != "0,0%" {
		t.Errorf("jendela September (churn Januari dikecualikan, active snapshot): %s = %q, want \"0,0%%\"", label, got)
	}
}

// --- Panel snapshot TAK terpengaruh Periode ----------------------------------

// TestReportsSubscriptionsFilter_SnapshotPanelsIgnorePeriod: MRR/ARR berjalan,
// Revenue-by-Plan, & Aging = kondisi SEKARANG tanpa dimensi waktu → Periode TAK
// berlaku. Langganan aktif ber-start_date hari ini tetap tampil di jendela masa
// lampau (2020): kartu MRR berjalan terisi, paket muncul di Revenue-by-Plan, &
// bucket "< 6 bulan" ada di Aging.
func TestReportsSubscriptionsFilter_SnapshotPanelsIgnorePeriod(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Snapshot", &uid, nil, nil)
	plan := env.seedPlan(t, "Paket Snapshot", "PL-SNAP", "1000000")
	env.seedActiveSubStart(t, acc.ID, plan, &uid, "777000", todayInAppTZ())

	_, past := env.reportsSubscriptionsBodyQ(t, uid, "owner", "admin", "?period=custom&start=2020-01-01&end=2020-12-31")
	if got := dashboardKPIValue(t, past, "MRR (berjalan)"); got != "Rp 777.000" {
		t.Errorf("MRR berjalan (snapshot) tak terpengaruh Periode: got %q, want \"Rp 777.000\"", got)
	}
	// Empty-marker ABSEN = Revenue-by-Plan berisi data walau jendela 2020 (bukti
	// snapshot; nama paket sendiri tak dipakai karena selalu muncul di dropdown).
	if strings.Contains(past, "Belum ada langganan aktif ber-paket") {
		t.Errorf("Revenue-by-Plan (snapshot) harus tetap terisi di jendela 2020, body:\n%s", past)
	}
	if !strings.Contains(past, "&lt; 6 bulan") {
		t.Errorf("Aging (snapshot) harus tetap punya bucket \"< 6 bulan\" di jendela 2020, body:\n%s", past)
	}
}

// --- Paket: menyempit aggregate + tak melebarkan F3 --------------------------

// TestReportsSubscriptionsFilter_PaketNarrowsAggregate: Paket menyaring Revenue-
// by-Plan. Manager (all-scope) 2 paket aktif; tanpa filter keduanya tampil,
// plan=A hanya Paket A (Paket B hilang).
func TestReportsSubscriptionsFilter_PaketNarrowsAggregate(t *testing.T) {
	env, uid := setupAccounts(t)
	now := todayInAppTZ()

	accA := env.seedAccount(t, "Desa Paket A", &uid, nil, nil)
	planA := env.seedPlan(t, "Paket Alpha", "PL-ALFA", "1000000")
	env.seedActiveSubStart(t, accA.ID, planA, &uid, "500000", now)

	accB := env.seedAccount(t, "Desa Paket B", &uid, nil, nil)
	planB := env.seedPlan(t, "Paket Beta", "PL-BETA", "1000000")
	env.seedActiveSubStart(t, accB.ID, planB, &uid, "730000", now)

	// Diskriminator = nilai MRR terformat (hanya di baris panel, TAK di dropdown
	// yang selalu memuat kedua nama paket): Alpha "Rp 500.000", Beta "Rp 730.000".
	_, all := env.reportsSubscriptionsBodyQ(t, uid, "owner", "manager", "")
	if !strings.Contains(all, "Rp 500.000") || !strings.Contains(all, "Rp 730.000") {
		t.Errorf("tanpa filter Paket: kedua paket harus tampil, body:\n%s", all)
	}
	_, onlyA := env.reportsSubscriptionsBodyQ(t, uid, "owner", "manager", "?plan="+itoa(planA))
	if !strings.Contains(onlyA, "Rp 500.000") {
		t.Errorf("plan=Alpha: MRR Paket Alpha harus tampil, body:\n%s", onlyA)
	}
	if strings.Contains(onlyA, "Rp 730.000") {
		t.Errorf("plan=Alpha: MRR Paket Beta tak boleh tampil (tersaring), body:\n%s", onlyA)
	}
}

// TestReportsSubscriptionsFilter_PaketDoesNotExpand: Paket di-AND DI ATAS F3 —
// sales own-scope memilih paket yang hanya dimiliki desa anggota LAIN tak
// melebarkan cakupan (F3 menang). Desa milikku Paket Milikku; desa anggota lain
// Paket Orang. sales plan=Orang → Revenue-by-Plan kosong (bukan paket desa lain).
func TestReportsSubscriptionsFilter_PaketDoesNotExpand(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "sub-paket-expand@local", "member", 0).ID
	now := todayInAppTZ()

	mine := env.seedAccount(t, "Desa Milikku Paket", &uid, nil, nil)
	pMine := env.seedPlan(t, "Paket Milikku", "PL-MINE", "1000000")
	env.seedActiveSubStart(t, mine.ID, pMine, &uid, "500000", now)

	theirs := env.seedAccount(t, "Desa Orang Paket", &other, nil, nil)
	pTheirs := env.seedPlan(t, "Paket Orang", "PL-THRS", "1000000")
	env.seedActiveSubStart(t, theirs.ID, pTheirs, &other, "700000", now)

	// Paket Orang hanya di desa anggota lain → F3 menyembunyikannya → kosong.
	_, foreign := env.reportsSubscriptionsBodyQ(t, uid, "owner", "sales", "?plan="+itoa(pTheirs))
	if strings.Contains(foreign, "Paket Orang") {
		t.Errorf("sales plan=Orang (hanya desa lain): Paket Orang tak boleh tampil (F3 menang), body:\n%s", foreign)
	}
	if !strings.Contains(foreign, "Belum ada langganan aktif ber-paket") {
		t.Errorf("sales plan=Orang: Revenue-by-Plan harus kosong, body:\n%s", foreign)
	}
	// Paket Milikku = desa binaannya → baris panel terisi (MRR "Rp 500.000";
	// nama paket sendiri selalu di dropdown, jadi pakai nilai sbg diskriminator).
	_, own := env.reportsSubscriptionsBodyQ(t, uid, "owner", "sales", "?plan="+itoa(pMine))
	if !strings.Contains(own, "Rp 500.000") {
		t.Errorf("sales plan=Milikku (desa binaannya): baris panel Paket Milikku harus tampil, body:\n%s", own)
	}
}

// --- CSV identik HTML --------------------------------------------------------

// TestReportsSubscriptionsFilter_CSVMatchesHTML: CSV Revenue-by-Plan tersaring
// Paket SAMA seperti HTML (satu sumber angka reportsSubscriptionsData). Dua
// paket aktif; filter plan=Terpilih → CSV memuat Paket Terpilih tapi bukan Paket
// Lain, identik HTML.
func TestReportsSubscriptionsFilter_CSVMatchesHTML(t *testing.T) {
	env, uid := setupAccounts(t)
	now := todayInAppTZ()

	accSel := env.seedAccount(t, "Desa CSV Terpilih", &uid, nil, nil)
	planSel := env.seedPlan(t, "Paket Terpilih", "PL-SEL", "1000000")
	env.seedActiveSubStart(t, accSel.ID, planSel, &uid, "500000", now)

	accOth := env.seedAccount(t, "Desa CSV Lain", &uid, nil, nil)
	planOth := env.seedPlan(t, "Paket Lain", "PL-OTH", "1000000")
	env.seedActiveSubStart(t, accOth.ID, planOth, &uid, "610000", now)

	// CSV tak punya dropdown → aman menyaring per NAMA paket.
	q := "?panel=plan&plan=" + itoa(planSel)
	code, csv := env.exportSubscriptionPanelCSVQ(t, uid, "admin", q)
	if code != http.StatusOK {
		t.Fatalf("CSV status = %d, want 200", code)
	}
	if !strings.Contains(csv, "Paket Terpilih") {
		t.Errorf("CSV plan=Terpilih harus memuat Paket Terpilih, body:\n%s", csv)
	}
	if strings.Contains(csv, "Paket Lain") {
		t.Errorf("CSV plan=Terpilih tak boleh memuat Paket Lain (tersaring), body:\n%s", csv)
	}
	// HTML: diskriminator nilai MRR (Terpilih "Rp 500.000", Lain "Rp 610.000") —
	// nama paket sendiri selalu ada di dropdown. Angka identik jalur CSV.
	_, html := env.reportsSubscriptionsBodyQ(t, uid, "owner", "admin", "?plan="+itoa(planSel))
	if !strings.Contains(html, "Rp 500.000") || strings.Contains(html, "Rp 610.000") {
		t.Errorf("HTML plan=Terpilih harus identik CSV (Terpilih ada, Lain tidak), body:\n%s", html)
	}
}

// --- Param kosong = BL-47 ----------------------------------------------------

// TestReportsSubscriptionsFilter_EmptyParamsNoFilter: tanpa query, breakdown
// churn menghitung lintas seluruh waktu (perilaku BL-47). Langganan churn masa
// lampau (2020) tetap terhitung.
func TestReportsSubscriptionsFilter_EmptyParamsNoFilter(t *testing.T) {
	env, uid := setupAccounts(t)
	gone := env.seedAccount(t, "Desa Empty Churn", &uid, nil, nil)
	plan := env.seedPlan(t, "Paket Empty", "PL-EMP", "1000000")
	churned := env.seedChurnedSub(t, gone.ID, plan, &uid, "Voluntary", "500000")
	env.backdateSubCancellation(t, churned.ID, ymd(2020, 5, 1))

	_, body := env.reportsSubscriptionsBodyQ(t, uid, "owner", "admin", "")
	if !strings.Contains(body, "Anggaran tidak lanjut") {
		t.Errorf("tanpa filter: breakdown churn lintas seluruh waktu harus memuat churn 2020, body:\n%s", body)
	}
}

// --- Dropdown filter selalu dirender -----------------------------------------

// TestReportsSubscriptionsFilter_ControlsRendered: dropdown Periode (enum tetap)
// & Paket (diturunkan dari DATA — paket yang benar-benar dipakai) hadir, plus
// opsi "Semua Paket" & catatan snapshot Periode.
func TestReportsSubscriptionsFilter_ControlsRendered(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Kontrol", &uid, nil, nil)
	plan := env.seedPlan(t, "Paket Kontrol", "PL-KTRL", "1000000")
	env.seedActiveSubStart(t, acc.ID, plan, &uid, "500000", todayInAppTZ())

	_, body := env.reportsSubscriptionsBodyQ(t, uid, "owner", "admin", "")
	for _, want := range []string{
		"Semua Waktu", "Bulan Ini", "Kuartal Ini", "Tahun Ini", "Kustom", // Periode
		"Semua Paket", "Paket Kontrol", // Paket (opsi default + dari data)
	} {
		if !strings.Contains(body, want) {
			t.Errorf("filter harus memuat %q, body:\n%s", want, body)
		}
	}
	if !strings.Contains(body, "menampilkan kondisi terkini") {
		t.Errorf("catatan snapshot Periode harus hadir, body:\n%s", body)
	}
}
