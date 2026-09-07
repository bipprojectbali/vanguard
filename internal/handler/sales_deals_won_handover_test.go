package handler

import (
	"strings"
	"testing"
)

// sales_deals_won_handover_test.go — BL-74: deal Closed Won → auto-create baris
// customer_success (handover Sales→CS). Menjaga: (1) Won melahirkan baris CS default
// {onboarding "Not Started", lifecycle "Onboarding", health NULL}; (2) Won saat status
// langganan Trial TETAP membuat baris CS (onboarding independen status komersial);
// (3) IDEMPOTEN — baris CS existing tak digandakan & tak ditimpa; (4) created_by = aktor
// yang meng-Won-kan deal (bukan pemilik deal). Koneksi test = superuser (bypass RLS) →
// uji LOGIKA handler; peran admin (crm:*, ScopeAll) memisahkan alur dari gate F2/F3.

// countCSForAccount menghitung baris CS desa (buktikan tak dobel — account_id UNIQUE).
func (e *testEnv) countCSForAccount(t *testing.T, accountID int64) int {
	t.Helper()
	var n int
	if err := e.h.Pool.QueryRow(t.Context(),
		`SELECT count(*) FROM customer_success WHERE account_id=$1`, accountID).Scan(&n); err != nil {
		t.Fatalf("count cs: %v", err)
	}
	return n
}

// TestDealWon_CreatesCSHandover: happy-path. Closed Won → baris customer_success desa
// lahir dgn default handover; nilai turunan health dibiarkan NULL; audit tercatat.
func TestDealWon_CreatesCSHandover(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Handover", &uid, nil, nil)
	plan := env.seedPlan(t, "Paket Inti", "PLN-HDV", "100000.00")
	deal := env.seedWonReadyDeal(t, acc.ID, &uid, plan, "12000000", "Annual")

	rec := env.postDealStage(uid, deal.ID, wonForm("Active"))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=staged") {
		t.Fatalf("Won harus sukses, got %q (status %d)", loc, rec.Code)
	}

	cs, err := env.q.GetCustomerSuccessByAccountID(t.Context(), acc.ID)
	if err != nil {
		t.Fatalf("baris CS handover harus lahir saat Won: %v", err)
	}
	if cs.OnboardingStatus == nil || *cs.OnboardingStatus != "Not Started" {
		t.Errorf("onboarding_status = %v, want \"Not Started\"", cs.OnboardingStatus)
	}
	if cs.LifecycleStage == nil || *cs.LifecycleStage != "Onboarding" {
		t.Errorf("lifecycle_stage = %v, want \"Onboarding\"", cs.LifecycleStage)
	}
	// Nilai turunan health SENGAJA NULL — operator/CSM isi kemudian.
	if cs.OverallHealthScore != nil {
		t.Errorf("overall_health_score handover harus NULL, got %v", *cs.OverallHealthScore)
	}
	if cs.HealthStatus != nil {
		t.Errorf("health_status handover harus NULL, got %v", *cs.HealthStatus)
	}
	if cs.CreatedBy == nil || *cs.CreatedBy != uid {
		t.Errorf("created_by = %v, want %d (aktor)", cs.CreatedBy, uid)
	}
	if n := env.countCSForAccount(t, acc.ID); n != 1 {
		t.Errorf("baris CS desa = %d, want 1", n)
	}
	env.assertAudited(t, "cs.handover.created")
}

// TestDealWon_CSHandoverDuringTrial: Won dgn status langganan TRIAL tetap melahirkan
// baris CS — bukti onboarding INDEPENDEN status komersial (tak menunggu Active).
func TestDealWon_CSHandoverDuringTrial(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Trial CS", &uid, nil, nil)
	plan := env.seedPlan(t, "Paket Inti", "PLN-TRLCS", "100000.00")
	deal := env.seedWonReadyDeal(t, acc.ID, &uid, plan, "6000000", "Monthly")

	rec := env.postDealStage(uid, deal.ID, wonForm("Trial"))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=staged") {
		t.Fatalf("Won(Trial) harus sukses, got %q", loc)
	}
	cs, err := env.q.GetCustomerSuccessByAccountID(t.Context(), acc.ID)
	if err != nil {
		t.Fatalf("baris CS harus lahir walau langganan Trial: %v", err)
	}
	if cs.OnboardingStatus == nil || *cs.OnboardingStatus != "Not Started" {
		t.Errorf("onboarding_status = %v, want \"Not Started\"", cs.OnboardingStatus)
	}
	if n := env.countCSForAccount(t, acc.ID); n != 1 {
		t.Errorf("baris CS desa = %d, want 1", n)
	}
}

// TestDealWon_CSHandoverIdempotent: baris CS SUDAH ada (diisi CS, mis. onboarding
// "In Progress") → Won TAK menggandakan & TAK menimpa nilai existing (skip idempoten).
func TestDealWon_CSHandoverIdempotent(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa CS Ada", &uid, nil, nil)
	plan := env.seedPlan(t, "Paket Inti", "PLN-CSIDEM", "100000.00")
	env.seedCustomerSuccess(t, acc.ID) // onboarding "In Progress", health terisi
	deal := env.seedWonReadyDeal(t, acc.ID, &uid, plan, "12000000", "Annual")

	rec := env.postDealStage(uid, deal.ID, wonForm("Active"))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=staged") {
		t.Fatalf("Won harus sukses, got %q", loc)
	}
	cs, err := env.q.GetCustomerSuccessByAccountID(t.Context(), acc.ID)
	if err != nil {
		t.Fatalf("get cs: %v", err)
	}
	// Nilai existing HARUS bertahan — handover tak menimpa pekerjaan CS.
	if cs.OnboardingStatus == nil || *cs.OnboardingStatus != "In Progress" {
		t.Errorf("onboarding_status ditimpa! = %v, want \"In Progress\" (nilai existing)", cs.OnboardingStatus)
	}
	if cs.OverallHealthScore == nil || *cs.OverallHealthScore != 75 {
		t.Errorf("health existing ditimpa! overall_health_score = %v, want 75", cs.OverallHealthScore)
	}
	if n := env.countCSForAccount(t, acc.ID); n != 1 {
		t.Errorf("baris CS desa = %d, want 1 (tak dobel)", n)
	}
}

// TestDealWon_CSHandoverCreatedByActor: created_by = AKTOR yang meng-Won-kan (bukan
// pemilik deal). Deal dimiliki user lain; Won dijalankan admin uid → created_by=uid.
func TestDealWon_CSHandoverCreatedByActor(t *testing.T) {
	env, uid := setupAccounts(t)
	owner := env.seedMember(t, "owner-lain@local", "member", env.tenantID)
	acc := env.seedAccount(t, "Desa Aktor", &uid, nil, nil)
	plan := env.seedPlan(t, "Paket Inti", "PLN-ACT", "100000.00")
	deal := env.seedWonReadyDeal(t, acc.ID, &owner.ID, plan, "12000000", "Annual")

	rec := env.postDealStage(uid, deal.ID, wonForm("Active"))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=staged") {
		t.Fatalf("Won harus sukses, got %q", loc)
	}
	cs, err := env.q.GetCustomerSuccessByAccountID(t.Context(), acc.ID)
	if err != nil {
		t.Fatalf("get cs: %v", err)
	}
	if cs.CreatedBy == nil || *cs.CreatedBy != uid {
		t.Errorf("created_by = %v, want %d (aktor Won, bukan pemilik deal %d)", cs.CreatedBy, uid, owner.ID)
	}
}
