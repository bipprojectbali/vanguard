package handler

import (
	"net/http"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// subscriptions_renew_approve_test.go — keputusan Manager atas renewal Pending
// (approve/reject) + gerbang approve, dipisah dari subscriptions_renew_test.go
// (file health, batas 400 baris test). Reuse setupAccounts/accountsReq/runAccount
// (subscriptions_test.go) + redirectSubID (subscriptions_renew_test.go).

// seedPendingRenewal menaruh baris renewal 'PendingApproval' yang menunjuk baris
// lama (previous). Dipakai menguji approve/reject tanpa merangkai jalur renew.
func (e *testEnv) seedPendingRenewal(
	t *testing.T, accountID, planID int64, owner *int64, prevID int64, mrr, prevValue string,
) db.Subscription {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntitySubscription)
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	pending := "Pending"
	s, err := e.q.CreateSubscription(t.Context(), db.CreateSubscriptionParams{
		TenantID:               e.tenantID,
		EntityCode:             &code,
		SubscriptionOwner:      owner,
		AccountID:              accountID,
		PlanID:                 &planID,
		PreviousSubscriptionID: &prevID,
		Status:                 "PendingApproval",
		ApprovalStatus:         &pending,
		Mrr:                    numFrom(t, mrr),
		Arr:                    numFrom(t, ""),
		PreviousValue:          numFrom(t, prevValue),
		CreatedBy:              owner,
	})
	if err != nil {
		t.Fatalf("seed pending renewal: %v", err)
	}
	return s
}

// --- approve / reject ------------------------------------------------------

// TestSubscriptionRenewApprove_Activates: Manager menyetujui → baris Pending jadi
// Active, baris lama Expired, approved_by terisi, pemilik dinotifikasi.
func TestSubscriptionRenewApprove_Activates(t *testing.T) {
	env, uid := setupAccounts(t)
	rep := env.seedMember(t, "rep@local", "member", 0)
	planID := env.seedPlan(t, "Paket A", "PLAN-AP", "1000000")
	acc := env.seedAccount(t, "Desa A", &rep.ID, nil, nil)
	old := env.seedSubscription(t, acc.ID, planID, &rep.ID, "Active", "500000", "6000000")
	pending := env.seedPendingRenewal(t, acc.ID, planID, &rep.ID, old.ID, "800000", "500000")

	req := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(pending.ID)+"/approve", nil, itoa(pending.ID))
	rec := env.runAccount(uid, "owner", "manager", req, env.h.SubscriptionRenewApprove)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303\n%s", rec.Code, rec.Body.String())
	}
	if !contains(rec.Header().Get("Location"), "ok=renew_approved") {
		t.Errorf("Location harus ok=renew_approved, got %q", rec.Header().Get("Location"))
	}
	gotOld, _ := env.q.GetSubscription(t.Context(), old.ID)
	if gotOld.Status != "Expired" {
		t.Errorf("baris lama status = %q, want Expired", gotOld.Status)
	}
	gotNew, _ := env.q.GetSubscription(t.Context(), pending.ID)
	if gotNew.Status != "Active" {
		t.Errorf("baris pending status = %q, want Active", gotNew.Status)
	}
	if gotNew.ApprovalStatus == nil || *gotNew.ApprovalStatus != "Approved" {
		t.Errorf("approval_status = %v, want Approved", gotNew.ApprovalStatus)
	}
	if gotNew.ApprovedBy == nil || *gotNew.ApprovedBy != uid {
		t.Errorf("approved_by = %v, want %d", gotNew.ApprovedBy, uid)
	}
	if n, _ := env.q.CountUnreadNotifications(t.Context(), rep.ID); n < 1 {
		t.Errorf("pemilik harus dinotifikasi persetujuan (unread=%d)", n)
	}
}

// TestSubscriptionRenewReject_Cancels: Manager menolak → baris Pending jadi
// Cancelled (approval Rejected); baris lama TAK disentuh (tetap Active).
func TestSubscriptionRenewReject_Cancels(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket J", "PLAN-RJ", "1000000")
	acc := env.seedAccount(t, "Desa J", &uid, nil, nil)
	old := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")
	pending := env.seedPendingRenewal(t, acc.ID, planID, &uid, old.ID, "800000", "500000")

	req := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(pending.ID)+"/reject", nil, itoa(pending.ID))
	rec := env.runAccount(uid, "owner", "manager", req, env.h.SubscriptionRenewReject)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303\n%s", rec.Code, rec.Body.String())
	}
	if !contains(rec.Header().Get("Location"), "ok=renew_rejected") {
		t.Errorf("Location harus ok=renew_rejected, got %q", rec.Header().Get("Location"))
	}
	gotNew, _ := env.q.GetSubscription(t.Context(), pending.ID)
	if gotNew.Status != "Cancelled" {
		t.Errorf("baris pending status = %q, want Cancelled", gotNew.Status)
	}
	if gotNew.ApprovalStatus == nil || *gotNew.ApprovalStatus != "Rejected" {
		t.Errorf("approval_status = %v, want Rejected", gotNew.ApprovalStatus)
	}
	gotOld, _ := env.q.GetSubscription(t.Context(), old.ID)
	if gotOld.Status != "Active" {
		t.Errorf("baris lama status = %q, want Active (reject tak menyentuhnya)", gotOld.Status)
	}
}

// --- gate --------------------------------------------------------------

// TestSubscriptionRenewApprove_Gate: approve butuh crm:renewals APPROVE (BL-145
// subtask 2, dipindah dari crm:renewal_mgmt). csm (punya renewal_mgmt write tapi
// bukan approve) & sales ditolak 403; manager lolos.
func TestSubscriptionRenewApprove_Gate(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket AG", "PLAN-AG", "1000000")
	acc := env.seedAccount(t, "Desa AG", &uid, nil, nil)
	old := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")
	pending := env.seedPendingRenewal(t, acc.ID, planID, &uid, old.ID, "800000", "500000")

	for _, role := range []string{"sales", "csm"} {
		req := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(pending.ID)+"/approve", nil, itoa(pending.ID))
		rec := env.runAccount(uid, "owner", role, req, env.h.SubscriptionRenewApprove)
		if rec.Code != http.StatusForbidden {
			t.Errorf("role %q harus 403 saat approve, got %d", role, rec.Code)
		}
	}
	req := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(pending.ID)+"/approve", nil, itoa(pending.ID))
	rec := env.runAccount(uid, "owner", "manager", req, env.h.SubscriptionRenewApprove)
	if rec.Code == http.StatusForbidden {
		t.Errorf("manager harus lolos gate approve, got 403")
	}
}
