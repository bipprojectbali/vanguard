package handler

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// subscriptions_renew_test.go — mutasi renewal (M5-3c) di sisi handler: perpanjang
// (straight & upsell), setujui, tolak, plus gerbang bisnis. Invarian yang dikunci:
// renewal = INSERT baris baru; straight → baris lama Expired lebih dulu; upsell →
// baris lama TETAP Active sampai Manager menyetujui (idx_subs_one_active).
//
// Koneksi test = superuser (bypass RLS) → uji LOGIKA handler. Reuse setupAccounts/
// accountsReq/runAccount + seedSubscription (subscriptions_test.go).

var reSubID = regexp.MustCompile(`/subscriptions/(\d+)`)

// redirectSubID mengekstrak id langganan dari header Location redirect PRG. Gagal
// → fatal (tes tak bisa lanjut tanpa id baris baru).
func redirectSubID(t *testing.T, loc string) int64 {
	t.Helper()
	m := reSubID.FindStringSubmatch(loc)
	if m == nil {
		t.Fatalf("Location %q tak memuat id langganan", loc)
	}
	id, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		t.Fatalf("parse id dari %q: %v", loc, err)
	}
	return id
}

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

// --- renew: straight -------------------------------------------------------

// TestSubscriptionRenew_Straight: perpanjang tanpa menaikkan MRR → baris lama
// Expired, baris baru Active dgn previous_subscription_id + previous_value snapshot.
// ARR baru = MRR × 12.
func TestSubscriptionRenew_Straight(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket R", "PLAN-R", "1000000")
	acc := env.seedAccount(t, "Desa R", &uid, nil, nil)
	old := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")

	form := url.Values{"new_mrr": {""}} // kosong = ikut MRR lama
	req := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(old.ID)+"/renew", form, itoa(old.ID))
	rec := env.runAccount(uid, "owner", "manager", req, env.h.SubscriptionRenew)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303\n%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !contains(loc, "ok=renewed") {
		t.Errorf("Location %q harus memuat ok=renewed", loc)
	}
	// Baris lama harus Expired.
	got, err := env.q.GetSubscription(t.Context(), old.ID)
	if err != nil {
		t.Fatalf("get old: %v", err)
	}
	if got.Status != "Expired" {
		t.Errorf("baris lama status = %q, want Expired", got.Status)
	}
	// Baris baru Active + tautan rantai.
	newID := redirectSubID(t, loc)
	if newID == old.ID {
		t.Fatal("renew harus membuat baris BARU, bukan mengubah yang lama")
	}
	fresh, err := env.q.GetSubscription(t.Context(), newID)
	if err != nil {
		t.Fatalf("get new: %v", err)
	}
	if fresh.Status != "Active" {
		t.Errorf("baris baru status = %q, want Active", fresh.Status)
	}
	if fresh.PreviousSubscriptionID == nil || *fresh.PreviousSubscriptionID != old.ID {
		t.Errorf("previous_subscription_id = %v, want %d", fresh.PreviousSubscriptionID, old.ID)
	}
	if !numEq(fresh.PreviousValue, "500000") {
		t.Errorf("previous_value harus snapshot MRR lama 500000")
	}
	if !numEq(fresh.Arr, "6000000") {
		t.Errorf("ARR baru harus MRR×12 = 6000000")
	}
}

// --- renew: upsell ---------------------------------------------------------

// TestSubscriptionRenew_Upsell: MRR baru > lama → baris baru 'PendingApproval'
// (approval_status Pending), baris lama TETAP Active (belum disetujui), dan setiap
// Manager workspace menerima notifikasi.
func TestSubscriptionRenew_Upsell(t *testing.T) {
	env, uid := setupAccounts(t)
	mgr := env.seedMember(t, "mgr@local", "member", 0)
	if err := env.q.UpdateMemberBusinessRole(t.Context(), db.UpdateMemberBusinessRoleParams{
		UserID: mgr.ID, TenantID: env.tenantID, BusinessRole: ptr("manager"),
	}); err != nil {
		t.Fatalf("set manager business_role: %v", err)
	}
	planID := env.seedPlan(t, "Paket U", "PLAN-U", "1000000")
	acc := env.seedAccount(t, "Desa U", &uid, nil, nil)
	old := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")

	form := url.Values{"new_mrr": {"800000"}}
	req := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(old.ID)+"/renew", form, itoa(old.ID))
	rec := env.runAccount(uid, "owner", "manager", req, env.h.SubscriptionRenew)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303\n%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !contains(loc, "ok=renew_pending") {
		t.Errorf("Location %q harus memuat ok=renew_pending", loc)
	}
	// Baris lama TETAP Active — upsell belum disetujui.
	got, err := env.q.GetSubscription(t.Context(), old.ID)
	if err != nil {
		t.Fatalf("get old: %v", err)
	}
	if got.Status != "Active" {
		t.Errorf("baris lama status = %q, want Active (upsell belum disetujui)", got.Status)
	}
	// Baris baru PendingApproval + approval_status Pending.
	fresh, err := env.q.GetSubscription(t.Context(), redirectSubID(t, loc))
	if err != nil {
		t.Fatalf("get new: %v", err)
	}
	if fresh.Status != "PendingApproval" {
		t.Errorf("baris baru status = %q, want PendingApproval", fresh.Status)
	}
	if fresh.ApprovalStatus == nil || *fresh.ApprovalStatus != "Pending" {
		t.Errorf("approval_status = %v, want Pending", fresh.ApprovalStatus)
	}
	if !numEq(fresh.Mrr, "800000") {
		t.Errorf("MRR baru harus 800000")
	}
	// Manager menerima notifikasi.
	n, err := env.q.CountUnreadNotifications(t.Context(), mgr.ID)
	if err != nil {
		t.Fatalf("count notif: %v", err)
	}
	if n < 1 {
		t.Errorf("Manager harus menerima notifikasi renewal upsell (unread=%d)", n)
	}
}

// --- renew: pengisian periode (BL-126) -------------------------------------

// TestSubscriptionRenew_FillsDates: renewal (straight & upsell) WAJIB mengisi
// start_date/end_date — dulu kosong sehingga detail langganan hasil perpanjangan
// menampilkan Mulai/Berakhir kosong (BL-126). Periode = hari ini + termin lama;
// seedSubscription tak set contract_term_months → fallback 12 bulan.
func TestSubscriptionRenew_FillsDates(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket D", "PLAN-D", "1000000")
	acc := env.seedAccount(t, "Desa D", &uid, nil, nil)

	assertPeriod := func(t *testing.T, s db.Subscription, months int) {
		t.Helper()
		if !s.StartDate.Valid {
			t.Fatal("start_date renewal kosong (BL-126: wajib terisi)")
		}
		if !s.EndDate.Valid {
			t.Fatal("end_date renewal kosong (BL-126: wajib terisi)")
		}
		want := s.StartDate.Time.AddDate(0, months, 0)
		if !s.EndDate.Time.Equal(want) {
			t.Errorf("end_date = %s, want start + %d bln = %s",
				s.EndDate.Time.Format("2006-01-02"), months, want.Format("2006-01-02"))
		}
	}

	// Straight: baris Active baru harus punya periode.
	oldS := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")
	req := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(oldS.ID)+"/renew",
		url.Values{"new_mrr": {""}}, itoa(oldS.ID))
	rec := env.runAccount(uid, "owner", "manager", req, env.h.SubscriptionRenew)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("straight status = %d, want 303\n%s", rec.Code, rec.Body.String())
	}
	fresh, err := env.q.GetSubscription(t.Context(), redirectSubID(t, rec.Header().Get("Location")))
	if err != nil {
		t.Fatalf("get straight new: %v", err)
	}
	assertPeriod(t, fresh, monthsPerYear)

	// Upsell: baris PendingApproval juga harus punya periode (bertahan lewat approve).
	// Akun terpisah — hindari bentrok idx_subscription_items_one_active dgn baris
	// Active hasil renewal straight di atas (satu item aktif per akun+paket).
	accU := env.seedAccount(t, "Desa DU", &uid, nil, nil)
	oldU := env.seedSubscription(t, accU.ID, planID, &uid, "Active", "500000", "6000000")
	req = accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(oldU.ID)+"/renew",
		url.Values{"new_mrr": {"800000"}}, itoa(oldU.ID))
	rec = env.runAccount(uid, "owner", "manager", req, env.h.SubscriptionRenew)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("upsell status = %d, want 303\n%s", rec.Code, rec.Body.String())
	}
	pend, err := env.q.GetSubscription(t.Context(), redirectSubID(t, rec.Header().Get("Location")))
	if err != nil {
		t.Fatalf("get upsell new: %v", err)
	}
	assertPeriod(t, pend, monthsPerYear)
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

// --- gates -----------------------------------------------------------------

// TestSubscriptionRenew_Gate: renew butuh crm:renewals write. sales & csm (read
// saja) ditolak 403; manager lolos (bukan 403).
func TestSubscriptionRenew_Gate(t *testing.T) {
	env, uid := setupAccounts(t)
	planID := env.seedPlan(t, "Paket G", "PLAN-RG", "1000000")
	acc := env.seedAccount(t, "Desa G", &uid, nil, nil)
	old := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")

	for _, role := range []string{"sales", "csm"} {
		req := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(old.ID)+"/renew",
			url.Values{"new_mrr": {""}}, itoa(old.ID))
		rec := env.runAccount(uid, "owner", role, req, env.h.SubscriptionRenew)
		if rec.Code != http.StatusForbidden {
			t.Errorf("role %q harus 403 saat renew, got %d", role, rec.Code)
		}
	}
	// Manager lolos gate (tak 403).
	req := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(old.ID)+"/renew",
		url.Values{"new_mrr": {""}}, itoa(old.ID))
	rec := env.runAccount(uid, "owner", "manager", req, env.h.SubscriptionRenew)
	if rec.Code == http.StatusForbidden {
		t.Errorf("manager harus lolos gate renew, got 403")
	}
}

// TestSubscriptionRenewApprove_Gate: approve butuh crm:renewal_mgmt APPROVE. csm
// (punya renewal_mgmt write tapi bukan approve) & sales ditolak 403; manager lolos.
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
