package handler

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"testing"

	"go_starter/internal/db"
)

// subscriptions_renew_test.go — mutasi renew (M5-3c) di sisi handler: perpanjang
// (straight, upsell & downgrade — BL-172) + gerbang renew. Invarian yang dikunci:
// renewal = INSERT baris baru; straight → baris lama Expired lebih dulu; upsell/
// downgrade → baris lama TETAP Active sampai Manager menyetujui
// (idx_subs_one_active); harga BERBEDA (naik ATAU turun) sama-sama butuh
// persetujuan — hanya harga SAMA PERSIS yang auto-approve (straight).
//
// approve/reject dipisah ke subscriptions_renew_approve_test.go (file health).
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
	// Regresi BL-172: MRR sama persis (kosong = ikut lama) tetap auto-approve,
	// action audit generik (tak berubah oleh penambahan arah upsell/downgrade).
	env.assertAudited(t, "subscription.renew")
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
	if fresh.RenewalType == nil || *fresh.RenewalType != "Upsell" {
		t.Errorf("renewal_type = %v, want Upsell", fresh.RenewalType)
	}
	// Regresi BL-172: arah naik tetap pakai action audit lama (Opsi A — dua string
	// berbeda per arah, bukan satu generic + metadata; lihat auditAction()).
	env.assertAudited(t, "subscription.renew.upsell")
}

// TestSubscriptionRenew_Downgrade: MRR baru < lama (BL-172) — SEBELUMNYA lolos
// diam-diam lewat renewStraight (auto-approve); SEKARANG harus lewat jalur sama
// dgn upsell: baris baru 'PendingApproval' (renewal_type "Downgrade"), baris lama
// TETAP Active, Manager dinotifikasi, dan action audit
// "subscription.renew.downgrade" (Opsi A: string terpisah, bukan flag di metadata,
// agar tetap bisa difilter via ActionPrefix di /dev/logs).
func TestSubscriptionRenew_Downgrade(t *testing.T) {
	env, uid := setupAccounts(t)
	mgr := env.seedMember(t, "mgrd@local", "member", 0)
	if err := env.q.UpdateMemberBusinessRole(t.Context(), db.UpdateMemberBusinessRoleParams{
		UserID: mgr.ID, TenantID: env.tenantID, BusinessRole: ptr("manager"),
	}); err != nil {
		t.Fatalf("set manager business_role: %v", err)
	}
	planID := env.seedPlan(t, "Paket DG", "PLAN-DG", "1000000")
	acc := env.seedAccount(t, "Desa DG", &uid, nil, nil)
	old := env.seedSubscription(t, acc.ID, planID, &uid, "Active", "500000", "6000000")

	form := url.Values{"new_mrr": {"300000"}} // di BAWAH MRR lama (500000)
	req := accountsReq(http.MethodPost, "/w/test/subscriptions/"+itoa(old.ID)+"/renew", form, itoa(old.ID))
	rec := env.runAccount(uid, "owner", "manager", req, env.h.SubscriptionRenew)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303\n%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !contains(loc, "ok=renew_pending") {
		t.Errorf("Location %q harus memuat ok=renew_pending (harga turun TETAP butuh approval)", loc)
	}
	// Baris lama TETAP Active — downgrade belum disetujui (sama seperti upsell).
	got, err := env.q.GetSubscription(t.Context(), old.ID)
	if err != nil {
		t.Fatalf("get old: %v", err)
	}
	if got.Status != "Active" {
		t.Errorf("baris lama status = %q, want Active (downgrade belum disetujui)", got.Status)
	}
	// Baris baru PendingApproval + renewal_type Downgrade.
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
	if !numEq(fresh.Mrr, "300000") {
		t.Errorf("MRR baru harus 300000")
	}
	if fresh.RenewalType == nil || *fresh.RenewalType != "Downgrade" {
		t.Errorf("renewal_type = %v, want Downgrade", fresh.RenewalType)
	}
	// Manager menerima notifikasi (kind digeneralisasi "renewal.pending").
	n, err := env.q.CountUnreadNotifications(t.Context(), mgr.ID)
	if err != nil {
		t.Fatalf("count notif: %v", err)
	}
	if n < 1 {
		t.Errorf("Manager harus menerima notifikasi renewal downgrade (unread=%d)", n)
	}
	env.assertAudited(t, "subscription.renew.downgrade")
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

// --- gate --------------------------------------------------------------

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
