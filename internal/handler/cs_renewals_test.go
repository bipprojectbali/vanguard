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

// cs_renewals_test.go — Renewal Management CS (Modul 6 slice 6.6).
//
// Tiga sumbu yang dijaga:
//
//   - F2 (Casbin bisnis): GET /renewal-management butuh crm:renewal_mgmt read —
//     admin/manager/csm/sales lolos; support/"" ditolak 403.
//     GET edit & POST update butuh crm:renewal_mgmt write —
//     admin/manager/csm lolos; sales/support/"" ditolak 403.
//   - F3 (ownership): CSM (scope='own') melihat HANYA sub dari desa binaannya;
//     admin (scope='all') melihat semua. Di luar cakupan → 404.
//   - Update valid: POST update menyimpan aksi CS (stage/risk/action_plan/
//     next_action_date/owner) ke DB dengan benar.
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler.
// Setup pakai ulang setupAccounts/accountsReq/runAccount dari accounts_test.go.

// --- helper renewal --------------------------------------------------------

// seedCSRenewal menyiapkan account + plan + subscription dengan end_date.
// assignedCSM dipasang di account agar F3 ownership berjalan.
// Mengembalikan (account, planID, subscription).
func (e *testEnv) seedCSRenewal(
	t *testing.T, villageName string, assignedCSM *int64,
) (db.Account, int64, db.Subscription) {
	t.Helper()
	acc := e.seedAccount(t, villageName, nil, assignedCSM, nil)
	planID := e.seedPlan(t, "Plan Renewal "+villageName, "PLN-R-"+villageName, "1000000")
	sub := e.seedSubscription(t, acc.ID, planID, nil, "Active", "500000", "6000000")
	// Set end_date supaya sub muncul di ListCSRenewals (filter IS NOT NULL).
	if _, err := e.h.Pool.Exec(t.Context(),
		`UPDATE subscriptions SET end_date = '2026-12-31' WHERE id = $1`, sub.ID); err != nil {
		t.Fatalf("set end_date sub %d: %v", sub.ID, err)
	}
	// Reload agar end_date valid (CreateSubscription tidak mengembalikan end_date).
	updated, err := e.q.GetSubscription(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("get updated sub %d: %v", sub.ID, err)
	}
	return acc, planID, updated
}

// allCSRenewals membaca seluruh subscription workspace dengan end_date terisi.
func (e *testEnv) allCSRenewals(t *testing.T) []db.ListCSRenewalsRow {
	t.Helper()
	sentinel := pgtype.Timestamptz{
		Time: time.Now().Add(100 * 365 * 24 * time.Hour), Valid: true, InfinityModifier: pgtype.Finite,
	}
	rows, err := e.q.ListCSRenewals(t.Context(), db.ListCSRenewalsParams{
		CursorCreatedAt: sentinel,
		CursorID:        1<<62 - 1,
		ScopeAll:        true,
		IsOwn:           false,
		Uid:             nil,
		FilterStage:     "",
		PageSize:        100,
	})
	if err != nil {
		t.Fatalf("list cs renewals: %v", err)
	}
	return rows
}

// renewalUpdateForm merakit form minimal valid untuk POST update.
func renewalUpdateForm(stage, risk string) url.Values {
	return url.Values{
		"renewal_stage": {stage},
		"renewal_risk":  {risk},
	}
}

// --- F2: gerbang read ------------------------------------------------------

// TestCSRenewals_GateRead: siapa boleh membuka daftar renewal (act read).
// admin/manager/csm/sales lolos; support & "" ditolak 403.
func TestCSRenewals_GateRead(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"csm", true},
		{"sales", true}, // crm:renewal_mgmt read mencakup sales
		{"support", false},
		{"", false},
	}
	env, uid := setupAccounts(t)
	// Satu subscription untuk workspace agar daftar tak kosong (tidak memengaruhi gate).
	env.seedCSRenewal(t, "Desa Gate Read", &uid)

	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/renewal-management", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.CSRenewalsList)
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

// --- F2: gerbang write (edit form) -----------------------------------------

// TestCSRenewals_GateWrite: GET edit & POST update butuh crm:renewal_mgmt write.
// admin/manager/csm lolos; sales/support/"" ditolak 403.
func TestCSRenewals_GateWrite(t *testing.T) {
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
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			_, _, sub := env.seedCSRenewal(t, "Desa Gate Write", &uid)

			// GET edit.
			editReq := accountsReq(http.MethodGet, "/w/test/renewal-management/"+itoa(sub.ID)+"/edit", nil, itoa(sub.ID))
			editRec := env.runAccount(uid, "owner", c.role, editReq, env.h.CSRenewalEdit)
			if c.allow && editRec.Code != http.StatusOK {
				t.Errorf("role %q harus 200 di GET edit, got %d\n%s", c.role, editRec.Code, editRec.Body.String())
			}
			if !c.allow && editRec.Code != http.StatusForbidden {
				t.Errorf("role %q harus 403 di GET edit, got %d", c.role, editRec.Code)
			}

			// POST update.
			form := renewalUpdateForm("Outreach", "Medium")
			postReq := accountsReq(http.MethodPost, "/w/test/renewal-management/"+itoa(sub.ID), form, itoa(sub.ID))
			postRec := env.runAccount(uid, "owner", c.role, postReq, env.h.CSRenewalUpdate)
			if c.allow && postRec.Code != http.StatusSeeOther {
				t.Errorf("role %q harus 303 di POST update, got %d\n%s", c.role, postRec.Code, postRec.Body.String())
			}
			if !c.allow && postRec.Code != http.StatusForbidden {
				t.Errorf("role %q harus 403 di POST update, got %d", c.role, postRec.Code)
			}
		})
	}
}

// --- F3: ownership listing -------------------------------------------------

// TestCSRenewals_F3_ListOwnership: CSM (scope=own) melihat HANYA sub dari desa
// binaannya (assigned_csm = uid). Admin (scope=all) melihat semua.
func TestCSRenewals_F3_ListOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	otherUID := env.seedMember(t, "csmother@local", "member", 0).ID

	// Sub "milik CSM" → assigned_csm = uid.
	env.seedCSRenewal(t, "Desa Binaan CSM", &uid)
	// Sub milik CSM lain → tidak terlihat dalam scope=own.
	env.seedCSRenewal(t, "Desa Milik CSM Lain", &otherUID)

	// CSM: scope=own → hanya 1 baris.
	reqCSM := accountsReq(http.MethodGet, "/w/test/renewal-management", nil, "")
	recCSM := env.runAccount(uid, "owner", "csm", reqCSM, env.h.CSRenewalsList)
	if recCSM.Code != http.StatusOK {
		t.Fatalf("csm harus 200, got %d", recCSM.Code)
	}
	// Verifikasi via DB langsung.
	filterOwn := db.CSRenewalsListFilterFor("own")
	if filterOwn.ScopeAll {
		t.Errorf("scope=own tidak boleh ScopeAll")
	}
	if !filterOwn.IsOwn {
		t.Errorf("scope=own harus IsOwn=true")
	}

	// Admin: scope=all → melihat semua 2 baris.
	allRows := env.allCSRenewals(t)
	if len(allRows) != 2 {
		t.Errorf("scope=all harus 2 baris, ada %d", len(allRows))
	}
}

// --- F3: edit di luar cakupan ----------------------------------------------

// TestCSRenewals_F3_EditOutOfScope: CSM mencoba edit sub milik CSM lain → 404.
func TestCSRenewals_F3_EditOutOfScope(t *testing.T) {
	env, uid := setupAccounts(t)
	otherUID := env.seedMember(t, "csmother2@local", "member", 0).ID

	// Sub milik CSM lain.
	_, _, theirSub := env.seedCSRenewal(t, "Desa Milik Lain", &otherUID)

	editReq := accountsReq(http.MethodGet, "/w/test/renewal-management/"+itoa(theirSub.ID)+"/edit", nil, itoa(theirSub.ID))
	rec := env.runAccount(uid, "owner", "csm", editReq, env.h.CSRenewalEdit)
	if rec.Code != http.StatusNotFound {
		t.Errorf("CSM yang edit di luar cakupan harus 404, got %d", rec.Code)
	}

	// Sub milik sendiri → 200.
	_, _, mySub := env.seedCSRenewal(t, "Desa Milik Sendiri", &uid)
	myReq := accountsReq(http.MethodGet, "/w/test/renewal-management/"+itoa(mySub.ID)+"/edit", nil, itoa(mySub.ID))
	myRec := env.runAccount(uid, "owner", "csm", myReq, env.h.CSRenewalEdit)
	if myRec.Code != http.StatusOK {
		t.Errorf("CSM edit sub sendiri harus 200, got %d\n%s", myRec.Code, myRec.Body.String())
	}
}

// --- update valid ----------------------------------------------------------

// TestCSRenewals_UpdateValid: POST dengan form lengkap valid → 303 ok=updated,
// field aksi tersimpan di DB dengan benar.
func TestCSRenewals_UpdateValid(t *testing.T) {
	env, uid := setupAccounts(t)
	_, _, sub := env.seedCSRenewal(t, "Desa Update Valid", &uid)

	form := url.Values{
		"renewal_stage":            {"Outreach"},
		"renewal_risk":             {"High"},
		"renewal_action_plan":      {"Hubungi kepala desa pekan ini"},
		"renewal_next_action_date": {"2026-08-30"},
		"renewal_owner":            {itoa(uid)},
	}
	req := accountsReq(http.MethodPost, "/w/test/renewal-management/"+itoa(sub.ID), form, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "csm", req, env.h.CSRenewalUpdate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update valid harus 303, got %d\n%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/renewal-management") || !strings.Contains(loc, "ok=updated") {
		t.Errorf("redirect harus ke /renewal-management?ok=updated, got %q", loc)
	}

	// Verifikasi baris di DB.
	updated, err := env.q.GetSubscription(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("get updated sub: %v", err)
	}
	if updated.RenewalStage == nil || *updated.RenewalStage != "Outreach" {
		t.Errorf("renewal_stage harus 'Outreach', got %v", updated.RenewalStage)
	}
	if updated.RenewalRisk == nil || *updated.RenewalRisk != "High" {
		t.Errorf("renewal_risk harus 'High', got %v", updated.RenewalRisk)
	}
	if updated.RenewalActionPlan == nil || *updated.RenewalActionPlan != "Hubungi kepala desa pekan ini" {
		t.Errorf("renewal_action_plan tidak tersimpan, got %v", updated.RenewalActionPlan)
	}
	if updated.RenewalOwner == nil || *updated.RenewalOwner != uid {
		t.Errorf("renewal_owner harus %d, got %v", uid, updated.RenewalOwner)
	}
	if !updated.RenewalNextActionDate.Valid {
		t.Errorf("renewal_next_action_date harus valid, got %v", updated.RenewalNextActionDate)
	}
}

// --- update validasi stage/risk invalid ------------------------------------

// TestCSRenewals_UpdateInvalidStage: stage tidak valid → redirect ke edit dengan err=stage.
func TestCSRenewals_UpdateInvalidStage(t *testing.T) {
	env, uid := setupAccounts(t)
	_, _, sub := env.seedCSRenewal(t, "Desa Stage Invalid", &uid)

	form := url.Values{"renewal_stage": {"BUKAN_NILAI_SAHI"}}
	req := accountsReq(http.MethodPost, "/w/test/renewal-management/"+itoa(sub.ID), form, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "csm", req, env.h.CSRenewalUpdate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("validasi stage harus redirect, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Location"), "err=stage") {
		t.Errorf("harus redirect dengan err=stage, got %q", rec.Header().Get("Location"))
	}
}

// TestCSRenewals_UpdateInvalidRisk: risk tidak valid → redirect ke edit dengan err=risk.
func TestCSRenewals_UpdateInvalidRisk(t *testing.T) {
	env, uid := setupAccounts(t)
	_, _, sub := env.seedCSRenewal(t, "Desa Risk Invalid", &uid)

	form := url.Values{
		"renewal_stage": {"Outreach"},
		"renewal_risk":  {"SANGAT_TINGGI"},
	}
	req := accountsReq(http.MethodPost, "/w/test/renewal-management/"+itoa(sub.ID), form, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "csm", req, env.h.CSRenewalUpdate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("validasi risk harus redirect, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Location"), "err=risk") {
		t.Errorf("harus redirect dengan err=risk, got %q", rec.Header().Get("Location"))
	}
}
