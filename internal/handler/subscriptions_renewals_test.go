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

// subscriptions_renewals_test.go — dasbor Renewals (Modul 5, Menu 5.2, READ-ONLY)
// di sisi handler. Yang dikunci: gerbang read (F2 crm:subscriptions), pemilahan
// JENDELA (due/grace/renewed/all) atas end_date & renewal_status, F3 ownership
// (sales own-scope), dan langganan tanpa end_date TAK muncul (bukan renewal).

// seedRenewalSub menaruh satu langganan langsung lewat pool dengan end_date &
// renewal_status tertentu — untuk menguji pemilahan jendela tanpa merangkai renew.
// Tiap Active harus di account+plan BERBEDA (idx_subs_one_active).
func (e *testEnv) seedRenewalSub(
	t *testing.T, accountID, planID int64, owner *int64,
	status string, endDate time.Time, renewalStatus, renewalType string,
) db.Subscription {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntitySubscription)
	if err != nil {
		t.Fatalf("generate subscription code: %v", err)
	}
	var rs, rt *string
	if renewalStatus != "" {
		rs = &renewalStatus
	}
	if renewalType != "" {
		rt = &renewalType
	}
	s, err := e.q.CreateSubscription(t.Context(), db.CreateSubscriptionParams{
		TenantID:          e.tenantID,
		EntityCode:        &code,
		SubscriptionOwner: owner,
		AccountID:         accountID,
		PlanID:            &planID,
		Status:            status,
		EndDate:           pgtype.Date{Time: endDate, Valid: true},
		AutoRenew:         false,
		Mrr:               numFrom(t, "500000"),
		Arr:               numFrom(t, "6000000"),
		PreviousValue:     numFrom(t, "400000"),
		RenewalStatus:     rs,
		RenewalType:       rt,
		CreatedBy:         owner,
	})
	if err != nil {
		t.Fatalf("seed renewal subscription: %v", err)
	}
	return s
}

// renewalsReq membuat GET /subscriptions/renewals dengan jendela opsional.
func renewalsReq(window string) *http.Request {
	target := "/w/test/subscriptions/renewals"
	if window != "" {
		target += "?window=" + window
	}
	return accountsReq(http.MethodGet, target, nil, "")
}

// TestSubscriptionRenewals_GateRead: dasbor pakai gerbang SAMA dgn daftar (crm:
// subscriptions read). sales lolos (200); support tanpa objek → 403.
func TestSubscriptionRenewals_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)

	req := renewalsReq("")
	if rec := env.runAccount(uid, "owner", "sales", req, env.h.SubscriptionRenewals); rec.Code != http.StatusOK {
		t.Errorf("sales harus lolos gate read, got %d\n%s", rec.Code, rec.Body.String())
	}
	reqS := renewalsReq("")
	if rec := env.runAccount(uid, "owner", "support", reqS, env.h.SubscriptionRenewals); rec.Code != http.StatusForbidden {
		t.Errorf("support harus 403, got %d", rec.Code)
	}
}

// TestSubscriptionRenewals_Windows: jendela memilah baris dengan benar.
//   - due     : Active end_date dalam [today, today+30]
//   - grace   : Active end_date < today
//   - renewed : renewal_status='Renewed' (tanpa peduli tanggal)
//   - all      : semua langganan ber-end_date
func TestSubscriptionRenewals_Windows(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	due := env.seedAccount(t, "Desa Due", &uid, nil, nil)
	grace := env.seedAccount(t, "Desa Grace", &uid, nil, nil)
	far := env.seedAccount(t, "Desa Far", &uid, nil, nil)
	renewed := env.seedAccount(t, "Desa Renewed", &uid, nil, nil)
	pDue := env.seedPlan(t, "Plan Due", "PL-DUE", "1000000")
	pGrace := env.seedPlan(t, "Plan Grace", "PL-GRC", "1000000")
	pFar := env.seedPlan(t, "Plan Far", "PL-FAR", "1000000")
	pRen := env.seedPlan(t, "Plan Ren", "PL-REN", "1000000")

	env.seedRenewalSub(t, due.ID, pDue, &uid, "Active", now.AddDate(0, 0, 10), "", "Manual")
	env.seedRenewalSub(t, grace.ID, pGrace, &uid, "Active", now.AddDate(0, 0, -5), "", "Manual")
	env.seedRenewalSub(t, far.ID, pFar, &uid, "Active", now.AddDate(0, 0, 60), "", "Manual")
	env.seedRenewalSub(t, renewed.ID, pRen, &uid, "Active", now.AddDate(0, 0, 90), "Renewed", "Manual")

	cases := []struct {
		window   string
		contains []string
		absent   []string
	}{
		{"due", []string{"Desa Due"}, []string{"Desa Grace", "Desa Far", "Desa Renewed"}},
		{"grace", []string{"Desa Grace"}, []string{"Desa Due", "Desa Far"}},
		{"renewed", []string{"Desa Renewed"}, []string{"Desa Due", "Desa Grace"}},
		{"all", []string{"Desa Due", "Desa Grace", "Desa Far", "Desa Renewed"}, nil},
	}
	for _, c := range cases {
		t.Run("window="+c.window, func(t *testing.T) {
			rec := env.runAccount(uid, "owner", "manager", renewalsReq(c.window), env.h.SubscriptionRenewals)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200\n%s", rec.Code, rec.Body.String())
			}
			body := rec.Body.String()
			for _, want := range c.contains {
				if !strings.Contains(body, want) {
					t.Errorf("window %q harus memuat %q", c.window, want)
				}
			}
			for _, no := range c.absent {
				if strings.Contains(body, no) {
					t.Errorf("window %q TAK boleh memuat %q", c.window, no)
				}
			}
		})
	}
}

// TestSubscriptionRenewals_RenewedNearExcludedFromDue (BL-152): langganan yang
// SUDAH diperpanjang (renewal_status='Renewed') dgn end_date masih ≤30 hari
// TIDAK boleh muncul di tab 'due' (dulu dobel: due + renewed) dan badge-nya harus
// "Diperpanjang", bukan "Akan Jatuh Tempo". Tetap muncul di tab 'renewed'.
func TestSubscriptionRenewals_RenewedNearExcludedFromDue(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	plain := env.seedAccount(t, "Desa DuePlain", &uid, nil, nil)
	renNear := env.seedAccount(t, "Desa RenewedNear", &uid, nil, nil)
	pPlain := env.seedPlan(t, "Plan DuePlain", "PL-DP", "1000000")
	pRenNear := env.seedPlan(t, "Plan RenewedNear", "PL-RN", "1000000")
	// Keduanya end_date +10 hari (dalam jendela due 30 hari); beda hanya Renewed.
	env.seedRenewalSub(t, plain.ID, pPlain, &uid, "Active", now.AddDate(0, 0, 10), "", "Manual")
	env.seedRenewalSub(t, renNear.ID, pRenNear, &uid, "Active", now.AddDate(0, 0, 10), "Renewed", "Manual")

	// Tab 'due': hanya yang belum diperpanjang.
	due := env.runAccount(uid, "owner", "manager", renewalsReq("due"), env.h.SubscriptionRenewals)
	if due.Code != http.StatusOK {
		t.Fatalf("due status = %d, want 200", due.Code)
	}
	dueBody := due.Body.String()
	if !strings.Contains(dueBody, "Desa DuePlain") {
		t.Error("tab due harus memuat langganan belum-diperpanjang (Desa DuePlain)")
	}
	if strings.Contains(dueBody, "Desa RenewedNear") {
		t.Error("BL-152: tab due TAK boleh memuat langganan Renewed walau end ≤30 hari")
	}

	// Tab 'renewed': memuat yang diperpanjang, badge "Diperpanjang".
	ren := env.runAccount(uid, "owner", "manager", renewalsReq("renewed"), env.h.SubscriptionRenewals)
	renBody := ren.Body.String()
	if !strings.Contains(renBody, "Desa RenewedNear") {
		t.Error("tab renewed harus memuat Desa RenewedNear")
	}
	if !strings.Contains(renBody, "Diperpanjang") {
		t.Error("BL-152: baris Renewed harus berbadge Diperpanjang")
	}
}

// TestSubscriptionRenewals_DefaultWindowDue: tanpa ?window= → jendela 'due'.
func TestSubscriptionRenewals_DefaultWindowDue(t *testing.T) {
	env, uid := setupAccounts(t)
	now := time.Now()
	due := env.seedAccount(t, "Desa DueDef", &uid, nil, nil)
	grace := env.seedAccount(t, "Desa GraceDef", &uid, nil, nil)
	pDue := env.seedPlan(t, "Plan DueDef", "PL-DUED", "1000000")
	pGrace := env.seedPlan(t, "Plan GraceDef", "PL-GRCD", "1000000")
	env.seedRenewalSub(t, due.ID, pDue, &uid, "Active", now.AddDate(0, 0, 7), "", "Manual")
	env.seedRenewalSub(t, grace.ID, pGrace, &uid, "Active", now.AddDate(0, 0, -3), "", "Manual")

	rec := env.runAccount(uid, "owner", "manager", renewalsReq(""), env.h.SubscriptionRenewals)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa DueDef") {
		t.Error("default harus jendela due (memuat Desa DueDef)")
	}
	if strings.Contains(body, "Desa GraceDef") {
		t.Error("default (due) TAK boleh memuat baris grace")
	}
}

// TestSubscriptionRenewals_ScopedByOwnership: sales (own-scope) hanya melihat
// renewal miliknya; milik anggota lain tak muncul. Manager melihat keduanya.
func TestSubscriptionRenewals_ScopedByOwnership(t *testing.T) {
	env, uid := setupAccounts(t)
	other := env.seedMember(t, "other@local", "member", 0).ID
	now := time.Now()
	mine := env.seedAccount(t, "Desa Mine", &uid, nil, nil)
	theirs := env.seedAccount(t, "Desa Theirs", &other, nil, nil)
	pM := env.seedPlan(t, "Plan Mine", "PL-MINE", "1000000")
	pT := env.seedPlan(t, "Plan Theirs", "PL-THR", "1000000")
	env.seedRenewalSub(t, mine.ID, pM, &uid, "Active", now.AddDate(0, 0, 10), "", "Manual")
	env.seedRenewalSub(t, theirs.ID, pT, &other, "Active", now.AddDate(0, 0, 10), "", "Manual")

	rec := env.runAccount(uid, "owner", "sales", renewalsReq("due"), env.h.SubscriptionRenewals)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Mine") {
		t.Error("sales harus melihat renewal miliknya (Desa Mine)")
	}
	if strings.Contains(body, "Desa Theirs") {
		t.Error("sales TAK boleh melihat renewal milik anggota lain (Desa Theirs)")
	}

	recM := env.runAccount(uid, "owner", "manager", renewalsReq("due"), env.h.SubscriptionRenewals)
	bodyM := recM.Body.String()
	if !strings.Contains(bodyM, "Desa Mine") || !strings.Contains(bodyM, "Desa Theirs") {
		t.Error("manager (all-scope) harus melihat semua renewal")
	}
}

// TestSubscriptionRenewals_ExcludesNoEndDate: langganan tanpa end_date bukan
// renewal → tak muncul bahkan di jendela 'all'.
func TestSubscriptionRenewals_ExcludesNoEndDate(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa NoEnd", &uid, nil, nil)
	plan := env.seedPlan(t, "Plan NoEnd", "PL-NOE", "1000000")
	// seedSubscription (bukan seedRenewalSub) meninggalkan end_date NULL.
	env.seedSubscription(t, acc.ID, plan, &uid, "Active", "500000", "6000000")

	rec := env.runAccount(uid, "owner", "manager", renewalsReq("all"), env.h.SubscriptionRenewals)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "Desa NoEnd") {
		t.Error("langganan tanpa end_date TAK boleh muncul di dasbor renewals")
	}
}
