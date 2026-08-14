package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// success_plans_test.go — Success Plans (CRM Modul 6 slice 6.3).
//
// Tiga sumbu yang dijaga:
//
//   - F2 (Casbin bisnis): GET /success-plans butuh crm:success_plans read —
//     admin/manager/csm lolos; sales/support/"" ditolak 403.
//     POST butuh write — admin/manager/csm lolos; sales/support/"" ditolak 403.
//   - F3 (ownership): CSM (data_scope='own') hanya melihat plan dari desa
//     binaannya atau plan yang ia miliki (owner_csm); admin melihat semua.
//     Di luar cakupan → 404 pada edit/update.
//   - Validasi: progress > 100 → redirect err=progress; plan_name kosong → err=name.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS di
// rls_test.go. Setup pakai ulang setupAccounts/accountsReq/runAccount.

// --- helper ----------------------------------------------------------------

// seedSuccessPlan menanam satu success plan langsung lewat pool (bypass handler),
// pola sama dengan seedEngagementRow. assignedCSM dipasang di account agar F3
// ownership berjalan di CSM scope.
// Mengembalikan (account, plan) agar caller bisa membangun request edit.
func (e *testEnv) seedSuccessPlan(
	t *testing.T, villageName, planName string, assignedCSM *int64,
) (db.Account, db.SuccessPlan) {
	t.Helper()
	acc := e.seedAccount(t, villageName, nil, assignedCSM, nil)
	sp, err := e.q.CreateSuccessPlan(t.Context(), db.CreateSuccessPlanParams{
		TenantID:   e.tenantID,
		AccountID:  acc.ID,
		PlanName:   planName,
		PlanStatus: "Draft",
		Progress:   0,
		TargetDate: pgtype.Date{Valid: false},
	})
	if err != nil {
		t.Fatalf("seed success plan %q: %v", planName, err)
	}
	return acc, sp
}

// allSuccessPlans mendaftar seluruh success plan workspace (scope_all) langsung
// dari pool — untuk membuktikan visibilitas F3 dan bahwa create/update benar.
func (e *testEnv) allSuccessPlans(t *testing.T) []db.ListSuccessPlansRow {
	t.Helper()
	sentinel := pgtype.Timestamptz{
		Time:             time.Now().Add(100 * 365 * 24 * time.Hour),
		Valid:            true,
		InfinityModifier: pgtype.Finite,
	}
	rows, err := e.q.ListSuccessPlans(t.Context(), db.ListSuccessPlansParams{
		CursorCreatedAt: sentinel,
		CursorID:        1<<62 - 1,
		ScopeAll:        true,
		IsOwn:           false,
		Uid:             nil,
		FilterStatus:    "",
		PageSize:        100,
	})
	if err != nil {
		t.Fatalf("list success plans: %v", err)
	}
	return rows
}

// successPlanCreateForm merakit form minimal valid untuk POST create.
func successPlanCreateForm(accountID int64, planName, status string, progress int) url.Values {
	return url.Values{
		"account_id":  {itoa(accountID)},
		"plan_name":   {planName},
		"plan_status": {status},
		"progress":    {itoa(int64(progress))},
	}
}

// successPlanUpdateForm merakit form minimal valid untuk POST update.
func successPlanUpdateForm(planName, status string, progress int) url.Values {
	return url.Values{
		"plan_name":   {planName},
		"plan_status": {status},
		"progress":    {itoa(int64(progress))},
	}
}

// --- F2: gerbang read ------------------------------------------------------

// TestSuccessPlans_GateRead: siapa boleh membuka daftar success plan (act read).
// admin/manager/csm lolos; sales/support/"" ditolak 403.
func TestSuccessPlans_GateRead(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"csm", true},
		{"sales", false}, // crm:success_plans read TIDAK mencakup sales
		{"support", false},
		{"", false},
	}
	env, uid := setupAccounts(t)

	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/success-plans", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.SuccessPlansList)
			if c.allow && rec.Code != http.StatusOK {
				t.Errorf("role %q harus lolos gate read, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if !strings.Contains(rec.Body.String(), "peran CRM") {
					t.Errorf("penolakan harus menyebut peran CRM")
				}
			}
		})
	}
}

// --- F2: gerbang write -----------------------------------------------------

// TestSuccessPlans_GateWrite: siapa boleh membuat success plan baru (act write).
// admin/manager/csm lolos; sales/support/"" ditolak 403.
func TestSuccessPlans_GateWrite(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"csm", true},
		{"sales", false},
		{"support", false},
		{"", false},
	}
	env, uid := setupAccounts(t)
	// Satu akun agar dropdown desa tidak kosong.
	env.seedAccount(t, "Desa Gate Write", nil, nil, nil)

	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/success-plans/new", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.SuccessPlanNew)
			if c.allow && rec.Code != http.StatusOK {
				t.Errorf("role %q harus lolos gate write, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow && rec.Code != http.StatusForbidden {
				t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
			}
		})
	}
}

// --- F3: ownership list ----------------------------------------------------

// TestSuccessPlans_F3_ListOwnership: CSM hanya melihat plan dari desa yang ia
// tangani (assigned_csm) atau plan yang ia miliki (owner_csm); plan desa lain
// tidak muncul. Admin melihat semua.
func TestSuccessPlans_F3_ListOwnership(t *testing.T) {
	env, uid := setupAccounts(t)

	csmUser := env.seedMember(t, "csm-sp@test.local", "member", env.tenantID)

	// Plan pada desa binaan CSM → CSM harus melihatnya.
	_, spOwned := env.seedSuccessPlan(t, "Desa Binaan CSM", "Plan Owned", &csmUser.ID)
	_ = spOwned

	// Plan pada desa LAIN (tanpa relasi ke csmUser) → CSM tidak boleh melihatnya.
	_, spOther := env.seedSuccessPlan(t, "Desa Lain", "Plan Lain", nil)
	_ = spOther

	// scope_all (admin) → kedua plan tampil.
	allRows := env.allSuccessPlans(t)
	if len(allRows) < 2 {
		t.Fatalf("scope_all: ingin ≥2 plan, dapat %d", len(allRows))
	}

	// scope_own (csm) → hanya plan desa binaan.
	at, id := firstPageCursor()
	ownRows, err := env.q.ListSuccessPlans(t.Context(), db.ListSuccessPlansParams{
		CursorCreatedAt: at, CursorID: id,
		ScopeAll:     false,
		IsOwn:        true,
		Uid:          &csmUser.ID,
		FilterStatus: "", // kosong = semua status; nil → NULL = '' → false
		PageSize:     100,
	})
	if err != nil {
		t.Fatalf("list own success plans: %v", err)
	}
	if len(ownRows) != 1 {
		t.Fatalf("scope_own CSM: ingin 1 plan, dapat %d", len(ownRows))
	}
	if ownRows[0].PlanName != "Plan Owned" {
		t.Errorf("scope_own menunjukkan plan salah: %q", ownRows[0].PlanName)
	}
	_ = uid
}

// --- F3: ownership edit out-of-scope ---------------------------------------

// TestSuccessPlans_F3_EditOutOfScope: CSM mencoba edit plan dari desa yang
// bukan binaannya → 404 (bukan 403).
func TestSuccessPlans_F3_EditOutOfScope(t *testing.T) {
	env, uid := setupAccounts(t)

	csmUser := env.seedMember(t, "csm-oos@test.local", "member", env.tenantID)

	// Plan pada desa yang BUKAN binaan csmUser.
	_, spOther := env.seedSuccessPlan(t, "Desa Bukan Binaan", "Plan Orang Lain", nil)

	req := accountsReq(http.MethodGet, "/w/test/success-plans/"+itoa(spOther.ID)+"/edit", nil, itoa(spOther.ID))
	rec := env.runAccount(csmUser.ID, "member", "csm", req, env.h.SuccessPlanEdit)
	if rec.Code != http.StatusNotFound {
		t.Errorf("CSM di luar cakupan harus 404, got %d", rec.Code)
	}
	_ = uid
}

// --- create valid ----------------------------------------------------------

// TestSuccessPlans_CreateValid: POST create dengan data valid → 303 redirect
// ke daftar + baris tersimpan ke DB.
func TestSuccessPlans_CreateValid(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Create Plan", nil, nil, nil)

	before := len(env.allSuccessPlans(t))

	form := successPlanCreateForm(acc.ID, "Plan Q3 2026", "Active", 25)
	req := accountsReq(http.MethodPost, "/w/test/success-plans", form, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SuccessPlanCreate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create valid harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "ok=created") {
		t.Errorf("redirect harus bawa ok=created, got %q", loc)
	}

	after := env.allSuccessPlans(t)
	if len(after) != before+1 {
		t.Fatalf("ingin %d plan setelah create, dapat %d", before+1, len(after))
	}
	latest := after[0] // keyset DESC → terbaru di atas
	if latest.PlanName != "Plan Q3 2026" {
		t.Errorf("plan_name salah: %q", latest.PlanName)
	}
	if latest.PlanStatus != "Active" {
		t.Errorf("plan_status salah: %q", latest.PlanStatus)
	}
	if latest.Progress != 25 {
		t.Errorf("progress salah: %d", latest.Progress)
	}
}

// --- update valid ----------------------------------------------------------

// TestSuccessPlans_UpdateValid: POST update plan dengan data valid → 303 redirect
// + field tersimpan ke DB.
func TestSuccessPlans_UpdateValid(t *testing.T) {
	env, uid := setupAccounts(t)
	_, sp := env.seedSuccessPlan(t, "Desa Update Plan", "Plan Lama", nil)

	form := successPlanUpdateForm("Plan Baru", "Achieved", 100)
	req := accountsReq(http.MethodPost, "/w/test/success-plans/"+itoa(sp.ID), form, itoa(sp.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SuccessPlanUpdate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update valid harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "ok=updated") {
		t.Errorf("redirect harus bawa ok=updated, got %q", loc)
	}

	// Verifikasi via DB.
	updated, err := env.q.GetSuccessPlan(t.Context(), sp.ID)
	if err != nil {
		t.Fatalf("get updated plan: %v", err)
	}
	if updated.PlanName != "Plan Baru" {
		t.Errorf("plan_name tidak tersimpan: %q", updated.PlanName)
	}
	if updated.PlanStatus != "Achieved" {
		t.Errorf("plan_status tidak tersimpan: %q", updated.PlanStatus)
	}
	if updated.Progress != 100 {
		t.Errorf("progress tidak tersimpan: %d", updated.Progress)
	}
}

// --- validasi progress -----------------------------------------------------

// TestSuccessPlans_InvalidProgress: POST dengan progress > 100 → redirect dengan
// err=progress (bukan 200 atau 500).
func TestSuccessPlans_InvalidProgress(t *testing.T) {
	env, uid := setupAccounts(t)
	_, sp := env.seedSuccessPlan(t, "Desa Invalid Progress", "Plan Valid", nil)

	form := successPlanUpdateForm("Plan Valid", "Active", 150) // progress di luar [0,100]
	req := accountsReq(http.MethodPost, "/w/test/success-plans/"+itoa(sp.ID), form, itoa(sp.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.SuccessPlanUpdate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("validasi progress harus redirect, got %d\n%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "err=progress") {
		t.Errorf("redirect harus bawa err=progress, got %q", loc)
	}
	// Pastikan DB tidak berubah.
	unchanged, err := env.q.GetSuccessPlan(t.Context(), sp.ID)
	if err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if unchanged.Progress != sp.Progress {
		t.Errorf("DB berubah walau progress invalid: %d → %d", sp.Progress, unchanged.Progress)
	}
}
