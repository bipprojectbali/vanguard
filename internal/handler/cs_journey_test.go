package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"go_starter/internal/db"
)

// cs_journey_test.go — Customer Journey / Lifecycle (CRM Modul 6, 6.2 — BL-77)
// di sisi handler. Dashboard portofolio fase perjalanan desa. Empat sumbu
// dijaga sesuai spec:
//
//   - F2 (gerbang): objek Casbin crm:journey read (REUSE). Support TAK punya
//     crm:journey → 403 fail-closed; csm/admin (pemegang) → 200.
//   - Agregasi KPI (Onboarding/Adoption/Retention/Menuju Renewal) & funnel
//     per-fase (jumlah desa, macet) konsisten dengan data seed.
//   - days_in_stage DIHITUNG DI GO dari stage_entry_date (gotcha #14) —
//     diuji langsung sebagai fungsi murni.
//   - TableScroll membungkus tabel (regresi mobile-first).
//
// F3 (ownership) & keyset di cs_journey_scope_test.go (paket sama, dipisah agar
// tiap file di bawah ambang tipe Test 400). Koneksi test = superuser (bypass
// RLS) → uji LOGIKA handler; isolasi RLS di rls_test.go.

// seedCSJourneyRow menyisip satu baris customer_success dengan field yang
// relevan untuk journey (fase, tanggal masuk fase, onboarding). Pembungkus
// tipis seedCSRow agar test hanya merakit yang dipakai.
func (e *testEnv) seedCSJourneyRow(
	t *testing.T, accountID int64, stage string, stageEntry pgtype.Date,
	onboardingStatus *string, progress *int16, targetGoLive pgtype.Date,
) {
	t.Helper()
	e.seedCSRow(t, db.CreateCustomerSuccessParams{
		AccountID:          accountID,
		LifecycleStage:     &stage,
		StageEntryDate:     stageEntry,
		OnboardingStatus:   onboardingStatus,
		OnboardingProgress: progress,
		TargetGoLiveDate:   targetGoLive,
	})
}

// daysAgoDate mengembalikan tanggal (date-only UTC) n hari sebelum hari ini.
func daysAgoDate(n int) pgtype.Date {
	d := time.Now().AddDate(0, 0, -n)
	return pgtype.Date{Time: time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC), Valid: true}
}

// csJourneyBody menjalankan CSJourneyList sebagai (uid, tenantRole, businessRole).
func (e *testEnv) csJourneyBody(t *testing.T, uid int64, tenantRole, businessRole string) (int, string) {
	t.Helper()
	req := accountsReq(http.MethodGet, "/w/test/journey", nil, "")
	rec := e.runAccount(uid, tenantRole, businessRole, req, e.h.CSJourneyList)
	return rec.Code, rec.Body.String()
}

// --- F2: gerbang -------------------------------------------------------------

// TestCSJourney_GateSupportForbidden: Support TAK memegang crm:journey read →
// 403 + judul halaman (bukan bocor data).
func TestCSJourney_GateSupportForbidden(t *testing.T) {
	env, uid := setupAccounts(t)

	code, body := env.csJourneyBody(t, uid, "member", "support")
	if code != http.StatusForbidden {
		t.Fatalf("Support harus 403, got %d", code)
	}
	if !strings.Contains(body, "Customer Journey") {
		t.Errorf("body 403 harus menyebut judul halaman, got:\n%s", body)
	}
}

// TestCSJourney_GateAllowedRoles: csm & admin (pemegang crm:journey read) → 200.
func TestCSJourney_GateAllowedRoles(t *testing.T) {
	env, uid := setupAccounts(t)

	for _, role := range []string{"csm", "admin", "manager", "sales"} {
		tenantRole := "member"
		if role == "admin" {
			tenantRole = "owner"
		}
		code, _ := env.csJourneyBody(t, uid, tenantRole, role)
		if code != http.StatusOK {
			t.Errorf("role %s harus 200, got %d", role, code)
		}
	}
}

// --- Agregasi KPI ------------------------------------------------------------

// TestCSJourney_KPIAggregation: hitungan per-fase KPI konsisten dengan seed,
// dan "Menuju Renewal" MURNI dari lifecycle_stage='Renewal' (tak menghitung
// fase lain).
func TestCSJourney_KPIAggregation(t *testing.T) {
	env, uid := setupAccounts(t)

	accOn1 := env.seedAccount(t, "Desa Onboarding 1", &uid, nil, nil)
	accOn2 := env.seedAccount(t, "Desa Onboarding 2", &uid, nil, nil)
	accAdopt := env.seedAccount(t, "Desa Adoption", &uid, nil, nil)
	accRet := env.seedAccount(t, "Desa Retention", &uid, nil, nil)
	accRen := env.seedAccount(t, "Desa Renewal", &uid, nil, nil)

	inProg := "In Progress"
	env.seedCSJourneyRow(t, accOn1.ID, "Onboarding", daysAgoDate(10), &inProg, ptr(int16(30)), pgtype.Date{})
	env.seedCSJourneyRow(t, accOn2.ID, "Onboarding", daysAgoDate(20), &inProg, ptr(int16(50)), pgtype.Date{})
	env.seedCSJourneyRow(t, accAdopt.ID, "Adoption", daysAgoDate(40), nil, nil, pgtype.Date{})
	env.seedCSJourneyRow(t, accRet.ID, "Retention", daysAgoDate(5), nil, nil, pgtype.Date{})
	env.seedCSJourneyRow(t, accRen.ID, "Renewal", daysAgoDate(3), nil, nil, pgtype.Date{})

	kpis, err := env.q.CountCSJourneyKPIs(t.Context(), db.CountCSJourneyKPIsParams{
		ScopeAll: true, Uid: &uid,
	})
	if err != nil {
		t.Fatalf("CountCSJourneyKPIs: %v", err)
	}

	if kpis.OnboardingCount != 2 {
		t.Errorf("onboarding_count = %d, want 2", kpis.OnboardingCount)
	}
	if kpis.AdoptionCount != 1 {
		t.Errorf("adoption_count = %d, want 1", kpis.AdoptionCount)
	}
	if kpis.RetentionCount != 1 {
		t.Errorf("retention_count = %d, want 1", kpis.RetentionCount)
	}
	if kpis.RenewalCount != 1 {
		t.Errorf("renewal_count (Menuju Renewal) = %d, want 1 (murni Renewal)", kpis.RenewalCount)
	}
	// avg hari fase Onboarding (10 & 20 hari lalu) harus > 0 dan masuk akal.
	if kpis.OnboardingAvgDays <= 0 {
		t.Errorf("onboarding_avg_days = %v, want > 0", kpis.OnboardingAvgDays)
	}
	// Adoption fase terlama = 40 hari lalu → max_days ≈ 40 (≥ 1).
	if kpis.AdoptionMaxDays < 1 {
		t.Errorf("adoption_max_days = %d, want ≥ 1", kpis.AdoptionMaxDays)
	}
}

// --- Agregasi per-fase (funnel) ---------------------------------------------

// TestCSJourney_PhaseAggregation: ListCSJourneyPhases mengelompokkan per fase &
// menandai desa MACET (stage_entry_date lebih tua dari ambang stalled).
func TestCSJourney_PhaseAggregation(t *testing.T) {
	env, uid := setupAccounts(t)

	accFresh := env.seedAccount(t, "Desa Adopsi Segar", &uid, nil, nil)
	accStalled := env.seedAccount(t, "Desa Adopsi Macet", &uid, nil, nil)

	// Dua desa fase Adoption: satu baru (5 hari), satu macet (>90 hari).
	env.seedCSJourneyRow(t, accFresh.ID, "Adoption", daysAgoDate(5), nil, nil, pgtype.Date{})
	env.seedCSJourneyRow(t, accStalled.ID, "Adoption", daysAgoDate(csJourneyStalledDays+30), nil, nil, pgtype.Date{})

	phases, err := env.q.ListCSJourneyPhases(t.Context(), db.ListCSJourneyPhasesParams{
		StalledDays: csJourneyStalledDays, ScopeAll: true, Uid: &uid,
	})
	if err != nil {
		t.Fatalf("ListCSJourneyPhases: %v", err)
	}

	var adoption *db.ListCSJourneyPhasesRow
	for i := range phases {
		if phases[i].Stage != nil && *phases[i].Stage == "Adoption" {
			adoption = &phases[i]
		}
	}
	if adoption == nil {
		t.Fatalf("fase Adoption tak ada di hasil agregat")
	}
	if adoption.VillageCount != 2 {
		t.Errorf("Adoption village_count = %d, want 2", adoption.VillageCount)
	}
	if adoption.StalledCount != 1 {
		t.Errorf("Adoption stalled_count = %d, want 1 (satu desa >%d hari)", adoption.StalledCount, csJourneyStalledDays)
	}
}

// --- days_in_stage dihitung di Go (gotcha #14) -------------------------------

// TestCSJourney_DaysInStageComputedInGo: fungsi murni daysInStage menghitung
// selisih hari dari stage_entry_date terhadap "today" — tanpa ekspresi tanggal
// di SELECT list SQL. NULL → tak diketahui; masa depan → diklem 0.
func TestCSJourney_DaysInStageComputedInGo(t *testing.T) {
	today := time.Date(2026, 9, 7, 15, 0, 0, 0, time.UTC)

	cases := []struct {
		name      string
		entry     pgtype.Date
		wantDays  int
		wantKnown bool
	}{
		{"10 hari lalu", pgtype.Date{Time: time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC), Valid: true}, 10, true},
		{"hari ini", pgtype.Date{Time: time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC), Valid: true}, 0, true},
		{"masa depan diklem 0", pgtype.Date{Time: time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC), Valid: true}, 0, true},
		{"NULL tak diketahui", pgtype.Date{}, 0, false},
	}
	for _, c := range cases {
		days, known := daysInStage(c.entry, today)
		if known != c.wantKnown {
			t.Errorf("%s: known = %v, want %v", c.name, known, c.wantKnown)
		}
		if days != c.wantDays {
			t.Errorf("%s: days = %d, want %d", c.name, days, c.wantDays)
		}
	}
}

// --- Render: TableScroll (mobile-first) --------------------------------------

// TestCSJourney_TableScrollWraps: tabel utama & Onboarding Aktif dibungkus
// ui.TableScroll (overflow-x-auto min-w-0) — regresi tablescroll.
func TestCSJourney_TableScrollWraps(t *testing.T) {
	env, uid := setupAccounts(t)

	acc := env.seedAccount(t, "Desa Render", &uid, nil, nil)
	inProg := "In Progress"
	env.seedCSJourneyRow(t, acc.ID, "Onboarding", daysAgoDate(7), &inProg, ptr(int16(40)), daysAgoDate(-14))

	code, body := env.csJourneyBody(t, uid, "owner", "admin")
	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", code, body)
	}
	if !strings.Contains(body, "overflow-x-auto") {
		t.Errorf("tabel journey wajib dibungkus ui.TableScroll (overflow-x-auto)")
	}
	if !strings.Contains(body, "Desa Binaan") {
		t.Errorf("tabel utama 'Desa Binaan — Posisi Lifecycle' harus dirender")
	}
	if !strings.Contains(body, "Desa Render") {
		t.Errorf("baris desa seed harus tampil di tabel")
	}
}
