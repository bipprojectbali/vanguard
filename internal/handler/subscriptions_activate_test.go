package handler

import (
	"net/http"
	"net/url"
	"testing"
)

// subscriptions_activate_test.go — aktivasi langganan Trial → Active (BL-73) di
// sisi handler. Yang dikunci: (1) Trial→Active sukses + audit, (2) sudah ada Active
// plan sama → sub_dup_active & Trial tak berubah (invarian idx_subs_one_active),
// (3) status non-Trial → sub_not_trial, (4) gate = crm:renewals write (sales ditolak,
// manager lolos). F3 ownership sudah diliput pola loadOwnedSubscription (churn test).

// TestSubscriptionActivate_Success: manager mengaktifkan langganan Trial → status
// Active + aksi tercatat di audit_logs.
func TestSubscriptionActivate_Success(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket A1", "PLAN-A1", "1000000")
	acc := env.seedAccount(t, "Desa A1", &uid, nil, nil)
	sub := env.seedSubscription(t, acc.ID, planID, &uid, "Trial", "500000", "6000000")

	req := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(sub.ID)+"/activate", url.Values{}, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "manager", req, env.h.SubscriptionActivate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303\n%s", rec.Code, rec.Body.String())
	}
	if !contains(rec.Header().Get("Location"), "ok=activated") {
		t.Errorf("Location harus ok=activated, got %q", rec.Header().Get("Location"))
	}
	got, err := env.q.GetSubscription(t.Context(), sub.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "Active" {
		t.Errorf("status = %q, want Active", got.Status)
	}
	env.assertAudited(t, "subscription.activated")
}

// TestSubscriptionActivate_DuplicateActive: sudah ada langganan Active untuk (desa,
// paket) yang sama → aktivasi Trial ditolak (sub_dup_active), Trial TETAP Trial.
// Cek satu-Active WAJIB di sini: idx_subs_one_active hanya menjaga status='Active'
// sehingga Trial boleh koeksis, tapi menaikkannya akan melanggar partial-unique.
func TestSubscriptionActivate_DuplicateActive(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket A2", "PLAN-A2", "1000000")
	acc := env.seedAccount(t, "Desa A2", &uid, nil, nil)
	// Satu Active + satu Trial untuk (account, plan) yang SAMA.
	env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")
	trial := env.seedSubscription(t, acc.ID, planID, &uid, "Trial", "500000", "6000000")

	req := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(trial.ID)+"/activate", url.Values{}, itoa(trial.ID))
	rec := env.runAccount(uid, "owner", "manager", req, env.h.SubscriptionActivate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if !contains(rec.Header().Get("Location"), "err=sub_dup_active") {
		t.Errorf("Location harus err=sub_dup_active, got %q", rec.Header().Get("Location"))
	}
	got, _ := env.q.GetSubscription(t.Context(), trial.ID)
	if got.Status != "Trial" {
		t.Errorf("Trial tak boleh berubah saat konflik; status = %q", got.Status)
	}
}

// TestSubscriptionActivate_NotTrial: langganan non-Trial (mis. Active) → ditolak
// sub_not_trial, status tak berubah.
func TestSubscriptionActivate_NotTrial(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket A3", "PLAN-A3", "1000000")
	acc := env.seedAccount(t, "Desa A3", &uid, nil, nil)
	sub := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")

	req := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(sub.ID)+"/activate", url.Values{}, itoa(sub.ID))
	rec := env.runAccount(uid, "owner", "manager", req, env.h.SubscriptionActivate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", rec.Code)
	}
	if !contains(rec.Header().Get("Location"), "err=sub_not_trial") {
		t.Errorf("Location harus err=sub_not_trial, got %q", rec.Header().Get("Location"))
	}
	got, _ := env.q.GetSubscription(t.Context(), sub.ID)
	if got.Status != "Active" {
		t.Errorf("status = %q, want Active (tak berubah)", got.Status)
	}
}

// TestSubscriptionActivate_Gate: aktivasi butuh crm:renewals write (sekelas renewal).
// sales (renewals read saja) → 403; manager (write) lolos gate (bukan 403).
func TestSubscriptionActivate_Gate(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket AG", "PLAN-AG", "1000000")
	acc := env.seedAccount(t, "Desa AG", &uid, nil, nil)
	sub := env.seedSubscription(t, acc.ID, planID, &uid, "Trial", "500000", "6000000")

	reqSales := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(sub.ID)+"/activate", url.Values{}, itoa(sub.ID))
	if rec := env.runAccount(uid, "owner", "sales", reqSales, env.h.SubscriptionActivate); rec.Code != http.StatusForbidden {
		t.Errorf("sales harus 403 saat aktivasi, got %d", rec.Code)
	}
	reqMgr := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(sub.ID)+"/activate", url.Values{}, itoa(sub.ID))
	if rec := env.runAccount(uid, "owner", "manager", reqMgr, env.h.SubscriptionActivate); rec.Code == http.StatusForbidden {
		t.Errorf("manager harus lolos gate aktivasi, got 403")
	}
}
