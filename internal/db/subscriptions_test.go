package db

import (
	"context"
	"testing"
	"time"

	"go_starter/internal/codes"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// subscriptions_test.go — bukti query pipeline Subscription (M5 slice 2). Sifat
// yang, bila rusak, tak terlihat sampai angka langganan / riwayat renewal salah di
// produksi:
//
//   (1) create-from-deal: entity_code (SUB-0001) dialokasikan DI DALAM tx;
//       source_deal_id terikat; SetDealCreatedSubscription menutup tautan dua-arah.
//   (2) INVARIAN one-active-per-chain (idx_subs_one_active): tepat 1 Active per
//       (account,plan) — INSERT Active kedua ditolak; account/plan lain lolos.
//   (3) Renewal = baris BARU: Expired baris lama LEBIH DULU lalu CreateSubscription
//       Active dgn previous_subscription_id — baris lama TAK ditimpa, rantai tertaut.
//   (4) ListRenewalChain menelusuri self-FK mundur → seluruh periode kronologis.
//   (5) Churn = satu aksi: status + kolom churn (5.4) tertulis sekaligus.
//   (6) ListSubscriptions = keyset + F3 fail-closed (subscription_owner).

func i64ptr(v int64) *int64 { return &v }

func pgDate(y int, m time.Month, d int) pgtype.Date {
	return pgtype.Date{Time: time.Date(y, m, d, 0, 0, 0, 0, time.UTC), Valid: true}
}

// seedSubscription membuat satu langganan lewat CreateSubscription asli (entity_code
// dari GenerateEntityCode) di dalam tx ber-tenant. Default status 'Active'.
func seedSubscription(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, accountID, planID int64, mut func(*CreateSubscriptionParams)) Subscription {
	t.Helper()
	var out Subscription
	if err := WithTenant(ctx, pool, tenantID, func(q *Queries) error {
		code, err := q.GenerateEntityCode(ctx, tenantID, codes.EntitySubscription)
		if err != nil {
			return err
		}
		p := CreateSubscriptionParams{
			TenantID:   tenantID,
			EntityCode: &code,
			AccountID:  accountID,
			PlanID:     planID,
			Status:     "Active",
		}
		if mut != nil {
			mut(&p)
		}
		out, err = q.CreateSubscription(ctx, p)
		return err
	}); err != nil {
		t.Fatalf("seed subscription: %v", err)
	}
	return out
}

func TestCreateSubscriptionFromDeal(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Sukamaju", nil)
	plan := seedPlan(t, ctx, pool, ten.ID, "PKG-INTI", "1000.00")
	deal := seedDeal(t, ctx, pool, ten.ID, acc.ID, "Langganan", nil)

	// Create-from-deal + tautan balik deal→sub dalam SATU tx (atomik).
	var sub Subscription
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		code, err := q.GenerateEntityCode(ctx, ten.ID, codes.EntitySubscription)
		if err != nil {
			return err
		}
		sub, err = q.CreateSubscription(ctx, CreateSubscriptionParams{
			TenantID: ten.ID, EntityCode: &code,
			AccountID: acc.ID, PlanID: plan, SourceDealID: &deal.ID,
			Status: "Active", Arr: numeric(t, "12000.00"),
		})
		if err != nil {
			return err
		}
		return q.SetDealCreatedSubscription(ctx, SetDealCreatedSubscriptionParams{
			CreatedSubscriptionID: i64ptr(sub.ID), ID: deal.ID,
		})
	}); err != nil {
		t.Fatalf("create-from-deal: %v", err)
	}

	if sub.EntityCode == nil || *sub.EntityCode != "SUB-0001" {
		t.Errorf("entity_code pertama harus SUB-0001, got %v", sub.EntityCode)
	}
	if sub.AccountID != acc.ID || sub.PlanID != plan {
		t.Errorf("account/plan harus terikat, got acc=%d plan=%d", sub.AccountID, sub.PlanID)
	}
	if sub.SourceDealID == nil || *sub.SourceDealID != deal.ID {
		t.Errorf("source_deal_id harus terikat, got %v", sub.SourceDealID)
	}

	// Tautan balik: deal.created_subscription_id menunjuk langganan yang lahir.
	var gotDeal Deal
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		gotDeal, e = q.GetDeal(ctx, deal.ID)
		return e
	}); err != nil {
		t.Fatalf("get deal: %v", err)
	}
	if gotDeal.CreatedSubscriptionID == nil || *gotDeal.CreatedSubscriptionID != sub.ID {
		t.Errorf("deal.created_subscription_id harus menunjuk sub %d, got %v", sub.ID, gotDeal.CreatedSubscriptionID)
	}
}

func TestOneActivePerChain_Guard(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)
	acc2 := seedAccount(t, ctx, pool, ten.ID, "Acc2", nil)
	plan := seedPlan(t, ctx, pool, ten.ID, "PKG-A", "100.00")
	plan2 := seedPlan(t, ctx, pool, ten.ID, "PKG-B", "200.00")

	seedSubscription(t, ctx, pool, ten.ID, acc.ID, plan, nil) // Active (acc,plan)

	// Active KEDUA untuk (acc,plan) yang sama → ditolak idx_subs_one_active.
	if e := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, err := q.CreateSubscription(ctx, CreateSubscriptionParams{
			TenantID: ten.ID, AccountID: acc.ID, PlanID: plan, Status: "Active",
		})
		return err
	}); e == nil {
		t.Errorf("Active kedua untuk (acc,plan) sama harus ditolak idx_subs_one_active")
	}

	// Plan lain (acc,plan2) → lolos: guard per (account,plan).
	if e := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, err := q.CreateSubscription(ctx, CreateSubscriptionParams{
			TenantID: ten.ID, AccountID: acc.ID, PlanID: plan2, Status: "Active",
		})
		return err
	}); e != nil {
		t.Errorf("Active untuk plan berbeda harus lolos, got %v", e)
	}

	// Account lain (acc2,plan) → lolos.
	if e := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, err := q.CreateSubscription(ctx, CreateSubscriptionParams{
			TenantID: ten.ID, AccountID: acc2.ID, PlanID: plan, Status: "Active",
		})
		return err
	}); e != nil {
		t.Errorf("Active untuk account berbeda harus lolos, got %v", e)
	}
}

// TestRenewSubscription_NewRowOldExpired — acceptance M5: renew TAK menimpa baris
// lama; Expired dulu lalu INSERT Active baru bertaut previous_subscription_id.
func TestRenewSubscription_NewRowOldExpired(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)
	plan := seedPlan(t, ctx, pool, ten.ID, "PKG", "100.00")

	old := seedSubscription(t, ctx, pool, ten.ID, acc.ID, plan, func(p *CreateSubscriptionParams) {
		p.Arr = numeric(t, "1000.00")
	})

	// Renewal dalam SATU tx: Expired lama → Active baru (previous_* snapshot).
	var fresh Subscription
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		if e := q.UpdateSubscriptionStatus(ctx, UpdateSubscriptionStatusParams{
			Status: "Expired", ID: old.ID,
		}); e != nil {
			return e
		}
		code, e := q.GenerateEntityCode(ctx, ten.ID, codes.EntitySubscription)
		if e != nil {
			return e
		}
		fresh, e = q.CreateSubscription(ctx, CreateSubscriptionParams{
			TenantID: ten.ID, EntityCode: &code,
			AccountID: acc.ID, PlanID: plan, Status: "Active",
			PreviousSubscriptionID: &old.ID, PreviousValue: numeric(t, "1000.00"),
			RenewalType: strPtr("Auto"), Arr: numeric(t, "1200.00"),
		})
		return e
	}); err != nil {
		t.Fatalf("renewal: %v", err)
	}

	if fresh.ID == old.ID {
		t.Fatalf("renewal harus baris BARU, dapat id sama %d", fresh.ID)
	}
	if fresh.PreviousSubscriptionID == nil || *fresh.PreviousSubscriptionID != old.ID {
		t.Errorf("previous_subscription_id harus menunjuk periode lama %d, got %v", old.ID, fresh.PreviousSubscriptionID)
	}
	if fresh.Status != "Active" {
		t.Errorf("baris baru harus Active, got %q", fresh.Status)
	}

	// Baris lama TETAP ADA, status Expired (tak ditimpa).
	var gotOld Subscription
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		gotOld, e = q.GetSubscription(ctx, old.ID)
		return e
	}); err != nil {
		t.Fatalf("get old: %v", err)
	}
	if gotOld.Status != "Expired" {
		t.Errorf("periode lama harus Expired (tak ditimpa), got %q", gotOld.Status)
	}

	// Rantai renewal: telusuri mundur dari baris baru → [lama, baru] kronologis.
	var chain []ListRenewalChainRow
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		chain, e = q.ListRenewalChain(ctx, fresh.ID)
		return e
	}); err != nil {
		t.Fatalf("chain: %v", err)
	}
	if len(chain) != 2 || chain[0].ID != old.ID || chain[1].ID != fresh.ID {
		t.Fatalf("rantai harus [lama,baru], got %d baris", len(chain))
	}
}

func TestChurnSubscription(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)
	plan := seedPlan(t, ctx, pool, ten.ID, "PKG", "100.00")
	sub := seedSubscription(t, ctx, pool, ten.ID, acc.ID, plan, nil)

	var got Subscription
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		if e := q.ChurnSubscription(ctx, ChurnSubscriptionParams{
			Status:           "Churned",
			CancellationDate: pgDate(2026, time.August, 1),
			ChurnReason:      strPtr("Budget"),
			ChurnType:        strPtr("Voluntary"),
			ChurnNotes:       strPtr("APBDes dipangkas"),
			LostValueMrr:     numeric(t, "100.00"),
			WinBackEligible:  boolPtr(true),
			ID:               sub.ID,
		}); e != nil {
			return e
		}
		var e error
		got, e = q.GetSubscription(ctx, sub.ID)
		return e
	}); err != nil {
		t.Fatalf("churn: %v", err)
	}
	if got.Status != "Churned" {
		t.Errorf("status harus Churned, got %q", got.Status)
	}
	if got.ChurnReason == nil || *got.ChurnReason != "Budget" {
		t.Errorf("churn_reason harus Budget, got %v", got.ChurnReason)
	}
	if numStr(t, got.LostValueMrr) != "100.00" {
		t.Errorf("lost_value_mrr harus 100.00, got %q", numStr(t, got.LostValueMrr))
	}

	// churn_reason di luar domain ditolak subs_churn_reason_chk.
	if e := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.ChurnSubscription(ctx, ChurnSubscriptionParams{
			Status: "Churned", ChurnReason: strPtr("Alien Invasion"), ID: sub.ID,
		})
	}); e == nil {
		t.Errorf("churn_reason liar harus ditolak subs_churn_reason_chk")
	}
}

func TestListSubscriptions_OwnershipFailClosed(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)
	plan := seedPlan(t, ctx, pool, ten.ID, "PKG-A", "100.00")
	plan2 := seedPlan(t, ctx, pool, ten.ID, "PKG-B", "200.00")
	sales, _ := q.CreateUser(ctx, CreateUserParams{Email: "s@x", PassHash: strPtr("x")})
	other, _ := q.CreateUser(ctx, CreateUserParams{Email: "o@x", PassHash: strPtr("x")})

	seedSubscription(t, ctx, pool, ten.ID, acc.ID, plan, func(p *CreateSubscriptionParams) { p.SubscriptionOwner = &sales.ID })
	seedSubscription(t, ctx, pool, ten.ID, acc.ID, plan2, func(p *CreateSubscriptionParams) { p.SubscriptionOwner = &other.ID })

	listWith := func(scopeAll, isOwn bool, uid int64) []ListSubscriptionsRow {
		var rows []ListSubscriptionsRow
		c0, id0 := firstCursor()
		if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
			var e error
			rows, e = q.ListSubscriptions(ctx, ListSubscriptionsParams{
				CursorCreatedAt: c0, CursorID: id0,
				ScopeAll: scopeAll, IsOwn: isOwn, Uid: &uid, PageSize: 50,
			})
			return e
		}); err != nil {
			t.Fatalf("list: %v", err)
		}
		return rows
	}

	if got := listWith(true, false, sales.ID); len(got) != 2 {
		t.Errorf("scope_all harus 2 langganan, got %d", len(got))
	}
	if got := listWith(false, true, sales.ID); len(got) != 1 || got[0].SubscriptionOwner == nil || *got[0].SubscriptionOwner != sales.ID {
		t.Errorf("own/sales harus 1 milik sales, got %d", len(got))
	}
	// Fail-closed: tanpa flag → NOL baris (kriteria F3).
	if got := listWith(false, false, sales.ID); len(got) != 0 {
		t.Errorf("tanpa flag harus 0 baris (fail-closed), got %d", len(got))
	}
	// JOIN membawa nama untuk kolom (hindari N+1).
	if got := listWith(true, false, sales.ID); got[0].VillageName == "" || got[0].PlanName == "" {
		t.Errorf("JOIN harus membawa village_name & plan_name")
	}
}

func boolPtr(b bool) *bool { return &b }
