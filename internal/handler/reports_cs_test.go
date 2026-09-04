package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"go_starter/internal/db"
)

// reports_cs_test.go — Customer Success Report (Modul 8, wireframe 8.2, BL-45) di
// sisi handler. Enam panel spec 8.2, DIBANGUN hanya yang datanya SUDAH ADA; empat
// sumbu dijaga:
//
//   - F2 (gerbang): tanpa izin crm:reports read → 403 + penjelasan.
//   - Agregasi 3 KPI + panel (health band, adoption, retention/churn, onboarding,
//     engagement) konsisten dgn data yang diseed.
//   - F3 (ownership): CSM (own) hanya melihat desa binaannya (Rata Health Score);
//     Admin (all) lintas-CSM.
//   - F4 (masking): Support pemegang crm:reports tapi DI LUAR canSeeARR → Nilai
//     Hilang churn tersamar (•••), tak bocor Rupiah mentah.
//
// Panel NPS/CSAT (8.2 panel 4) TAK dirender (tak ada tabel surveys) — diverifikasi
// TestReportsCS_SkippedPanelsAbsent. Koneksi test = superuser (bypass RLS) → uji
// LOGIKA handler; isolasi RLS di rls_test.go.

// reportsCSBody menjalankan ReportsCS sebagai (uid, tenantRole, businessRole)
// & mengembalikan (status, body).
func (e *testEnv) reportsCSBody(t *testing.T, uid int64, tenantRole, businessRole string) (int, string) {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/reports/customer-success", nil, "")
	rec := e.runAccount(uid, tenantRole, businessRole, req, e.h.ReportsCS)
	return rec.Code, rec.Body.String()
}

// seedCSRow menyisip satu baris customer_success (satu per account) — pembungkus
// tipis CreateCustomerSuccess agar test merakit field yang relevan saja.
func (e *testEnv) seedCSRow(t *testing.T, p db.CreateCustomerSuccessParams) {
	t.Helper()
	p.TenantID = e.tenantID
	if _, err := e.q.CreateCustomerSuccess(t.Context(), p); err != nil {
		t.Fatalf("seed customer_success (account %d): %v", p.AccountID, err)
	}
}

// seedChurnedSub (langganan Churned + churn_reason "Budget" + lost_value_mrr)
// dipakai dari subscriptions_churn_page_test.go — tak diredefinisi di sini.

// seedEngagement menyisip satu engagement dgn tipe/status/owner yang ditentukan.
func (e *testEnv) seedEngagement(t *testing.T, accountID int64, etype, status string, owner *int64) {
	t.Helper()
	at := pgtype.Timestamptz{Time: time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC), Valid: true}
	if _, err := e.q.CreateEngagement(t.Context(), db.CreateEngagementParams{
		TenantID:       e.tenantID,
		AccountID:      accountID,
		Subject:        "Engagement " + status,
		EngagementType: etype,
		ScheduledAt:    at,
		Status:         status,
		OwnerID:        owner,
	}); err != nil {
		t.Fatalf("seed engagement (%s/%s): %v", etype, status, err)
	}
}

func csDate(y int, m time.Month, d int) pgtype.Date {
	return pgtype.Date{Time: time.Date(y, m, d, 0, 0, 0, 0, time.UTC), Valid: true}
}

// --- F2: gerbang -------------------------------------------------------------

// TestReportsCS_GateRead: anggota tanpa business_role → 403 + penjelasan.
func TestReportsCS_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)

	code, body := env.reportsCSBody(t, uid, "member", "")
	if code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", code)
	}
	if !strings.Contains(body, "Customer Success Report") {
		t.Errorf("body 403 harus menyebut judul halaman, got:\n%s", body)
	}
}

// TestReportsCS_GateRead_AllowedRoles: csm/manager/admin/support (pemegang
// crm:reports read) → 200.
func TestReportsCS_GateRead_AllowedRoles(t *testing.T) {
	env, uid := setupAccounts(t)
	for _, role := range []string{"csm", "manager", "admin", "support"} {
		t.Run("role="+role, func(t *testing.T) {
			code, _ := env.reportsCSBody(t, uid, "owner", role)
			if code != http.StatusOK {
				t.Errorf("role %q: status = %d, want 200", role, code)
			}
		})
	}
}

// --- Panel 1: Health Score --------------------------------------------------

// TestReportsCS_HealthBands: KPI Rata Health Score = rata skor (koma Indonesia)
// & tabel memuat label band distribusi.
func TestReportsCS_HealthBands(t *testing.T) {
	env, uid := setupAccounts(t)
	a1 := env.seedAccount(t, "Desa Sehat", &uid, nil, nil)
	env.seedHealthScore(t, a1.ID, 90, "Healthy")
	a2 := env.seedAccount(t, "Desa Cukup", &uid, nil, nil)
	env.seedHealthScore(t, a2.ID, 70, "At-Risk")
	a3 := env.seedAccount(t, "Desa Kritis", &uid, nil, nil)
	env.seedHealthScore(t, a3.ID, 30, "Critical")

	_, body := env.reportsCSBody(t, uid, "owner", "admin")
	if got := dashboardKPIValue(t, body, "Rata Health Score"); got != "63,3" {
		t.Errorf("Rata Health Score = %q, want \"63,3\" (avg 90,70,30)", got)
	}
	for _, label := range []string{"Sehat (80–100)", "Cukup (60–79)", "Kritis"} {
		if !strings.Contains(body, label) {
			t.Errorf("tabel health harus memuat band %q", label)
		}
	}
}

// --- Panel 2: Adoption ------------------------------------------------------

// TestReportsCS_AdoptionKPI: KPI Adoption Rate = rata feature_adoption_rate "%".
func TestReportsCS_AdoptionKPI(t *testing.T) {
	env, uid := setupAccounts(t)
	a1 := env.seedAccount(t, "Desa Adopsi Tinggi", &uid, nil, nil)
	env.seedCSRow(t, db.CreateCustomerSuccessParams{AccountID: a1.ID, FeatureAdoptionRate: numFrom(t, "80")})
	a2 := env.seedAccount(t, "Desa Adopsi Rendah", &uid, nil, nil)
	env.seedCSRow(t, db.CreateCustomerSuccessParams{AccountID: a2.ID, FeatureAdoptionRate: numFrom(t, "40")})

	_, body := env.reportsCSBody(t, uid, "owner", "admin")
	if got := dashboardKPIValue(t, body, "Adoption Rate"); got != "60,0%" {
		t.Errorf("Adoption Rate = %q, want \"60,0%%\" (avg 80,40)", got)
	}
	if !strings.Contains(body, "Tinggi (80–100)") {
		t.Errorf("tabel adopsi harus memuat band Tinggi (80–100)")
	}
}

// --- Panel 3: Retention/Churn -----------------------------------------------

// TestReportsCS_RetentionAndChurn: KPI Retention Rate = active/(active+churned);
// tabel churn memuat alasan + Nilai Hilang (Rupiah) untuk role ber-ARR.
func TestReportsCS_RetentionAndChurn(t *testing.T) {
	env, uid := setupAccounts(t)
	plan := env.seedPlan(t, "Paket CS", "PLAN-CS-1", "1000000")
	// idx_subs_one_active hanya izinkan 1 langganan Active per desa → sebar 3
	// langganan aktif ke 3 desa berbeda, lalu 1 churn (retention = 3/4).
	for i, nm := range []string{"Desa Aktif 1", "Desa Aktif 2", "Desa Aktif 3"} {
		a := env.seedAccount(t, nm, &uid, nil, nil)
		env.seedSubscription(t, a.ID, plan, &uid, "Active", "1000000", "12000000")
		_ = i
	}
	churnAcc := env.seedAccount(t, "Desa Langganan Churn", &uid, nil, nil)
	env.seedChurnedSub(t, churnAcc.ID, plan, &uid, "Voluntary", "500000")

	_, body := env.reportsCSBody(t, uid, "owner", "admin")
	if got := dashboardKPIValue(t, body, "Retention Rate"); got != "75,0%" {
		t.Errorf("Retention Rate = %q, want \"75,0%%\" (3 aktif / 4)", got)
	}
	if !strings.Contains(body, "Budget") {
		t.Errorf("tabel churn harus memuat alasan Budget, body:\n%s", body)
	}
	if !strings.Contains(body, "Rp 500.000") {
		t.Errorf("admin harus melihat Nilai Hilang Rp 500.000")
	}
}

// TestReportsCS_ChurnMasking: F4 — buildChurnRows memasking Nilai Hilang bagi
// role DI LUAR canSeeARR. Diuji langsung di level builder (bukan via handler):
// role bawaan yang bisa melihat langganan (scope all/own) SELALU juga pemegang
// ARR, sedangkan Support scope=none → nol baris churn (tak ada yang di-mask).
// Masking baru menggigit pada ROLE KUSTOM (data_scope all/own tapi bukan 4 nama
// ber-ARR) — persis yang di-simulasikan businessRole "support" pada builder di
// bawah: canSeeARR("support")=false → tersamar; "admin"=true → Rupiah utuh.
func TestReportsCS_ChurnMasking(t *testing.T) {
	rows := []db.ReportChurnReasonsRow{
		{ChurnReason: "Budget", AccountCount: 1, LostValue: numFrom(t, "500000")},
	}

	masked := buildChurnRows(rows, "support")
	if len(masked) != 1 {
		t.Fatalf("buildChurnRows(support) len = %d, want 1", len(masked))
	}
	if masked[0].LostValue != flsHidden {
		t.Errorf("Nilai Hilang non-ARR = %q, want %q (tersamar)", masked[0].LostValue, flsHidden)
	}

	seen := buildChurnRows(rows, "admin")
	if !strings.Contains(seen[0].LostValue, "500.000") {
		t.Errorf("admin harus melihat Nilai Hilang Rp 500.000, got %q", seen[0].LostValue)
	}
}

// --- Panel 5: Onboarding ----------------------------------------------------

// TestReportsCS_Onboarding: Rata Durasi (actual−kickoff), Selesai, & Terlambat
// dihitung; status "Completed"/"Stalled" muncul di distribusi.
func TestReportsCS_Onboarding(t *testing.T) {
	env, uid := setupAccounts(t)
	// Selesai tepat waktu: durasi 30 hari, actual ≤ target → tak terlambat.
	done := env.seedAccount(t, "Desa Onboard Selesai", &uid, nil, nil)
	env.seedCSRow(t, db.CreateCustomerSuccessParams{
		AccountID:        done.ID,
		OnboardingStatus: ptr("Completed"),
		KickoffDate:      csDate(2026, 1, 1),
		TargetGoLiveDate: csDate(2026, 3, 1),
		ActualGoLiveDate: csDate(2026, 1, 31),
	})
	// Tersendat & kickoff jauh di masa lalu → terlambat.
	stalled := env.seedAccount(t, "Desa Onboard Tersendat", &uid, nil, nil)
	env.seedCSRow(t, db.CreateCustomerSuccessParams{
		AccountID:        stalled.ID,
		OnboardingStatus: ptr("Stalled"),
		KickoffDate:      csDate(2020, 1, 1),
	})

	_, body := env.reportsCSBody(t, uid, "owner", "admin")
	if !strings.Contains(body, "30,0 hari") {
		t.Errorf("Rata Durasi harus 30,0 hari, body:\n%s", body)
	}
	for _, label := range []string{"Selesai", "Tersendat"} {
		if !strings.Contains(body, label) {
			t.Errorf("distribusi onboarding harus memuat status %q", label)
		}
	}
}

// --- Panel 6: Engagement ----------------------------------------------------

// TestReportsCS_EngagementCompliance: kepatuhan per tipe = done/total; tabel
// per-CSM memuat pemilik.
func TestReportsCS_EngagementCompliance(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Engagement", nil, &uid, nil) // assigned_csm = uid
	env.seedEngagement(t, acc.ID, "touch_point", "done", &uid)
	env.seedEngagement(t, acc.ID, "touch_point", "planned", &uid)

	_, body := env.reportsCSBody(t, uid, "owner", "admin")
	if !strings.Contains(body, "Touch Point") {
		t.Errorf("panel engagement harus memuat tipe Touch Point, body:\n%s", body)
	}
	if !strings.Contains(body, "1 / 2") {
		t.Errorf("kepatuhan touch_point harus 1 / 2 (1 done dari 2)")
	}
	if !strings.Contains(body, "Per CSM") {
		t.Errorf("panel engagement harus punya sub-tabel Per CSM")
	}
}

// --- F3: ownership ----------------------------------------------------------

// TestReportsCS_ScopedByOwnership: CSM (own) hanya menghitung desa binaannya di
// Rata Health Score; Admin (all) lintas-CSM.
func TestReportsCS_ScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other-cs-report@local", "member", 0).ID

	own := env.seedAccount(t, "Desa Binaanku Report", nil, &uid, nil)
	env.seedHealthScore(t, own.ID, 85, "Healthy")
	otherAcc := env.seedAccount(t, "Desa Binaan Lain Report", nil, &other, nil)
	env.seedHealthScore(t, otherAcc.ID, 35, "Critical")

	const label = "Rata Health Score"
	_, ownBody := env.reportsCSBody(t, uid, "member", "csm")
	if got := dashboardKPIValue(t, ownBody, label); got != "85,0" {
		t.Errorf("csm own-scope: %s = %q, want \"85,0\" (hanya binaannya)", label, got)
	}
	_, allBody := env.reportsCSBody(t, uid, "owner", "admin")
	if got := dashboardKPIValue(t, allBody, label); got != "60,0" {
		t.Errorf("admin all-scope: %s = %q, want \"60,0\" (avg 85,35)", label, got)
	}
}

// --- panel dilewatkan --------------------------------------------------------

// TestReportsCS_SkippedPanelsAbsent: NPS/CSAT (panel 8.2 tanpa data) TAK dirender.
func TestReportsCS_SkippedPanelsAbsent(t *testing.T) {
	env, uid := setupAccounts(t)
	_, body := env.reportsCSBody(t, uid, "owner", "admin")
	for _, absent := range []string{"NPS", "CSAT"} {
		if strings.Contains(body, absent) {
			t.Errorf("panel %q tak boleh dirender (tak ada datanya), body memuatnya", absent)
		}
	}
}

// --- Export CSV --------------------------------------------------------------

// TestReportsCS_Export: default (health) → 200 text/csv memuat band; panel churn
// memuat alasan yang diseed.
func TestReportsCS_Export(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Export CS", &uid, nil, nil)
	env.seedHealthScore(t, acc.ID, 95, "Healthy")
	plan := env.seedPlan(t, "Paket Export", "PLAN-EXP-1", "1000000")
	env.seedChurnedSub(t, acc.ID, plan, &uid, "Voluntary", "250000")

	// Default panel = health.
	req := accountsReq(http.MethodGet, "/w/test/reports/customer-success/export", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.ReportsCSExport)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/csv") {
		t.Errorf("Content-Type = %q, want text/csv prefix", ct)
	}
	if !strings.Contains(rec.Body.String(), "Sehat") {
		t.Errorf("CSV health harus memuat band Sehat, got:\n%s", rec.Body.String())
	}

	// Panel churn.
	reqC := accountsReq(http.MethodGet, "/w/test/reports/customer-success/export?panel=retention", nil, "")
	recC := env.runAccount(uid, "owner", "admin", reqC, env.h.ReportsCSExport)
	if !strings.Contains(recC.Body.String(), "Budget") {
		t.Errorf("CSV churn harus memuat alasan churn, got:\n%s", recC.Body.String())
	}
}
