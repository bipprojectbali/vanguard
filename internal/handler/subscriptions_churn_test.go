package handler

import (
	"net/http"
	"net/url"
	"testing"
)

// subscriptions_churn_test.go — churn langganan (M5-3c) di sisi handler. Yang
// dikunci: churn butuh crm:churn write (sales ditolak), hanya Active/Trial yang
// bisa di-churn, domain alasan/tipe divalidasi, dan F3 (di luar cakupan → 404).
// CSM sengaja dipakai sbg subjek utama: ia PUNYA churn write TAPI cakupan 'own'
// → satu-satunya peran yang bisa memicu jalur 404 F3 pada aksi ini.

// TestSubscriptionChurn_Success: CSM men-churn langganan yang ia tangani → status
// Churned + kolom churn terisi (lost_value_mrr = MRR saat ini).
func TestSubscriptionChurn_Success(t *testing.T) {
	env, uid := setupAccounts(t)
	csm := env.seedMember(t, "csm@local", "member", 0)
	planID := env.seedPlan(t, "Paket C", "PLAN-C", "1000000")
	acc := env.seedAccount(t, "Desa C", nil, &csm.ID, nil) // CSM ditugaskan
	sub := env.seedSubscription(t, acc.ID, planID, &csm.ID, "Active", "500000", "6000000")

	form := url.Values{
		"churn_reason":      {"Budget"},
		"churn_type":        {"Voluntary"},
		"churn_notes":       {"Anggaran desa dipangkas"},
		"win_back_eligible": {"1"},
	}
	req := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(sub.ID)+"/churn", form, itoa(sub.ID))
	rec := env.runAccount(csm.ID, "member", "csm", req, env.h.SubscriptionChurn)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303\n%s", rec.Code, rec.Body.String())
	}
	if !contains(rec.Header().Get("Location"), "ok=churned") {
		t.Errorf("Location harus ok=churned, got %q", rec.Header().Get("Location"))
	}
	got, err := env.q.GetSubscription(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "Churned" {
		t.Errorf("status = %q, want Churned", got.Status)
	}
	if got.ChurnReason == nil || *got.ChurnReason != "Budget" {
		t.Errorf("churn_reason = %v, want Budget", got.ChurnReason)
	}
	if got.ChurnType == nil || *got.ChurnType != "Voluntary" {
		t.Errorf("churn_type = %v, want Voluntary", got.ChurnType)
	}
	if got.WinBackEligible == nil || !*got.WinBackEligible {
		t.Errorf("win_back_eligible = %v, want true", got.WinBackEligible)
	}
	if !numEq(got.LostValueMrr, "500000") {
		t.Errorf("lost_value_mrr harus = MRR saat ini (500000)")
	}
	_ = uid
}

// TestSubscriptionChurn_Gate: churn butuh crm:churn write. sales (read saja) → 403;
// csm & manager lolos gate (bukan 403).
func TestSubscriptionChurn_Gate(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket CG", "PLAN-CG", "1000000")
	acc := env.seedAccount(t, "Desa CG", &uid, nil, nil)
	sub := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")

	form := url.Values{"churn_reason": {"Budget"}}
	reqSales := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(sub.ID)+"/churn", form, itoa(sub.ID))
	if rec := env.runAccount(uid, "owner", "sales", reqSales, env.h.SubscriptionChurn); rec.Code != http.StatusForbidden {
		t.Errorf("sales harus 403 saat churn, got %d", rec.Code)
	}
	// manager (all-scope, churn write) lolos gate.
	reqMgr := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(sub.ID)+"/churn", form, itoa(sub.ID))
	if rec := env.runAccount(uid, "owner", "manager", reqMgr, env.h.SubscriptionChurn); rec.Code == http.StatusForbidden {
		t.Errorf("manager harus lolos gate churn, got 403")
	}
}

// TestSubscriptionChurn_OutOfScope404: CSM (cakupan 'own') men-churn langganan yang
// BUKAN miliknya → 404 (F3: keberadaan baris tak diungkap), bukan 403.
func TestSubscriptionChurn_OutOfScope404(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other@local", "member", 0)
	csm := env.seedMember(t, "csm2@local", "member", 0)
	planID := env.seedPlan(t, "Paket O", "PLAN-CO", "1000000")
	acc := env.seedAccount(t, "Desa O", &other.ID, nil, nil) // ditangani orang lain
	sub := env.seedSubscription(t, acc.ID, planID, &other.ID, "Active", "500000", "6000000")

	form := url.Values{"churn_reason": {"Budget"}}
	req := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(sub.ID)+"/churn", form, itoa(sub.ID))
	rec := env.runAccount(csm.ID, "member", "csm", req, env.h.SubscriptionChurn)
	if rec.Code != http.StatusNotFound {
		t.Errorf("CSM di luar cakupan harus 404, got %d", rec.Code)
	}
	_ = uid
}

// TestSubscriptionChurn_InvalidReason: alasan di luar domain → redirect ?err=
// churn_reason, langganan TIDAK berubah.
func TestSubscriptionChurn_InvalidReason(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket IR", "PLAN-IR", "1000000")
	acc := env.seedAccount(t, "Desa IR", &uid, nil, nil)
	sub := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")

	form := url.Values{"churn_reason": {"Bukan Alasan Valid"}}
	req := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(sub.ID)+"/churn", form, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "manager", req, env.h.SubscriptionChurn)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if !contains(rec.Header().Get("Location"), "err=churn_reason") {
		t.Errorf("Location harus err=churn_reason, got %q", rec.Header().Get("Location"))
	}
	got, _ := env.q.GetSubscription(t.Context(), sub.ID)
	if got.Status != "Active" {
		t.Errorf("langganan tak boleh berubah saat validasi gagal; status = %q", got.Status)
	}
}

// TestSubscriptionChurn_NotActive: langganan Expired/Churned tak bisa di-churn →
// ?err=sub_not_active.
func TestSubscriptionChurn_NotActive(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket NA", "PLAN-NA", "1000000")
	acc := env.seedAccount(t, "Desa NA", &uid, nil, nil)
	sub := env.seedSubscription(t, acc.ID, planID, &uid, "Expired", "500000", "6000000")

	form := url.Values{"churn_reason": {"Budget"}}
	req := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(sub.ID)+"/churn", form, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "manager", req, env.h.SubscriptionChurn)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if !contains(rec.Header().Get("Location"), "err=sub_not_active") {
		t.Errorf("Location harus err=sub_not_active, got %q", rec.Header().Get("Location"))
	}
}
