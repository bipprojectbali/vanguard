package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// success_plans_dashboard_test.go — BL-97: perkaya halaman Success Plans (6.3)
// jadi dasbor. Yang dijaga:
//
//   - CountSuccessPlanKPIs: aktif (Active/At-Risk) + desa distinct, terlambat
//     (target < today), jatuh tempo 14 hari, rata capaian — semua honor filter
//     ownership yang SAMA dengan ListSuccessPlans.
//   - successPlanKPIsToView: sub-teks & pembulatan rata capaian benar.
//   - ListSuccessPlans: kolom Health ter-JOIN dari customer_success (BL-24).
//   - Render: kolom mockup selaras (Nama Plan & Owner CS absen; Health/Buka ada;
//     "Langkah" ditunda), KPI + subtitle tampil.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA; isolasi RLS di rls_test.go.
// Reuse helper setupAccounts/seedAccount/seedHealthScore/runAccount.

// --- helper ----------------------------------------------------------------

// seedPlanFull menanam success plan lengkap (status, target_date, progress) untuk
// desa yang sudah ada — dipakai menguji bucket KPI. assignedCSM di account bila
// perlu F3, di sini nil (test scope_all).
func (e *testEnv) seedPlanFull(
	t *testing.T, accountID int64, planName, status string, progress int16, target pgtype.Date,
) db.SuccessPlan {
	t.Helper()
	sp, err := e.q.CreateSuccessPlan(t.Context(), db.CreateSuccessPlanParams{
		TenantID:   e.tenantID,
		AccountID:  accountID,
		PlanName:   planName,
		PlanStatus: status,
		Progress:   progress,
		TargetDate: target,
	})
	if err != nil {
		t.Fatalf("seed plan %q: %v", planName, err)
	}
	return sp
}

func dateDaysFromNow(d int) pgtype.Date {
	return pgtype.Date{Time: time.Now().AddDate(0, 0, d), Valid: true}
}

// --- CountSuccessPlanKPIs (DB) --------------------------------------------

func TestCountSuccessPlanKPIs_Aggregates(t *testing.T) {
	env, uid := setupAccounts(t)

	// Dua desa berbeda supaya active_villages (DISTINCT) teruji.
	a1 := env.seedAccount(t, "Desa Alpha", &uid, nil, nil)
	a2 := env.seedAccount(t, "Desa Beta", &uid, nil, nil)

	// Aktif (Active) — target masa depan jauh (bukan overdue/due-soon).
	env.seedPlanFull(t, a1.ID, "Aktif Alpha", "Active", 40, dateDaysFromNow(60))
	// Aktif (At-Risk) di desa a1 juga → active_count naik, active_villages tetap.
	env.seedPlanFull(t, a1.ID, "Berisiko Alpha", "At-Risk", 80, dateDaysFromNow(60))
	// Aktif di desa a2 → active_villages = 2.
	env.seedPlanFull(t, a2.ID, "Aktif Beta", "Active", 60, dateDaysFromNow(5)) // due-soon
	// Terlambat: Active dgn target kemarin.
	env.seedPlanFull(t, a2.ID, "Telat Beta", "Active", 20, dateDaysFromNow(-3))
	// Draft & Achieved & Cancelled → TIDAK dihitung aktif.
	env.seedPlanFull(t, a1.ID, "Draft Alpha", "Draft", 0, dateDaysFromNow(-3))
	env.seedPlanFull(t, a2.ID, "Selesai Beta", "Achieved", 100, dateDaysFromNow(-1))

	k, err := env.q.CountSuccessPlanKPIs(t.Context(), db.CountSuccessPlanKPIsParams{
		Today:    pgtype.Date{Time: time.Now(), Valid: true},
		ScopeAll: true,
	})
	if err != nil {
		t.Fatalf("CountSuccessPlanKPIs: %v", err)
	}

	// Aktif = 4 (Active Alpha, At-Risk Alpha, Active Beta, Telat Beta).
	if k.ActiveCount != 4 {
		t.Errorf("ActiveCount = %d, want 4", k.ActiveCount)
	}
	// Desa aktif distinct = 2 (Alpha, Beta).
	if k.ActiveVillages != 2 {
		t.Errorf("ActiveVillages = %d, want 2", k.ActiveVillages)
	}
	// Terlambat = 1 (Telat Beta; Draft & Achieved lampau tak dihitung).
	if k.Overdue != 1 {
		t.Errorf("Overdue = %d, want 1", k.Overdue)
	}
	// Jatuh tempo 14 hari = 1 (Aktif Beta target +5).
	if k.DueSoon != 1 {
		t.Errorf("DueSoon = %d, want 1", k.DueSoon)
	}
	// Rata capaian = AVG(40,80,60,20) = 50.
	if k.AvgProgress < 49.9 || k.AvgProgress > 50.1 {
		t.Errorf("AvgProgress = %v, want ~50", k.AvgProgress)
	}
}

// TestCountSuccessPlanKPIs_ScopeNoneEmpty: Support (ScopeNone → semua flag false)
// mendapat nol (fail-closed) walau data ada.
func TestCountSuccessPlanKPIs_ScopeNoneEmpty(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Support", &uid, nil, nil)
	env.seedPlanFull(t, a.ID, "Plan X", "Active", 50, dateDaysFromNow(10))

	k, err := env.q.CountSuccessPlanKPIs(t.Context(), db.CountSuccessPlanKPIsParams{
		Today:    pgtype.Date{Time: time.Now(), Valid: true},
		ScopeAll: false,
		IsOwn:    false, // fail-closed
		Uid:      &uid,
	})
	if err != nil {
		t.Fatalf("CountSuccessPlanKPIs: %v", err)
	}
	if k.ActiveCount != 0 || k.ActiveVillages != 0 || k.Overdue != 0 || k.DueSoon != 0 {
		t.Errorf("scope-none harus nol, got %+v", k)
	}
}

// --- successPlanKPIsToView -------------------------------------------------

func TestSuccessPlanKPIsToView(t *testing.T) {
	v := successPlanKPIsToView(db.CountSuccessPlanKPIsRow{
		ActiveCount:    12,
		ActiveVillages: 9,
		Overdue:        3,
		DueSoon:        5,
		AvgProgress:    67.5, // → 68
	})
	if v.ActiveCount != "12" {
		t.Errorf("ActiveCount = %q, want 12", v.ActiveCount)
	}
	if v.ActiveSub != "di 9 desa" {
		t.Errorf("ActiveSub = %q, want 'di 9 desa'", v.ActiveSub)
	}
	if v.Overdue != "3" || v.OverdueSub != "melewati tenggat" {
		t.Errorf("Overdue view salah: %q / %q", v.Overdue, v.OverdueSub)
	}
	if v.DueSoon != "5" || v.DueSoonSub != "perlu dikejar" {
		t.Errorf("DueSoon view salah: %q / %q", v.DueSoon, v.DueSoonSub)
	}
	if v.AvgProgress != "68%" {
		t.Errorf("AvgProgress = %q, want 68%% (bulat dari 67.5)", v.AvgProgress)
	}
	if v.AvgProgressSub != "rata capaian" {
		t.Errorf("AvgProgressSub = %q, want 'rata capaian'", v.AvgProgressSub)
	}
}

// --- ListSuccessPlans health JOIN -----------------------------------------

func TestListSuccessPlans_HealthJoin(t *testing.T) {
	env, uid := setupAccounts(t)

	// Desa dengan health score → HealthStatus terisi di baris plan.
	withHealth := env.seedAccount(t, "Desa Sehat", &uid, nil, nil)
	env.seedHealthScore(t, withHealth.ID, 85, "Healthy")
	env.seedPlanFull(t, withHealth.ID, "Plan Sehat", "Active", 50, dateDaysFromNow(10))

	// Desa tanpa health score → HealthStatus NULL.
	noHealth := env.seedAccount(t, "Desa Tanpa Skor", &uid, nil, nil)
	env.seedPlanFull(t, noHealth.ID, "Plan Kosong", "Active", 30, dateDaysFromNow(10))

	rows := env.allSuccessPlans(t)
	var gotHealthy, gotNil bool
	for _, r := range rows {
		switch r.PlanName {
		case "Plan Sehat":
			if r.HealthStatus == nil || *r.HealthStatus != "Healthy" {
				t.Errorf("Plan Sehat: HealthStatus = %v, want Healthy", r.HealthStatus)
			}
			if r.HealthScore == nil || *r.HealthScore != 85 {
				t.Errorf("Plan Sehat: HealthScore = %v, want 85", r.HealthScore)
			}
			gotHealthy = true
		case "Plan Kosong":
			if r.HealthStatus != nil {
				t.Errorf("Plan Kosong: HealthStatus = %v, want nil", *r.HealthStatus)
			}
			gotNil = true
		}
	}
	if !gotHealthy || !gotNil {
		t.Fatalf("baris uji tak lengkap: healthy=%v nil=%v", gotHealthy, gotNil)
	}
}

// --- render ----------------------------------------------------------------

// TestSuccessPlansDashboard_Render: HTML memuat elemen dasbor BL-97 & MENGHILANGKAN
// kolom lama sesuai keputusan (Nama Plan, Owner CS, Langkah).
func TestSuccessPlansDashboard_Render(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Render", &uid, nil, nil)
	env.seedHealthScore(t, a.ID, 30, "Critical")
	env.seedPlanFull(t, a.ID, "Plan Render", "Active", 55, dateDaysFromNow(7))

	req := accountsReq(http.MethodGet, "/w/test/success-plans", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SuccessPlansList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()

	// Ada: KPI, header baru, kolom mockup, aksi Buka.
	mustContain := []string{
		"Rencana Aktif", "Terlambat", "Jatuh Tempo 14 Hari", "Rata Capaian",
		"+ Rencana Baru", "Customer Success › Success Plans (6.3)",
		"Tujuan Rencana", "Capaian", "Tenggat", "Health",
		"rencana aktif · target terukur", // subtitle tabel
		"Buka",
	}
	for _, s := range mustContain {
		if !strings.Contains(body, s) {
			t.Errorf("body harus memuat %q", s)
		}
	}

	// Tak ada: kolom lama yang dibuang & kolom Langkah yang ditunda.
	mustNotContain := []string{
		"<th>Nama Plan</th>", "Owner CS", "Langkah", "+ Buat Plan",
	}
	for _, s := range mustNotContain {
		if strings.Contains(body, s) {
			t.Errorf("body TIDAK boleh memuat %q", s)
		}
	}
}
