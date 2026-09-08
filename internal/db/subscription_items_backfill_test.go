package db

import (
	"context"
	"testing"
)

// subscription_items_backfill_test.go — BL-88 PR2a: pernyataan backfill migrasi 00042
// (1 subscription_items per langganan LAMA tanpa item) harus IDEMPOTEN. Migrasi sudah
// termigrasi saat test jalan, jadi statement-nya diuji LANGSUNG atas langganan yang
// diseed (belum punya item): jalankan sekali → 1 item; ulang → tetap 1 (WHERE NOT
// EXISTS). Ini yang menjaga goose re-run / clone ulang tak menggandakan baris.

const backfillSubscriptionItems = `
INSERT INTO subscription_items (subscription_id, tenant_id, plan_id, quantity,
    unit_price, subtotal, mrr, arr, line_no)
SELECT s.id, s.tenant_id, s.plan_id, COALESCE(s.quantity_seats, 1),
    COALESCE(s.mrr, 0), COALESCE(s.mrr, 0) * COALESCE(s.contract_term_months, 1),
    s.mrr, s.arr, 1
FROM subscriptions s
WHERE s.plan_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM subscription_items si WHERE si.subscription_id = s.id)`

func TestBackfillSubscriptionItems_Idempotent(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, err := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t-bf"})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	acc := seedAccount(t, ctx, pool, ten.ID, "Desa Backfill", nil)
	plan := seedPlan(t, ctx, pool, ten.ID, "PKG-BF", "1000.00")
	sub := seedSubscription(t, ctx, pool, ten.ID, acc.ID, plan, func(p *CreateSubscriptionParams) {
		p.Mrr = numeric(t, "1000.00")
	})

	countItems := func() int {
		var n int
		if err := pool.QueryRow(ctx,
			`SELECT count(*) FROM subscription_items WHERE subscription_id=$1`, sub.ID).Scan(&n); err != nil {
			t.Fatalf("count items: %v", err)
		}
		return n
	}

	// Langganan seed belum punya item (lahir bukan lewat jalur Won-with-items).
	if n := countItems(); n != 0 {
		t.Fatalf("prakondisi: langganan seed harus 0 item, got %d", n)
	}

	if _, err := pool.Exec(ctx, backfillSubscriptionItems); err != nil {
		t.Fatalf("backfill run 1: %v", err)
	}
	if n := countItems(); n != 1 {
		t.Fatalf("setelah backfill pertama = %d item, want 1", n)
	}

	// Ulang: WHERE NOT EXISTS harus melewati langganan yang sudah punya item.
	if _, err := pool.Exec(ctx, backfillSubscriptionItems); err != nil {
		t.Fatalf("backfill run 2: %v", err)
	}
	if n := countItems(); n != 1 {
		t.Fatalf("backfill kedua menggandakan: %d item, want 1 (idempoten)", n)
	}
}
