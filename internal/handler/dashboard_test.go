package handler

import (
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"go_starter/internal/codes"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// dashboard_test.go — Beranda ruang kerja (Modul 1, tasks.md M1-3) di sisi
// handler. Empat sumbu dijaga:
//
//   - Agregasi: ARR total hanya menjumlah subscription Active; pipeline
//     mengecualikan Closed Won/Lost; renewal jatuh tempo hanya jendela 30 hari.
//   - F3 (ownership): sales/csm own-scope hanya menghitung baris miliknya
//     sendiri; admin/manager (all-scope) menghitung lintas-owner. Dibuktikan
//     lewat renewal COUNT (bukan ARR — ARR tersamar F4 utk sales/csm).
//   - F4 (masking ARR): admin/manager melihat ARR asli; sales/csm/support
//     melihat penanda flsHidden ("•••") — kebijakan subscriptions, BUKAN deals.
//   - F2 (gerbang) fail-soft: anggota tanpa business_role jatuh ke Placeholder
//     biasa (200, bukan 403) — Beranda tetap terbuka utk semua anggota workspace.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS
// sesungguhnya diuji di rls_test.go. Setup/helper reuse accounts_test.go
// (setupAccounts/accountsReq/runAccount) + seedPlan (sales_quotes_test.go).

// seedDashboardSub menaruh satu subscription langsung lewat pool dengan status/
// ARR/end_date terkontrol — untuk menguji agregasi tanpa merangkai create.
// endDate nil → NULL (tak masuk hitungan renewal manapun).
func (e *testEnv) seedDashboardSub(
	t *testing.T, accountID, planID int64, owner *int64, status, arr string, endDate *time.Time,
) db.Subscription {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntitySubscription)
	if err != nil {
		t.Fatalf("generate subscription code: %v", err)
	}
	end := pgtype.Date{}
	if endDate != nil {
		end = pgtype.Date{Time: *endDate, Valid: true}
	}
	s, err := e.q.CreateSubscription(t.Context(), db.CreateSubscriptionParams{
		TenantID:          e.tenantID,
		EntityCode:        &code,
		SubscriptionOwner: owner,
		AccountID:         accountID,
		PlanID:            planID,
		Status:            status,
		EndDate:           end,
		AutoRenew:         false,
		Mrr:               numFrom(t, "500000"),
		Arr:               numFrom(t, arr),
		CreatedBy:         owner,
	})
	if err != nil {
		t.Fatalf("seed dashboard subscription: %v", err)
	}
	return s
}

// dashboardBody menjalankan WorkspaceHome sebagai (uid, tenantRole, businessRole)
// & mengembalikan body HTML mentah.
func (e *testEnv) dashboardBody(t *testing.T, uid int64, tenantRole, businessRole string) string {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/", nil, "")
	rec := e.runAccount(uid, tenantRole, businessRole, req, e.h.WorkspaceHome)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	return rec.Body.String()
}

// dashboardKPIValue mengekstrak nilai kartu KPI berdasar label (pola pasti
// dashboardKPICard: <p>label</p><p class="...">value</p>) — presisi & tak
// rentan tabrakan digit dgn substring-match longgar di tempat lain di body.
func dashboardKPIValue(t *testing.T, body, label string) string {
	t.Helper()
	re := regexp.MustCompile(`>` + regexp.QuoteMeta(label) + `</p><p class="[^"]*">([^<]*)</p>`)
	m := re.FindStringSubmatch(body)
	if m == nil {
		t.Fatalf("kartu KPI %q tak ditemukan di body:\n%s", label, body)
	}
	return m[1]
}

// --- agregasi ---------------------------------------------------------------

// TestDashboard_ARRTotalSumsActiveOnly: ARR total hanya menjumlah subscription
// berstatus Active; Trial tak ikut terhitung.
func TestDashboard_ARRTotalSumsActiveOnly(t *testing.T) {
	env, uid := setupAccounts(t)
	planA := env.seedPlan(t, "Paket Aktif", "PLAN-ARR-A", "1000000")
	planB := env.seedPlan(t, "Paket Trial", "PLAN-ARR-B", "1000000")
	acc := env.seedAccount(t, "Desa ARR", &uid, nil, nil)
	env.seedDashboardSub(t, acc.ID, planA, &uid, "Active", "6000000", nil)
	env.seedDashboardSub(t, acc.ID, planB, &uid, "Trial", "9999999", nil)

	body := env.dashboardBody(t, uid, "owner", "admin")
	if !strings.Contains(body, "Rp 6.000.000") {
		t.Errorf("ARR total harus Rp 6.000.000 (hanya Active), body:\n%s", body)
	}
	if strings.Contains(body, "9.999.999") {
		t.Error("ARR subscription Trial tak boleh ikut terhitung")
	}
}

// TestDashboard_PipelineExcludesClosedStages: chart pipeline memuat stage
// TERBUKA (Prospecting/Qualification) tapi mengecualikan Closed Won/Lost.
func TestDashboard_PipelineExcludesClosedStages(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Pipeline", &uid, nil, nil)
	env.seedDeal(t, acc.ID, &uid) // stage awal Prospecting

	won, err := env.q.GenerateEntityCode(t.Context(), env.tenantID, codes.EntityDeal)
	if err != nil {
		t.Fatalf("generate entity code: %v", err)
	}
	if _, err := env.q.CreateDeal(t.Context(), db.CreateDealParams{
		TenantID:   env.tenantID,
		EntityCode: &won,
		DealName:   "Deal Menang",
		AccountID:  acc.ID,
		DealOwner:  &uid,
		Stage:      "Closed Won",
		CreatedBy:  &uid,
	}); err != nil {
		t.Fatalf("seed closed deal: %v", err)
	}

	body := env.dashboardBody(t, uid, "owner", "admin")
	if !strings.Contains(body, `"Prospecting"`) {
		t.Errorf("chart pipeline harus memuat stage Prospecting, body:\n%s", body)
	}
	if strings.Contains(body, "Closed Won") {
		t.Error("chart pipeline TAK boleh memuat deal Closed Won")
	}
}

// TestDashboard_RenewalsDueWindow: hanya subscription Active/PendingApproval
// dengan end_date dalam 30 hari ke depan yang terhitung — di luar jendela atau
// status lain tak ikut.
func TestDashboard_RenewalsDueWindow(t *testing.T) {
	env, uid := setupAccounts(t)
	planIn := env.seedPlan(t, "Paket Jatuh Tempo", "PLAN-DUE-IN", "1000000")
	planOut := env.seedPlan(t, "Paket Jauh", "PLAN-DUE-OUT", "1000000")
	planWrong := env.seedPlan(t, "Paket Salah Status", "PLAN-DUE-WS", "1000000")
	acc := env.seedAccount(t, "Desa Renewal", &uid, nil, nil)

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	within := today.AddDate(0, 0, 15)
	outside := today.AddDate(0, 0, 45)

	env.seedDashboardSub(t, acc.ID, planIn, &uid, "Active", "1200000", &within)
	env.seedDashboardSub(t, acc.ID, planOut, &uid, "Active", "9990000", &outside)
	env.seedDashboardSub(t, acc.ID, planWrong, &uid, "Trial", "8880000", &within)

	body := env.dashboardBody(t, uid, "owner", "admin")
	if !strings.Contains(body, "Rp 1.200.000") {
		t.Errorf("ARR renewal jatuh tempo harus Rp 1.200.000 (hanya dalam jendela), body:\n%s", body)
	}
	if strings.Contains(body, "9.990.000") || strings.Contains(body, "8.880.000") {
		t.Error("subscription di luar jendela/status salah tak boleh ikut ARR jatuh tempo")
	}
}

// --- F3: ownership -----------------------------------------------------------

// TestDashboard_RenewalsScopedByOwnership: sales (own-scope) hanya menghitung
// renewal jatuh tempo miliknya sendiri; manager (all-scope) menghitung lintas-
// owner. Dipakai renewal COUNT (bukan ARR) sebagai bukti F3 — ARR pada peran
// sales sengaja tersamar F4 (lihat TestDashboard_ARRMaskedForNonManager), jadi
// tak bisa dipakai membuktikan cakupan kepemilikan.
func TestDashboard_RenewalsScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "otherowner@local", "member", 0).ID
	planMine := env.seedPlan(t, "Paket Milikku", "PLAN-OWN-MINE", "1000000")
	planOther := env.seedPlan(t, "Paket Orang", "PLAN-OWN-OTHER", "1000000")
	accMine := env.seedAccount(t, "Desa Milikku", &uid, nil, nil)
	accOther := env.seedAccount(t, "Desa Orang", &other, nil, nil)

	within := time.Now().UTC().AddDate(0, 0, 10)
	env.seedDashboardSub(t, accMine.ID, planMine, &uid, "Active", "3000000", &within)
	env.seedDashboardSub(t, accOther.ID, planOther, &other, "Active", "5000000", &within)

	const label = "Renewal Jatuh Tempo (30 hari)"
	own := env.dashboardBody(t, uid, "owner", "sales")
	if got := dashboardKPIValue(t, own, label); got != "1" {
		t.Errorf("sales own-scope: renewal jatuh tempo = %q, want \"1\" (hanya miliknya)", got)
	}

	all := env.dashboardBody(t, uid, "owner", "manager")
	if got := dashboardKPIValue(t, all, label); got != "2" {
		t.Errorf("manager all-scope: renewal jatuh tempo = %q, want \"2\" (lintas-owner)", got)
	}
}

// --- F4: masking ARR ----------------------------------------------------------

// TestDashboard_ARRMaskedForNonManager: admin/manager melihat ARR asli;
// sales/csm/support melihat flsHidden — kebijakan subscriptions (bukan deals).
func TestDashboard_ARRMaskedForNonManager(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket Mask", "PLAN-MASK", "1000000")
	acc := env.seedAccount(t, "Desa Mask", &uid, nil, nil)
	env.seedDashboardSub(t, acc.ID, planID, &uid, "Active", "4000000", nil)

	cases := []struct {
		role     string
		wantReal bool
		wantHide bool
	}{
		{"admin", true, false},
		{"manager", true, false},
		{"sales", false, true},
		{"csm", false, true},
		{"support", false, true},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			body := env.dashboardBody(t, uid, "owner", c.role)
			if c.wantReal && !strings.Contains(body, "Rp 4.000.000") {
				t.Errorf("role %q harus melihat ARR asli, body:\n%s", c.role, body)
			}
			if c.wantHide && !strings.Contains(body, flsHidden) {
				t.Errorf("role %q harus melihat ARR tersamar (%s)", c.role, flsHidden)
			}
			if c.wantHide && strings.Contains(body, "Rp 4.000.000") {
				t.Errorf("role %q TAK boleh melihat ARR asli", c.role)
			}
		})
	}
}

// --- F2: gerbang fail-soft ----------------------------------------------------

// TestDashboard_NoBusinessRoleFallsBackToPlaceholder: anggota tanpa business_role
// (mis. baru diundang) tetap 200 tapi jatuh ke Placeholder biasa — bukan 403,
// karena Beranda harus terbuka utk semua anggota workspace.
func TestDashboard_NoBusinessRoleFallsBackToPlaceholder(t *testing.T) {
	env, uid := setupAccounts(t)

	body := env.dashboardBody(t, uid, "member", "")
	if strings.Contains(body, "ARR Total") {
		t.Error("tanpa business_role TAK boleh melihat kartu KPI Beranda")
	}
	if !strings.Contains(body, "Selamat datang di") {
		t.Errorf("tanpa business_role harus tetap melihat Placeholder sapaan, body:\n%s", body)
	}
}
