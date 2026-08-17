package handler

import (
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"
)

// reports_subscriptions_test.go — Subscription Report (Modul 8 M8-1) di sisi
// handler. Empat sumbu dijaga:
//
//   - F2 (gerbang): tanpa izin crm:reports read → 403 + penjelasan.
//   - Section: ?section=renewal (default) memakai jendela TETAP "due"
//     (reuse ListRenewals); ?section=churn memakai ListChurned.
//   - F3 (ownership): sales own-scope hanya melihat baris miliknya sendiri.
//   - Export "no silent caps": CSV export mengumpulkan SEMUA halaman keyset,
//     dibuktikan dgn seed > pageSize baris lalu hitung baris CSV = total seed.
//
// Setup/helper reuse accounts_test.go + seedRenewalSub/seedChurnedSub (sudah
// ada dari M5-4, package sama).

func reportsSubscriptionsReq(section string) *http.Request {
	target := "/w/test/reports/subscriptions"
	if section != "" {
		target += "?section=" + section
	}
	return accountsReq(http.MethodGet, target, nil, "")
}

func reportsSubscriptionsExportReq(section string) *http.Request {
	target := "/w/test/reports/subscriptions/export"
	if section != "" {
		target += "?section=" + section
	}
	return accountsReq(http.MethodGet, target, nil, "")
}

// --- F2: gerbang ---------------------------------------------------------

// TestReportsSubscriptions_GateRead: anggota tanpa business_role (izin crm:
// reports tak dimiliki) → 403 + penjelasan.
func TestReportsSubscriptions_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)

	rec := env.runAccount(uid, "member", "", reportsSubscriptionsReq(""), env.h.ReportsSubscriptions)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Subscription Report") {
		t.Errorf("body 403 harus menyebut judul halaman, got:\n%s", rec.Body.String())
	}
}

// TestReportsSubscriptions_GateRead_AllowedRoles: sales/manager/admin
// (pemegang crm:reports read) → 200.
func TestReportsSubscriptions_GateRead_AllowedRoles(t *testing.T) {
	env, uid := setupAccounts(t)
	for _, role := range []string{"sales", "manager", "admin"} {
		t.Run("role="+role, func(t *testing.T) {
			rec := env.runAccount(uid, "owner", role, reportsSubscriptionsReq(""), env.h.ReportsSubscriptions)
			if rec.Code != http.StatusOK {
				t.Errorf("role %q: status = %d, want 200\n%s", role, rec.Code, rec.Body.String())
			}
		})
	}
}

// --- Section: renewal (default, jendela "due") ---------------------------

// TestReportsSubscriptions_RenewalDefault: tanpa ?section= → tab renewal,
// hanya renewal jatuh tempo (jendela "due") yang muncul.
func TestReportsSubscriptions_RenewalDefault(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	due := env.seedAccount(t, "Desa Report Due", &uid, nil, nil)
	far := env.seedAccount(t, "Desa Report Far", &uid, nil, nil)
	pDue := env.seedPlan(t, "Plan Report Due", "PL-RPD", "1000000")
	pFar := env.seedPlan(t, "Plan Report Far", "PL-RPF", "1000000")
	env.seedRenewalSub(t, due.ID, pDue, &uid, "Active", now.AddDate(0, 0, 10), "", "Manual")
	env.seedRenewalSub(t, far.ID, pFar, &uid, "Active", now.AddDate(0, 0, 60), "", "Manual")

	rec := env.runAccount(uid, "owner", "manager", reportsSubscriptionsReq(""), env.h.ReportsSubscriptions)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Report Due") {
		t.Error("default section (renewal, jendela due) harus memuat Desa Report Due")
	}
	if strings.Contains(body, "Desa Report Far") {
		t.Error("jendela due TAK boleh memuat renewal jauh (Desa Report Far)")
	}
}

// --- Section: churn --------------------------------------------------------

// TestReportsSubscriptions_Churn: ?section=churn hanya memuat langganan
// berhenti, bukan renewal aktif.
func TestReportsSubscriptions_Churn(t *testing.T) {
	env, uid := setupAccounts(t)
	gone := env.seedAccount(t, "Desa Report Gone", &uid, nil, nil)
	pGone := env.seedPlan(t, "Plan Report Gone", "PL-RPG", "1000000")
	env.seedChurnedSub(t, gone.ID, pGone, &uid, "Voluntary", "500000")

	rec := env.runAccount(uid, "owner", "manager", reportsSubscriptionsReq("churn"), env.h.ReportsSubscriptions)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Report Gone") {
		t.Error("section churn harus memuat langganan berhenti (Desa Report Gone)")
	}
}

// --- F3: ownership -----------------------------------------------------------

// TestReportsSubscriptions_ScopedByOwnership: sales (own-scope) hanya
// melihat renewal miliknya; manager (all-scope) lintas-owner.
func TestReportsSubscriptions_ScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "otherreport@local", "member", 0).ID
	now := time.Now()
	mine := env.seedAccount(t, "Desa Report Mine", &uid, nil, nil)
	theirs := env.seedAccount(t, "Desa Report Theirs", &other, nil, nil)
	pM := env.seedPlan(t, "Plan Report Mine", "PL-RPM", "1000000")
	pT := env.seedPlan(t, "Plan Report Theirs", "PL-RPT", "1000000")
	env.seedRenewalSub(t, mine.ID, pM, &uid, "Active", now.AddDate(0, 0, 10), "", "Manual")
	env.seedRenewalSub(t, theirs.ID, pT, &other, "Active", now.AddDate(0, 0, 10), "", "Manual")

	rec := env.runAccount(uid, "owner", "sales", reportsSubscriptionsReq("renewal"), env.h.ReportsSubscriptions)
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Report Mine") {
		t.Error("sales harus melihat renewal miliknya (Desa Report Mine)")
	}
	if strings.Contains(body, "Desa Report Theirs") {
		t.Error("sales TAK boleh melihat renewal milik anggota lain (Desa Report Theirs)")
	}

	recM := env.runAccount(uid, "owner", "manager", reportsSubscriptionsReq("renewal"), env.h.ReportsSubscriptions)
	bodyM := recM.Body.String()
	if !strings.Contains(bodyM, "Desa Report Mine") || !strings.Contains(bodyM, "Desa Report Theirs") {
		t.Error("manager (all-scope) harus melihat semua renewal")
	}
}

// --- Export CSV: no silent caps ---------------------------------------------

// TestReportsSubscriptions_Export_GateRead: export tanpa izin → 403 (bukan
// CSV bocor).
func TestReportsSubscriptions_Export_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)
	rec := env.runAccount(uid, "member", "", reportsSubscriptionsExportReq(""), env.h.ReportsSubscriptionsExport)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}
}

// TestReportsSubscriptions_Export_Churn: export churn 200 + Content-Type
// text/csv + baris CSV sesuai data yang diseed.
func TestReportsSubscriptions_Export_Churn(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Report Export", &uid, nil, nil)
	plan := env.seedPlan(t, "Plan Report Export", "PL-RPE", "1000000")
	env.seedChurnedSub(t, acc.ID, plan, &uid, "Voluntary", "500000")

	rec := env.runAccount(uid, "owner", "admin", reportsSubscriptionsExportReq("churn"), env.h.ReportsSubscriptionsExport)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/csv; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "Desa Report Export") {
		t.Errorf("baris CSV harus memuat Desa Report Export, body:\n%s", rec.Body.String())
	}
}

// TestReportsSubscriptions_Export_NoSilentCaps: seed > pageSize renewal
// jatuh-tempo, export CSV harus memuat SEMUA baris (bukan cuma page pertama
// keyset) — CLAUDE.md rule "no silent caps".
func TestReportsSubscriptions_Export_NoSilentCaps(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	total := pageSize + 5
	for i := 0; i < total; i++ {
		acc := env.seedAccount(t, "Desa Export Bulk "+strconv.Itoa(i), &uid, nil, nil)
		plan := env.seedPlan(t, "Plan Export Bulk "+strconv.Itoa(i), "PL-EXB"+strconv.Itoa(i), "1000000")
		env.seedRenewalSub(t, acc.ID, plan, &uid, "Active", now.AddDate(0, 0, 10), "", "Manual")
	}

	rec := env.runAccount(uid, "owner", "manager", reportsSubscriptionsExportReq("renewal"), env.h.ReportsSubscriptionsExport)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	lines := strings.Split(strings.TrimRight(rec.Body.String(), "\n"), "\n")
	dataLines := len(lines) - 1 // minus header
	if dataLines != total {
		t.Errorf("baris data CSV = %d, want %d (export tak boleh terpotong ke page pertama)", dataLines, total)
	}
}
