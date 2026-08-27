package db

import (
	"context"
	"testing"
	"time"
)

// subscriptions_rollup_test.go — bukti GetLatestSubscriptionForAccount (kartu
// "Ringkasan Langganan" di detail Account, M2 rollup). Sifat kritis: harus
// mengambil baris TERBARU (bukan pertama) dan membawa plan_name lewat JOIN,
// serta pgx.ErrNoRows saat desa belum pernah berlangganan (empty-state, bukan
// error) — pisah dari subscriptions_test.go yang sudah dekat batas 400 baris.

func TestGetLatestSubscriptionForAccount(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Sukamaju", nil)
	plan := seedPlan(t, ctx, pool, ten.ID, "PKG-INTI", "1000.00")

	// Baris pertama (akan jadi lama).
	seedSubscription(t, ctx, pool, ten.ID, acc.ID, plan, func(p *CreateSubscriptionParams) {
		p.Status = "Expired"
		p.Arr = numeric(t, "1000.00")
	})
	time.Sleep(10 * time.Millisecond) // beda created_at agar urutan pasti

	latest := seedSubscription(t, ctx, pool, ten.ID, acc.ID, plan, func(p *CreateSubscriptionParams) {
		p.Arr = numeric(t, "2000.00")
	})

	var got GetLatestSubscriptionForAccountRow
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		got, e = q.GetLatestSubscriptionForAccount(ctx, acc.ID)
		return e
	}); err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if got.ID != latest.ID {
		t.Errorf("harus kembalikan baris TERBARU (id=%d), got id=%d", latest.ID, got.ID)
	}
	if got.PlanName != "Paket Inti" {
		t.Errorf("plan_name harus ikut lewat JOIN, got %q", got.PlanName)
	}
	if numStr(t, got.Arr) != "2000.00" {
		t.Errorf("arr harus dari baris terbaru (2000.00), got %q", numStr(t, got.Arr))
	}

	// Desa lain belum pernah berlangganan → pgx.ErrNoRows (empty-state).
	empty := seedAccount(t, ctx, pool, ten.ID, "BelumLangganan", nil)
	err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, e := q.GetLatestSubscriptionForAccount(ctx, empty.ID)
		return e
	})
	if err == nil {
		t.Errorf("desa tanpa langganan harus mengembalikan error (pgx.ErrNoRows)")
	}
}
