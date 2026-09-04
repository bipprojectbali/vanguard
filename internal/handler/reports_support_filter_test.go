package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"go_starter/internal/db"
)

// reports_support_filter_test.go — filter Periode + Prioritas Support Report
// (BL-51) di sisi handler. Jaminan inti (pola BL-49 Sales / BL-50 CS):
//
//   - Periode memotong KOLOM YANG BENAR per panel: Volume Masuk by created_at &
//     Selesai by resolved_at (dibuktikan tiket yang created & resolved di bulan
//     BERBEDA — hanya kolom yang tepat memuatnya di tiap jendela); KPI by
//     created_at; Resolution by resolved_at.
//   - Prioritas menyempit DI ATAS F3: aggregate KPI menyaring; csm own-scope
//     memilih prioritas yang hanya dimiliki desa CSM LAIN → NOL (F3 menang).
//   - DUA pengecualian: baris "bulan ini vs lalu" & panel KB TAK terpengaruh
//     Periode (snapshot) — tetap terisi di jendela masa lampau; Prioritas TETAP
//     menyaring baris bulan.
//   - CSV tersaring IDENTIK HTML (satu sumber angka lewat reportsSupportData).
//   - Param kosong = perilaku BL-46 (tanpa saring).
//
// Koneksi test = superuser (bypass RLS); created_at/resolved_at di-backdate
// mentah lewat pool (CreateTicket tak menerima keduanya). Helper reuse
// reports_support_test.go, tickets_test.go, reports_sales_panels_test.go (ymd).

// reportsSupportBodyQ = ReportsSupport dengan query string filter mentah.
func (e *testEnv) reportsSupportBodyQ(t *testing.T, uid int64, tenantRole, bizRole, query string) (int, string) {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/reports/support"+query, nil, "")
	rec := e.runAccount(uid, tenantRole, bizRole, req, e.h.ReportsSupport)
	return rec.Code, rec.Body.String()
}

// exportSupportPanelCSVQ = ReportsSupportExport dengan query string mentah.
func (e *testEnv) exportSupportPanelCSVQ(t *testing.T, uid int64, bizRole, query string) (int, string) {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/reports/support/export"+query, nil, "")
	rec := e.runAccount(uid, "owner", bizRole, req, e.h.ReportsSupportExport)
	return rec.Code, rec.Body.String()
}

// backdateTicketCreated menyetel created_at tiket (default now()); pool =
// superuser, bypass RLS.
func (e *testEnv) backdateTicketCreated(t *testing.T, ticketID int64, at time.Time) {
	t.Helper()
	if _, err := e.h.Pool.Exec(t.Context(),
		`UPDATE tickets SET created_at = $1 WHERE id = $2`, at, ticketID); err != nil {
		t.Fatalf("backdate created_at tiket %d: %v", ticketID, err)
	}
}

// resolveTicketAt menandai tiket selesai dengan resolved_at spesifik (status =
// 'selesai'); pool bypass RLS. created_at diatur terpisah bila perlu.
func (e *testEnv) resolveTicketAt(t *testing.T, ticketID int64, at time.Time) {
	t.Helper()
	if _, err := e.h.Pool.Exec(t.Context(),
		`UPDATE tickets SET status = 'selesai', resolved_at = $1 WHERE id = $2`, at, ticketID); err != nil {
		t.Fatalf("resolve tiket %d: %v", ticketID, err)
	}
}

// seedTicketPrio membuat tiket dengan prioritas eksplisit (default seedTicketRow
// = 'sedang'); mengembalikan db.Ticket.
func (e *testEnv) seedTicketPrio(t *testing.T, accountID int64, subject, priority string) db.Ticket {
	t.Helper()
	tk, err := e.q.CreateTicket(t.Context(), db.CreateTicketParams{
		TenantID:  e.tenantID,
		AccountID: accountID,
		Subject:   subject,
		Priority:  priority,
	})
	if err != nil {
		t.Fatalf("seed ticket prio %q/%q: %v", subject, priority, err)
	}
	return tk
}

// --- Periode: kolom tanggal yang benar per panel -----------------------------

// TestReportsSupportFilter_VolumeByCreatedResolved: panel Volume memotong Masuk
// pada created_at & Selesai pada resolved_at — kolom BERBEDA. Satu tiket dibuat
// Januari, diselesaikan Maret. Jendela Januari memuat "2026-01" (masuk) TAPI
// bukan "2026-03"; jendela Maret memuat "2026-03" (selesai) TAPI bukan "2026-01".
// Jika keduanya salah dipotong created_at seragam, "2026-03" tak akan pernah
// muncul di jendela Maret — itu yang dicegah.
func TestReportsSupportFilter_VolumeByCreatedResolved(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Volume Periode", &uid, nil, nil)
	tk := env.seedTicketRow(t, acc.ID, "Tiket Lintas Bulan")
	env.backdateTicketCreated(t, tk.ID, ymd(2026, 1, 15))
	env.resolveTicketAt(t, tk.ID, ymd(2026, 3, 20))

	_, jan := env.reportsSupportBodyQ(t, uid, "owner", "admin", "?period=custom&start=2026-01-01&end=2026-01-31")
	if !strings.Contains(jan, "Jan 2026") {
		t.Errorf("jendela Januari harus memuat baris created_at Jan 2026, body:\n%s", jan)
	}
	if strings.Contains(jan, "Mar 2026") {
		t.Errorf("jendela Januari tak boleh memuat resolved_at Mar 2026 (kolom Selesai), body:\n%s", jan)
	}
	_, mar := env.reportsSupportBodyQ(t, uid, "owner", "admin", "?period=custom&start=2026-03-01&end=2026-03-31")
	if !strings.Contains(mar, "Mar 2026") {
		t.Errorf("jendela Maret harus memuat baris resolved_at Mar 2026 (kolom Selesai), body:\n%s", mar)
	}
	if strings.Contains(mar, "Jan 2026") {
		t.Errorf("jendela Maret tak boleh memuat created_at Jan 2026 (kolom Masuk), body:\n%s", mar)
	}
}

// TestReportsSupportFilter_KPIByCreatedAt: Total Tiket (KPI) memotong created_at.
// Tiket dibuat Januari → jendela Januari Total=1, jendela September Total=0.
func TestReportsSupportFilter_KPIByCreatedAt(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa KPI Periode", &uid, nil, nil)
	tk := env.seedTicketRow(t, acc.ID, "Tiket Januari")
	env.backdateTicketCreated(t, tk.ID, ymd(2026, 1, 10))

	const label = "Total Tiket"
	_, jan := env.reportsSupportBodyQ(t, uid, "owner", "admin", "?period=custom&start=2026-01-01&end=2026-01-31")
	if got := dashboardKPIValue(t, jan, label); got != "1" {
		t.Errorf("jendela Januari (created_at): %s = %q, want \"1\"", label, got)
	}
	_, sep := env.reportsSupportBodyQ(t, uid, "owner", "admin", "?period=custom&start=2026-09-01&end=2026-09-30")
	if got := dashboardKPIValue(t, sep, label); got != "0" {
		t.Errorf("jendela September (created_at Januari dikecualikan): %s = %q, want \"0\"", label, got)
	}
}

// --- Prioritas: menyempit aggregate + tak melebarkan F3 ----------------------

// TestReportsSupportFilter_PriorityNarrowsAggregate: Prioritas menyaring KPI
// Total. 1 tinggi + 1 rendah; tanpa filter Total=2, priority=tinggi → 1,
// priority=rendah → 1.
func TestReportsSupportFilter_PriorityNarrowsAggregate(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Prioritas", &uid, nil, nil)
	env.seedTicketPrio(t, acc.ID, "Tiket Tinggi", "tinggi")
	env.seedTicketPrio(t, acc.ID, "Tiket Rendah", "rendah")

	const label = "Total Tiket"
	_, all := env.reportsSupportBodyQ(t, uid, "owner", "admin", "")
	if got := dashboardKPIValue(t, all, label); got != "2" {
		t.Errorf("tanpa prioritas: %s = %q, want \"2\"", label, got)
	}
	_, hi := env.reportsSupportBodyQ(t, uid, "owner", "admin", "?priority=tinggi")
	if got := dashboardKPIValue(t, hi, label); got != "1" {
		t.Errorf("priority=tinggi: %s = %q, want \"1\"", label, got)
	}
	_, lo := env.reportsSupportBodyQ(t, uid, "owner", "admin", "?priority=rendah")
	if got := dashboardKPIValue(t, lo, label); got != "1" {
		t.Errorf("priority=rendah: %s = %q, want \"1\"", label, got)
	}
}

// TestReportsSupportFilter_PriorityDoesNotExpand: Prioritas di-AND DI ATAS F3 —
// csm own-scope memilih prioritas yang hanya dimiliki desa CSM LAIN tak
// melebarkan cakupan (F3 menang). Desa binaan sendiri tiket 'sedang'; desa CSM
// lain tiket 'tinggi'. csm priority=tinggi → Total=0 (bukan tiket desa lain).
func TestReportsSupportFilter_PriorityDoesNotExpand(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "sup-prio-expand@local", "member", 0).ID
	own := env.seedAccount(t, "Desa Binaanku Prio", &uid, nil, nil)
	env.seedTicketPrio(t, own.ID, "Tiket Sedang Milikku", "sedang")
	otherAcc := env.seedAccount(t, "Desa Binaan Lain Prio", nil, &other, nil)
	env.seedTicketPrio(t, otherAcc.ID, "Tiket Tinggi Orang", "tinggi")

	const label = "Total Tiket"
	// Prioritas Tinggi hanya ada di desa CSM lain → F3 menyembunyikannya → 0.
	_, hi := env.reportsSupportBodyQ(t, uid, "member", "csm", "?priority=tinggi")
	if got := dashboardKPIValue(t, hi, label); got != "0" {
		t.Errorf("csm priority=tinggi (hanya di desa lain): %s = %q, want \"0\" (F3 menang)", label, got)
	}
	// Prioritas Sedang = desa binaannya → tampil (menyaring, tak melebarkan).
	_, sed := env.reportsSupportBodyQ(t, uid, "member", "csm", "?priority=sedang")
	if got := dashboardKPIValue(t, sed, label); got != "1" {
		t.Errorf("csm priority=sedang (desa binaannya): %s = %q, want \"1\"", label, got)
	}
}

// --- Pengecualian: month-compare & KB tak terpengaruh Periode -----------------

// TestReportsSupportFilter_MonthCompareIgnoresPeriod: baris "bulan ini vs lalu"
// inheren relatif now() → Periode TAK berlaku. Tiket diselesaikan bulan ini
// (durasi 5 jam). Jendela masa lampau (2020) mengosongkan panel Resolution
// (resolved_at di luar jendela) TAPI baris bulan ini TETAP "5,0 jam". Prioritas
// TETAP menyaring: priority=rendah (tiket 'tinggi') → baris bulan "—".
func TestReportsSupportFilter_MonthCompareIgnoresPeriod(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Bulan Ini", &uid, nil, nil)
	tk := env.seedTicketPrio(t, acc.ID, "Tiket Selesai Bulan Ini", "tinggi")
	now := todayInAppTZ()
	env.backdateTicketCreated(t, tk.ID, now.Add(-5*time.Hour))
	env.resolveTicketAt(t, tk.ID, now)

	thisLabel, _ := resolutionMonthLabels(now)
	const dur = "5,0 jam"

	// Jendela masa lampau: Resolution panel kosong, tapi baris bulan tetap terisi.
	_, past := env.reportsSupportBodyQ(t, uid, "owner", "admin", "?period=custom&start=2020-01-01&end=2020-12-31")
	if got := dashboardKPIValue(t, past, thisLabel); got != dur {
		t.Errorf("baris bulan ini tak terpengaruh Periode: %s = %q, want %q", thisLabel, got, dur)
	}
	if !strings.Contains(past, "Belum ada tiket selesai") {
		t.Errorf("panel Resolution harus kosong di jendela 2020 (resolved_at bulan ini di luar), body:\n%s", past)
	}

	// Prioritas TETAP menyaring baris bulan: rendah mengecualikan tiket 'tinggi'.
	_, lo := env.reportsSupportBodyQ(t, uid, "owner", "admin", "?priority=rendah")
	if got := dashboardKPIValue(t, lo, thisLabel); got != "—" {
		t.Errorf("priority=rendah harus mengosongkan baris bulan (tiket 'tinggi'): %s = %q, want \"—\"", thisLabel, got)
	}
}

// TestReportsSupportFilter_KBIgnoresPeriod: panel KB = snapshot Published tanpa
// dimensi waktu → Periode TAK berlaku. Artikel Published tetap muncul walau
// jendela 2020.
func TestReportsSupportFilter_KBIgnoresPeriod(t *testing.T) {
	env, uid := setupAccounts(t)
	if _, err := env.q.CreateKBArticle(t.Context(), db.CreateKBArticleParams{
		TenantID:     env.tenantID,
		ArticleTitle: "Artikel Snapshot KB",
		Status:       "Published",
		Visibility:   "Internal",
	}); err != nil {
		t.Fatalf("seed kb published: %v", err)
	}

	_, past := env.reportsSupportBodyQ(t, uid, "owner", "admin", "?period=custom&start=2020-01-01&end=2020-12-31")
	if !strings.Contains(past, "Artikel Snapshot KB") {
		t.Errorf("KB snapshot harus tetap muncul di jendela 2020 (tanpa dimensi waktu), body:\n%s", past)
	}
}

// --- CSV identik HTML --------------------------------------------------------

// TestReportsSupportFilter_CSVMatchesHTML: CSV Volume tersaring dengan Periode +
// Prioritas SAMA seperti HTML. Tiket created Januari priority=tinggi: jendela
// Januari+tinggi memuat "2026-01" di CSV & HTML; jendela September mengecualikan.
func TestReportsSupportFilter_CSVMatchesHTML(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa CSV Support", &uid, nil, nil)
	tk := env.seedTicketPrio(t, acc.ID, "Tiket CSV Tinggi", "tinggi")
	env.backdateTicketCreated(t, tk.ID, ymd(2026, 1, 12))

	const janWin = "period=custom&start=2026-01-01&end=2026-01-31&priority=tinggi"
	code, csvJan := env.exportSupportPanelCSVQ(t, uid, "admin", "?panel=volume&"+janWin)
	if code != http.StatusOK {
		t.Fatalf("CSV status = %d, want 200", code)
	}
	if !strings.Contains(csvJan, "Jan 2026") {
		t.Errorf("CSV Januari+tinggi harus memuat 2026-01, body:\n%s", csvJan)
	}
	_, htmlJan := env.reportsSupportBodyQ(t, uid, "owner", "admin", "?"+janWin)
	if !strings.Contains(htmlJan, "Jan 2026") {
		t.Errorf("HTML Januari+tinggi harus memuat 2026-01, body:\n%s", htmlJan)
	}

	const sepWin = "period=custom&start=2026-09-01&end=2026-09-30"
	_, csvSep := env.exportSupportPanelCSVQ(t, uid, "admin", "?panel=volume&"+sepWin)
	if strings.Contains(csvSep, "Jan 2026") {
		t.Errorf("CSV September tak boleh memuat 2026-01 (out-window), body:\n%s", csvSep)
	}
}

// --- Param kosong = BL-46 ----------------------------------------------------

// TestReportsSupportFilter_EmptyParamsNoFilter: tanpa query, Total menghitung
// lintas seluruh waktu (perilaku BL-46 tak berubah). Tiket created jauh lampau
// tetap terhitung.
func TestReportsSupportFilter_EmptyParamsNoFilter(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Empty Support", &uid, nil, nil)
	tk := env.seedTicketRow(t, acc.ID, "Tiket Lampau")
	env.backdateTicketCreated(t, tk.ID, ymd(2020, 5, 1))

	_, body := env.reportsSupportBodyQ(t, uid, "owner", "admin", "")
	if got := dashboardKPIValue(t, body, "Total Tiket"); got != "1" {
		t.Errorf("tanpa filter: Total Tiket = %q, want \"1\" (lintas seluruh waktu)", got)
	}
}

// --- Dropdown filter selalu dirender -----------------------------------------

// TestReportsSupportFilter_ControlsRendered: dropdown Periode & Prioritas selalu
// ada (enum tetap). Catatan snapshot hadir untuk memperjelas Periode tak
// menyentuh baris "bulan ini vs lalu" & KB.
func TestReportsSupportFilter_ControlsRendered(t *testing.T) {
	env, uid := setupAccounts(t)
	for _, role := range []string{"admin", "support"} {
		t.Run("role="+role, func(t *testing.T) {
			_, body := env.reportsSupportBodyQ(t, uid, "owner", role, "")
			for _, want := range []string{"Bulan Ini", "Semua Prioritas", "Tinggi", "Sedang", "Rendah"} {
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
