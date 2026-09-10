package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// health_score_dashboard_test.go — BL-96: pengayaan halaman Health Score jadi
// dasbor (INTI 1–4). Yang dijaga:
//
//   - healthKPIsToView: sub-teks KPI (rata skor / % Sehat), tiga panel dasbor
//     (Sebaran/Komposisi/Arah), placeholder saat belum ada skor (Scored=0).
//   - healthActionLabel: aksi kontekstual per-status (Playbook/Tinjau/Lihat).
//   - CountHealthScoreKPIs: agregat baru (avg per-komponen + cacah tren) benar.
//   - ListHealthScores: kolom "Jatuh Tempo" = end_date langganan aktif (LATERAL).
//   - Render tabel: header "Jatuh Tempo" ada, "Di Stage" (lama) hilang, aksi
//     kontekstual muncul, panel dasbor terrender.
//
// Fungsi murni diuji tanpa DB; sisanya pakai pool test (superuser, uji logika).

// --- helper: seed customer_success kaya (komponen + tren) ------------------

// seedHealthScoreFull menaruh baris customer_success lengkap agar test agregat
// bisa memverifikasi avg per-komponen dan cacah tren (seedHealthScore biasa
// hanya set overall + status).
func (e *testEnv) seedHealthScoreFull(
	t *testing.T, accountID int64, overall, adoption, engagement, support, sentiment int16, status, trend string,
) {
	t.Helper()
	_, err := e.q.CreateCustomerSuccess(t.Context(), db.CreateCustomerSuccessParams{
		TenantID:           e.tenantID,
		AccountID:          accountID,
		OverallHealthScore: &overall,
		HealthStatus:       &status,
		AdoptionScore:      &adoption,
		EngagementScore:    &engagement,
		SupportScore:       &support,
		SentimentScore:     &sentiment,
		ScoreTrend:         &trend,
	})
	if err != nil {
		t.Fatalf("seed full health score for account %d: %v", accountID, err)
	}
}

// --- unit: healthActionLabel ----------------------------------------------

func TestHealthActionLabel(t *testing.T) {
	crit, atrisk, healthy := "Critical", "At-Risk", "Healthy"
	cases := []struct {
		name   string
		status *string
		want   string
	}{
		{"nil (belum dinilai)", nil, "Lihat"},
		{"critical", &crit, "Playbook"},
		{"at-risk", &atrisk, "Tinjau"},
		{"healthy", &healthy, "Lihat"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := healthActionLabel(c.status); got != c.want {
				t.Errorf("healthActionLabel(%v) = %q, want %q", c.status, got, c.want)
			}
		})
	}
}

// --- unit: pctOf & roundToInt ---------------------------------------------

func TestPctOf(t *testing.T) {
	cases := []struct {
		count, total int64
		want         int
	}{
		{0, 0, 0},   // guard bagi nol
		{1, 0, 0},   // total nol → 0
		{1, 4, 25},  // 25%
		{1, 3, 33},  // 33.33 → 33 (bulat terdekat)
		{2, 3, 67},  // 66.67 → 67
		{5, 5, 100}, // penuh
	}
	for _, c := range cases {
		if got := pctOf(c.count, c.total); got != c.want {
			t.Errorf("pctOf(%d,%d) = %d, want %d", c.count, c.total, got, c.want)
		}
	}
}

func TestRoundToInt(t *testing.T) {
	cases := []struct {
		in   float64
		want int
	}{
		{-1, 0},    // negatif dijepit 0
		{0, 0},     //
		{78.4, 78}, // bulat bawah
		{78.5, 79}, // bulat atas
	}
	for _, c := range cases {
		if got := roundToInt(c.in); got != c.want {
			t.Errorf("roundToInt(%v) = %d, want %d", c.in, got, c.want)
		}
	}
}

// --- unit: healthKPIsToView (Scored>0) ------------------------------------

func TestHealthKPIsToView_Scored(t *testing.T) {
	k := db.CountHealthScoreKPIsRow{
		Total: 10, Healthy: 4, AtRisk: 3, Critical: 2, Scored: 9,
		AvgScore:       78.5,
		AvgAdoption:    60,
		AvgEngagement:  55.4,
		AvgSupport:     120, // sengaja >100 → harus dijepit ke 100
		AvgSentiment:   40,
		TrendImproving: 3, TrendStable: 4, TrendDeclining: 1,
	}
	kpi, panels := healthKPIsToView(k)

	if kpi.AvgScoreSub != "rata skor 79" {
		t.Errorf("AvgScoreSub = %q, want %q", kpi.AvgScoreSub, "rata skor 79")
	}
	// 4/10 = 40%
	if kpi.HealthySub != "40% dari binaan" {
		t.Errorf("HealthySub = %q, want %q", kpi.HealthySub, "40% dari binaan")
	}
	if !panels.Scored {
		t.Fatal("panels.Scored harus true saat Scored>0")
	}

	// Sebaran: 3 bar dengan persen count/total.
	if len(panels.Distribution) != 3 {
		t.Fatalf("Distribution harus 3 bar, got %d", len(panels.Distribution))
	}
	if panels.Distribution[0].Label != "Sehat" || panels.Distribution[0].Pct != 40 {
		t.Errorf("Distribution[0] = %+v, want Sehat 40%%", panels.Distribution[0])
	}
	if panels.Distribution[0].Value != "4 · 40%" {
		t.Errorf("Distribution[0].Value = %q, want %q", panels.Distribution[0].Value, "4 · 40%")
	}
	if panels.Distribution[0].Color != "bg-success" {
		t.Errorf("Distribution[0].Color = %q, want bg-success", panels.Distribution[0].Color)
	}

	// Komposisi: 4 bar, Support dijepit ke 100.
	if len(panels.Composition) != 4 {
		t.Fatalf("Composition harus 4 bar, got %d", len(panels.Composition))
	}
	if panels.Composition[0].Label != "Adopsi" || panels.Composition[0].Pct != 60 {
		t.Errorf("Composition[0] = %+v, want Adopsi 60", panels.Composition[0])
	}
	if panels.Composition[2].Label != "Support" || panels.Composition[2].Pct != 100 {
		t.Errorf("Composition[2] (Support) harus dijepit ke 100, got %+v", panels.Composition[2])
	}
	if panels.Composition[3].Label != "Sentimen" {
		t.Errorf("Composition[3].Label = %q, want Sentimen", panels.Composition[3].Label)
	}

	// Arah Pergerakan: basis = improving+stable+declining = 8 (BUKAN total 10).
	if len(panels.Movement) != 3 {
		t.Fatalf("Movement harus 3 bar, got %d", len(panels.Movement))
	}
	// 3/8 = 37.5 → 38
	if panels.Movement[0].Label != "Membaik" || panels.Movement[0].Pct != 38 {
		t.Errorf("Movement[0] = %+v, want Membaik 38%% (3/8)", panels.Movement[0])
	}
	// 1/8 = 12.5 → 13
	if panels.Movement[2].Label != "Menurun" || panels.Movement[2].Pct != 13 {
		t.Errorf("Movement[2] = %+v, want Menurun 13%% (1/8)", panels.Movement[2])
	}
}

// --- unit: healthKPIsToView (Scored=0 → placeholder) ----------------------

func TestHealthKPIsToView_NoScore(t *testing.T) {
	// Mis. Support scope=none, atau semua desa belum dinilai: COALESCE membuat
	// AvgScore=0, tapi Scored=0 → sub "belum ada skor" & panel kosong.
	k := db.CountHealthScoreKPIsRow{Total: 5, Scored: 0}
	kpi, panels := healthKPIsToView(k)

	if kpi.AvgScoreSub != "belum ada skor" {
		t.Errorf("AvgScoreSub = %q, want %q", kpi.AvgScoreSub, "belum ada skor")
	}
	if kpi.HealthySub != "0% dari binaan" {
		t.Errorf("HealthySub = %q, want %q", kpi.HealthySub, "0% dari binaan")
	}
	if panels.Scored {
		t.Error("panels.Scored harus false saat Scored=0")
	}
	if panels.Distribution != nil || panels.Composition != nil || panels.Movement != nil {
		t.Error("panel harus kosong (nil) saat Scored=0 agar bar 0% tak menyesatkan")
	}
}

// --- DB: agregat baru CountHealthScoreKPIs --------------------------------

func TestCountHealthScoreKPIs_Aggregates(t *testing.T) {
	env, uid := setupAccounts(t)

	// Dua desa terskor dengan komponen & tren diketahui.
	a1 := env.seedAccount(t, "Desa Agg A", &uid, nil, nil)
	env.seedHealthScoreFull(t, a1.ID, 80, 70, 60, 50, 40, "Healthy", "Improving")
	env.makeCustomer(t, a1.ID, "Active")
	a2 := env.seedAccount(t, "Desa Agg B", &uid, nil, nil)
	env.seedHealthScoreFull(t, a2.ID, 40, 30, 20, 10, 0, "Critical", "Declining")
	env.makeCustomer(t, a2.ID, "Active")

	// Filter ke hanya kedua desa ini via ownership CSM tidak praktis (setup
	// truncate menyisakan akun lain). Pakai scope_all lalu verifikasi via
	// hitung manual dari baris yang kita seed relatif ke agregat: cek bahwa
	// tren improving & declining masing-masing ≥1 dan avg berada di rentang.
	k, err := env.q.CountHealthScoreKPIs(t.Context(), db.CountHealthScoreKPIsParams{ScopeAll: true})
	if err != nil {
		t.Fatalf("CountHealthScoreKPIs: %v", err)
	}
	if k.Scored < 2 {
		t.Fatalf("Scored harus ≥2, got %d", k.Scored)
	}
	if k.TrendImproving < 1 {
		t.Errorf("TrendImproving harus ≥1, got %d", k.TrendImproving)
	}
	if k.TrendDeclining < 1 {
		t.Errorf("TrendDeclining harus ≥1, got %d", k.TrendDeclining)
	}
	// AvgScore harus non-nol (COALESCE 0 hanya saat tak ada baris terskor).
	if k.AvgScore <= 0 {
		t.Errorf("AvgScore harus >0 dengan baris terskor, got %v", k.AvgScore)
	}
	// Avg per-komponen non-negatif dan ≤100 (skor 0–100).
	for name, v := range map[string]float64{
		"adoption": k.AvgAdoption, "engagement": k.AvgEngagement,
		"support": k.AvgSupport, "sentiment": k.AvgSentiment,
	} {
		if v < 0 || v > 100 {
			t.Errorf("Avg%s di luar 0..100: %v", name, v)
		}
	}
}

// --- DB: kolom Jatuh Tempo = end_date langganan aktif (LATERAL) -----------

func TestListHealthScores_RenewalEndDate(t *testing.T) {
	env, uid := setupAccounts(t)

	acc := env.seedAccount(t, "Desa Renewal", &uid, nil, nil)
	env.seedHealthScore(t, acc.ID, 70, "At-Risk")

	planID := env.seedPlan(t, "Paket Renewal", "PKG-REN", "100000")
	want := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	end := pgtype.Date{Time: want, Valid: true}

	code, err := env.q.GenerateEntityCode(t.Context(), env.tenantID, "subscription")
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	if _, err := env.q.CreateSubscription(t.Context(), db.CreateSubscriptionParams{
		TenantID:   env.tenantID,
		EntityCode: &code,
		AccountID:  acc.ID,
		PlanID:     &planID,
		Status:     "Active",
		EndDate:    end,
		CreatedBy:  &uid,
	}); err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	rows := env.allHealthScoreRows(t)
	var found bool
	for _, r := range rows {
		if r.ID != acc.ID {
			continue
		}
		found = true
		if !r.RenewalEndDate.Valid {
			t.Fatal("RenewalEndDate harus terisi dari langganan aktif")
		}
		got := r.RenewalEndDate.Time.UTC()
		if got.Year() != want.Year() || got.Month() != want.Month() || got.Day() != want.Day() {
			t.Errorf("RenewalEndDate = %v, want %v", got, want)
		}
	}
	if !found {
		t.Fatal("baris Desa Renewal tak ditemukan di ListHealthScores")
	}
}

// TestListHealthScores_NoActiveSub: desa PELANGGAN (BL-114 → punya langganan
// hidup) tapi langganannya BUKAN status 'Active' (mis. Trial) → RenewalEndDate
// NULL (kolom "Jatuh Tempo" tampil "—"; LATERAL hanya menatap langganan Active).
// Desa tetap muncul di daftar karena Trial termasuk segmen aktif.
func TestListHealthScores_NoActiveSub(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Trial", &uid, nil, nil)
	env.seedHealthScore(t, acc.ID, 88, "Healthy")
	env.makeCustomer(t, acc.ID, "Trial") // hidup, tapi bukan 'Active' → tak ada end_date renewal

	rows := env.allHealthScoreRows(t)
	var found bool
	for _, r := range rows {
		if r.ID != acc.ID {
			continue
		}
		found = true
		if r.RenewalEndDate.Valid {
			t.Errorf("RenewalEndDate harus NULL tanpa langganan Active, got %v", r.RenewalEndDate.Time)
		}
	}
	if !found {
		t.Fatal("desa pelanggan Trial harus tetap muncul di ListHealthScores (segmen aktif)")
	}
}

// --- render: kolom tabel & panel dasbor -----------------------------------

// TestHealthScore_DashboardRender: header tabel diselaraskan mockup ("Jatuh
// Tempo" ada, "Di Stage" lama hilang), aksi kontekstual muncul, panel dasbor
// (Sebaran/Komposisi) & sub-teks KPI terrender.
func TestHealthScore_DashboardRender(t *testing.T) {
	env, uid := setupAccounts(t)

	a1 := env.seedAccount(t, "Desa Kritis Render", &uid, nil, nil)
	env.seedHealthScoreFull(t, a1.ID, 20, 15, 10, 25, 30, "Critical", "Declining")
	env.makeCustomer(t, a1.ID, "Active")
	a2 := env.seedAccount(t, "Desa Berisiko Render", &uid, nil, nil)
	env.seedHealthScoreFull(t, a2.ID, 55, 50, 45, 60, 55, "At-Risk", "Stable")
	env.makeCustomer(t, a2.ID, "Active")

	req := accountsReq(http.MethodGet, "/w/test/health-scores", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.HealthScoreList)
	if rec.Code != http.StatusOK {
		t.Fatalf("harus 200, got %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	// Kolom baru & kolom lama yang dibuang.
	if !strings.Contains(body, "Jatuh Tempo") {
		t.Error("header tabel harus punya kolom 'Jatuh Tempo'")
	}
	if strings.Contains(body, "Di Stage") {
		t.Error("kolom lama 'Di Stage' harus dihapus")
	}

	// Aksi kontekstual per-status.
	if !strings.Contains(body, "Playbook") {
		t.Error("baris Critical harus punya aksi 'Playbook'")
	}
	if !strings.Contains(body, "Tinjau") {
		t.Error("baris At-Risk harus punya aksi 'Tinjau'")
	}

	// Panel dasbor terrender (Scored>0).
	if !strings.Contains(body, "Sebaran Kesehatan") {
		t.Error("panel 'Sebaran Kesehatan' harus terrender")
	}
	if !strings.Contains(body, "Komposisi Skor") {
		t.Error("panel 'Komposisi Skor' harus terrender")
	}
	if !strings.Contains(body, "Arah Pergerakan") {
		t.Error("blok 'Arah Pergerakan' harus terrender")
	}

	// Sub-teks KPI.
	if !strings.Contains(body, "rata skor") {
		t.Error("KPI Desa Binaan harus punya sub-teks 'rata skor N'")
	}
	if !strings.Contains(body, "dari binaan") {
		t.Error("KPI Sehat harus punya sub-teks '% dari binaan'")
	}
}
