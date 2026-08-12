package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"go_starter/internal/codes"
	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_churn_page_test.go — dasbor Churn (Modul 5, Menu 5.2/5.4, READ-ONLY)
// di sisi handler. Yang dikunci: gerbang read (F2 crm:subscriptions), hanya langganan
// berhenti (Cancelled/Churned) yang muncul, filter tipe churn (Voluntary/Involuntary),
// dan F3 ownership (sales own-scope hanya melihat churn miliknya).

// seedChurnedSub menaruh satu langganan berhenti: buat baris lalu ChurnSubscription
// mengisi kolom churn (tipe + lost_value_mrr + tanggal). Dibuat langsung berstatus
// 'Churned' agar bebas dari idx_subs_one_active (guard hanya untuk 'Active').
func (e *testEnv) seedChurnedSub(
	t *testing.T, accountID, planID int64, owner *int64, churnType, lostMRR string,
) db.Subscription {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntitySubscription)
	if err != nil {
		t.Fatalf("generate subscription code: %v", err)
	}
	s, err := e.q.CreateSubscription(t.Context(), db.CreateSubscriptionParams{
		TenantID:          e.tenantID,
		EntityCode:        &code,
		SubscriptionOwner: owner,
		AccountID:         accountID,
		PlanID:            planID,
		Status:            "Churned",
		AutoRenew:         false,
		Mrr:               numFrom(t, lostMRR),
		Arr:               numFrom(t, "6000000"),
		CreatedBy:         owner,
	})
	if err != nil {
		t.Fatalf("seed churned subscription: %v", err)
	}
	ct, reason := churnType, "Budget"
	if err := e.q.ChurnSubscription(t.Context(), db.ChurnSubscriptionParams{
		Status:           "Churned",
		CancellationDate: pgtype.Date{Time: time.Now(), Valid: true},
		ChurnReason:      &reason,
		ChurnType:        &ct,
		LostValueMrr:     numFrom(t, lostMRR),
		UpdatedBy:        owner,
		ID:               s.ID,
	}); err != nil {
		t.Fatalf("churn subscription: %v", err)
	}
	return s
}

// churnListReq membuat GET /subscriptions/churn dengan filter tipe opsional.
func churnListReq(churnType string) *http.Request {
	target := "/w/test/subscriptions/churn"
	if churnType != "" {
		target += "?type=" + churnType
	}
	return accountsReq(http.MethodGet, target, nil, "")
}

// TestSubscriptionChurnList_GateRead: dasbor pakai gerbang SAMA dgn daftar (crm:
// subscriptions read). sales lolos (200); support tanpa objek → 403.
func TestSubscriptionChurnList_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)

	if rec := env.runAccount(uid, "owner", "sales", churnListReq(""), env.h.SubscriptionChurnList); rec.Code != http.StatusOK {
		t.Errorf("sales harus lolos gate read, got %d\n%s", rec.Code, rec.Body.String())
	}
	if rec := env.runAccount(uid, "owner", "support", churnListReq(""), env.h.SubscriptionChurnList); rec.Code != http.StatusForbidden {
		t.Errorf("support harus 403, got %d", rec.Code)
	}
}

// TestSubscriptionChurnList_OnlyChurned: hanya langganan berhenti muncul; langganan
// aktif (bukan churn) TIDAK muncul di dasbor churn.
func TestSubscriptionChurnList_OnlyChurned(t *testing.T) {
	env, uid := setupAccounts(t)
	gone := env.seedAccount(t, "Desa Gone", &uid, nil, nil)
	live := env.seedAccount(t, "Desa Live", &uid, nil, nil)
	pGone := env.seedPlan(t, "Plan Gone", "PL-GONE", "1000000")
	pLive := env.seedPlan(t, "Plan Live", "PL-LIVE", "1000000")
	env.seedChurnedSub(t, gone.ID, pGone, &uid, "Voluntary", "500000")
	env.seedSubscription(t, live.ID, pLive, &uid, "Active", "700000", "8400000")

	rec := env.runAccount(uid, "owner", "manager", churnListReq(""), env.h.SubscriptionChurnList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Gone") {
		t.Error("dasbor churn harus memuat langganan berhenti (Desa Gone)")
	}
	if strings.Contains(body, "Desa Live") {
		t.Error("dasbor churn TAK boleh memuat langganan aktif (Desa Live)")
	}
}

// TestSubscriptionChurnList_TypeFilter: ?type= memilah tipe churn; kosong = semua.
func TestSubscriptionChurnList_TypeFilter(t *testing.T) {
	env, uid := setupAccounts(t)
	vol := env.seedAccount(t, "Desa Vol", &uid, nil, nil)
	invol := env.seedAccount(t, "Desa Invol", &uid, nil, nil)
	pVol := env.seedPlan(t, "Plan Vol", "PL-VOL", "1000000")
	pInvol := env.seedPlan(t, "Plan Invol", "PL-INV", "1000000")
	env.seedChurnedSub(t, vol.ID, pVol, &uid, "Voluntary", "500000")
	env.seedChurnedSub(t, invol.ID, pInvol, &uid, "Involuntary", "600000")

	cases := []struct {
		typ      string
		contains []string
		absent   []string
	}{
		{"Voluntary", []string{"Desa Vol"}, []string{"Desa Invol"}},
		{"Involuntary", []string{"Desa Invol"}, []string{"Desa Vol"}},
		{"", []string{"Desa Vol", "Desa Invol"}, nil},
	}
	for _, c := range cases {
		name := c.typ
		if name == "" {
			name = "all"
		}
		t.Run("type="+name, func(t *testing.T) {
			rec := env.runAccount(uid, "owner", "manager", churnListReq(c.typ), env.h.SubscriptionChurnList)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			for _, want := range c.contains {
				if !strings.Contains(body, want) {
					t.Errorf("type %q harus memuat %q", c.typ, want)
				}
			}
			for _, no := range c.absent {
				if strings.Contains(body, no) {
					t.Errorf("type %q TAK boleh memuat %q", c.typ, no)
				}
			}
		})
	}
}

// TestSubscriptionChurnList_ScopedByOwnership: sales (own-scope) hanya melihat churn
// miliknya; milik anggota lain tak muncul. Manager melihat keduanya.
func TestSubscriptionChurnList_ScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other@local", "member", 0).ID
	mine := env.seedAccount(t, "Desa Mine", &uid, nil, nil)
	theirs := env.seedAccount(t, "Desa Theirs", &other, nil, nil)
	pM := env.seedPlan(t, "Plan Mine", "PL-MINE", "1000000")
	pT := env.seedPlan(t, "Plan Theirs", "PL-THR", "1000000")
	env.seedChurnedSub(t, mine.ID, pM, &uid, "Voluntary", "500000")
	env.seedChurnedSub(t, theirs.ID, pT, &other, "Voluntary", "500000")

	rec := env.runAccount(uid, "owner", "sales", churnListReq(""), env.h.SubscriptionChurnList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Mine") {
		t.Error("sales harus melihat churn miliknya (Desa Mine)")
	}
	if strings.Contains(body, "Desa Theirs") {
		t.Error("sales TAK boleh melihat churn milik anggota lain (Desa Theirs)")
	}

	recM := env.runAccount(uid, "owner", "manager", churnListReq(""), env.h.SubscriptionChurnList)
	bodyM := recM.Body.String()
	if !strings.Contains(bodyM, "Desa Mine") || !strings.Contains(bodyM, "Desa Theirs") {
		t.Error("manager (all-scope) harus melihat semua churn")
	}
}
