package db

import (
	"context"
	"testing"
	"time"
)

// deals_rollup_test.go — bukti GetLatestDealForAccount & CountDealsByAccount
// (chip "Terkait" di detail Account, M2 rollup). Pisah dari deals_test.go yang
// sudah dekat batas 400 baris.

func TestGetLatestDealForAccount(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)

	seedDeal(t, ctx, pool, ten.ID, acc.ID, "Lama", nil)
	time.Sleep(10 * time.Millisecond)
	latest := seedDeal(t, ctx, pool, ten.ID, acc.ID, "Terbaru", nil)

	var got Deal
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		got, e = q.GetLatestDealForAccount(ctx, acc.ID)
		return e
	}); err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if got.ID != latest.ID || got.DealName != "Terbaru" {
		t.Errorf("harus kembalikan deal TERBARU (Terbaru), got %q", got.DealName)
	}

	// Desa lain belum punya deal → pgx.ErrNoRows (empty-state).
	empty := seedAccount(t, ctx, pool, ten.ID, "TanpaDeal", nil)
	err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, e := q.GetLatestDealForAccount(ctx, empty.ID)
		return e
	})
	if err == nil {
		t.Errorf("desa tanpa deal harus mengembalikan error (pgx.ErrNoRows)")
	}
}

func TestCountDealsByAccount(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)

	seedDeal(t, ctx, pool, ten.ID, acc.ID, "Satu", nil)
	dua := seedDeal(t, ctx, pool, ten.ID, acc.ID, "Dua", nil)
	seedDeal(t, ctx, pool, ten.ID, acc.ID, "Tiga", nil)

	// Soft-delete satu → tak boleh ikut terhitung.
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.SoftDeleteDeal(ctx, SoftDeleteDealParams{ID: dua.ID})
	}); err != nil {
		t.Fatalf("soft-delete: %v", err)
	}

	var count int64
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		count, e = q.CountDealsByAccount(ctx, acc.ID)
		return e
	}); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Errorf("deal hidup harus 2 (2 live + 1 ter-soft-delete), got %d", count)
	}
}
