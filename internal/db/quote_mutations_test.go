package db

import (
	"context"
	"testing"
)

func TestUpdateQuoteTotals(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)
	deal := seedDeal(t, ctx, pool, ten.ID, acc.ID, "Deal", nil)
	quote := seedQuote(t, ctx, pool, ten.ID, acc.ID, &deal.ID, "Q")

	// BL-14: UpdateQuoteTotals kini juga menyimpan tax_mode (+ tax_rate saat percent).
	// Mode 'amount' → rate NULL, tax_amount = nilai flat.
	var got Quote
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		if e := q.UpdateQuoteTotals(ctx, UpdateQuoteTotalsParams{
			TaxMode:    "amount",
			GrandTotal: numeric(t, "1500.00"),
			TaxAmount:  numeric(t, "150.00"),
			ID:         quote.ID,
		}); e != nil {
			return e
		}
		var e error
		got, e = q.GetQuote(ctx, quote.ID)
		return e
	}); err != nil {
		t.Fatalf("totals: %v", err)
	}
	if got.TaxMode != "amount" || numStr(t, got.GrandTotal) != "1500.00" || numStr(t, got.TaxAmount) != "150.00" {
		t.Errorf("total harus tersimpan (mode=amount,grand=1500.00,tax=150.00), got mode=%q grand=%q tax=%q",
			got.TaxMode, numStr(t, got.GrandTotal), numStr(t, got.TaxAmount))
	}

	// Mode 'percent' → tax_rate tersimpan (snapshot tax_amount tetap dihitung app).
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		if e := q.UpdateQuoteTotals(ctx, UpdateQuoteTotalsParams{
			TaxMode:    "percent",
			TaxRate:    numeric(t, "11.00"),
			GrandTotal: numeric(t, "1110.00"),
			TaxAmount:  numeric(t, "110.00"),
			ID:         quote.ID,
		}); e != nil {
			return e
		}
		var e error
		got, e = q.GetQuote(ctx, quote.ID)
		return e
	}); err != nil {
		t.Fatalf("totals percent: %v", err)
	}
	if got.TaxMode != "percent" || numStr(t, got.TaxRate) != "11.00" {
		t.Errorf("mode/rate percent harus tersimpan, got mode=%q rate=%q",
			got.TaxMode, numStr(t, got.TaxRate))
	}
}

func TestUpdateQuoteStatus_DanCheckMenolakLiar(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)
	deal := seedDeal(t, ctx, pool, ten.ID, acc.ID, "Deal", nil)
	quote := seedQuote(t, ctx, pool, ten.ID, acc.ID, &deal.ID, "Q")

	// Transisi legal Draft→Sent tersimpan.
	var got Quote
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		if e := q.UpdateQuoteStatus(ctx, UpdateQuoteStatusParams{QuoteStatus: "Sent", ID: quote.ID}); e != nil {
			return e
		}
		var e error
		got, e = q.GetQuote(ctx, quote.ID)
		return e
	}); err != nil {
		t.Fatalf("status Sent: %v", err)
	}
	if got.QuoteStatus != "Sent" {
		t.Errorf("status harus Sent, got %q", got.QuoteStatus)
	}

	// Status liar ditolak quotes_status_chk.
	if e := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.UpdateQuoteStatus(ctx, UpdateQuoteStatusParams{QuoteStatus: "Warp", ID: quote.ID})
	}); e == nil {
		t.Errorf("status liar harus ditolak quotes_status_chk")
	}
}

func TestAddQuoteItem_CheckConstraints(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)
	deal := seedDeal(t, ctx, pool, ten.ID, acc.ID, "Deal", nil)
	quote := seedQuote(t, ctx, pool, ten.ID, acc.ID, &deal.ID, "Q")

	// quantity <= 0 ditolak quote_items_quantity_chk.
	if e := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, err := q.AddQuoteItem(ctx, AddQuoteItemParams{
			QuoteID: quote.ID, TenantID: ten.ID, Quantity: 0,
			UnitPrice: numeric(t, "100.00"), Subtotal: numeric(t, "0.00"),
		})
		return err
	}); e == nil {
		t.Errorf("quantity 0 harus ditolak quote_items_quantity_chk")
	}

	// discount_pct > 100 ditolak quote_items_discount_chk.
	if e := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, err := q.AddQuoteItem(ctx, AddQuoteItemParams{
			QuoteID: quote.ID, TenantID: ten.ID, Quantity: 1,
			UnitPrice: numeric(t, "100.00"), DiscountPct: numeric(t, "150.00"),
			Subtotal: numeric(t, "0.00"),
		})
		return err
	}); e == nil {
		t.Errorf("discount 150 harus ditolak quote_items_discount_chk")
	}
}

func TestSoftDeleteQuote(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)
	deal := seedDeal(t, ctx, pool, ten.ID, acc.ID, "Deal", nil)
	quote := seedQuote(t, ctx, pool, ten.ID, acc.ID, &deal.ID, "Buang")

	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.SoftDeleteQuote(ctx, SoftDeleteQuoteParams{ID: quote.ID})
	}); err != nil {
		t.Fatalf("soft-delete: %v", err)
	}

	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, e := q.GetQuote(ctx, quote.ID)
		return e
	}); err == nil {
		t.Errorf("get atas quote ter-soft-delete harus gagal")
	}

	if e := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.SoftDeleteQuote(ctx, SoftDeleteQuoteParams{ID: quote.ID})
	}); e != nil {
		t.Errorf("soft-delete kedua harus idempotent, got %v", e)
	}
}

func TestDeleteQuoteItem(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)
	deal := seedDeal(t, ctx, pool, ten.ID, acc.ID, "Deal", nil)
	quote := seedQuote(t, ctx, pool, ten.ID, acc.ID, &deal.ID, "Q")

	var item QuoteItem
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		item, e = q.AddQuoteItem(ctx, AddQuoteItemParams{
			QuoteID: quote.ID, TenantID: ten.ID, Quantity: 1,
			UnitPrice: numeric(t, "100.00"), Subtotal: numeric(t, "100.00"), LineNo: i16ptr(1),
		})
		return e
	}); err != nil {
		t.Fatalf("add item: %v", err)
	}

	var remaining []QuoteItem
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		if e := q.DeleteQuoteItem(ctx, item.ID); e != nil {
			return e
		}
		var e error
		remaining, e = q.ListQuoteItems(ctx, quote.ID)
		return e
	}); err != nil {
		t.Fatalf("delete item: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("item harus terhapus (hard-delete), sisa %d", len(remaining))
	}
}
