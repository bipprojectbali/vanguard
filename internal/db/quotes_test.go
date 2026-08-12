package db

import (
	"context"
	"testing"

	"go_starter/internal/codes"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// quotes_test.go — bukti query pipeline Quote (M4 slice 2). Sifat yang, bila rusak,
// tak terlihat sampai angka penawaran / isolasi salah di produksi:
//
//   (1) Roundtrip create→get: entity_code (QUO-001) dialokasikan DI DALAM tx;
//       account_id NOT NULL terikat; status default 'Draft'.
//   (2) ListQuotesForDeal = keyset + filter deal_id satu query. Keyset menjangkau
//       halaman kedua; filter deal_id mengisolasi quote antar-deal.
//   (3) quote_items: ListQuoteItems urut (line_no lalu id); QuoteItemsSubtotal =
//       agregat satu round-trip.
//   (4) SNAPSHOT HARGA (acceptance M4): unit_price item DIBEKUKAN saat dibuat —
//       ubah plans.base_price sesudahnya TAK menyentuh unit_price yang tersimpan.
//   (5) UpdateQuoteStatus tunduk quotes_status_chk (status liar ditolak); CHECK
//       quote_items menolak quantity<=0 & discount di luar 0..100.
//   (6) SoftDeleteQuote menyembunyikan & idempotent; DeleteQuoteItem hard-delete.

// numeric membangun pgtype.Numeric dari literal desimal (NOT NULL columns).
func numeric(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		t.Fatalf("numeric %q: %v", s, err)
	}
	return n
}

// numStr mengembalikan bentuk teks kanonik pgtype.Numeric untuk perbandingan.
func numStr(t *testing.T, n pgtype.Numeric) string {
	t.Helper()
	v, err := n.Value()
	if err != nil {
		t.Fatalf("numeric value: %v", err)
	}
	s, _ := v.(string)
	return s
}

func i16ptr(v int16) *int16 { return &v }

// seedPlan menyisipkan satu plan (katalog) via SQL mentah di dalam tx ber-tenant —
// tak ada CreatePlan query. plan_code unik per tenant → pemanggil beri code khas.
func seedPlan(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID int64, code, price string) int64 {
	t.Helper()
	var id int64
	if err := WithTenant(ctx, pool, tenantID, func(q *Queries) error {
		return q.db.QueryRow(ctx,
			`INSERT INTO plans (tenant_id, plan_name, plan_code, plan_category, base_price)
			 VALUES ($1, 'Paket Inti', $2, 'Core', $3) RETURNING id`,
			tenantID, code, price).Scan(&id)
	}); err != nil {
		t.Fatalf("seed plan %q: %v", code, err)
	}
	return id
}

// seedQuote membuat satu quote lewat CreateQuote asli (entity_code dari
// GenerateEntityCode) di dalam tx ber-tenant. account_id wajib (NOT NULL).
func seedQuote(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, accountID int64, dealID *int64, name string) Quote {
	t.Helper()
	var out Quote
	if err := WithTenant(ctx, pool, tenantID, func(q *Queries) error {
		code, err := q.GenerateEntityCode(ctx, tenantID, codes.EntityQuote)
		if err != nil {
			return err
		}
		out, err = q.CreateQuote(ctx, CreateQuoteParams{
			TenantID:    tenantID,
			EntityCode:  &code,
			DealID:      dealID,
			AccountID:   accountID,
			QuoteName:   &name,
			QuoteStatus: "Draft",
		})
		return err
	}); err != nil {
		t.Fatalf("seed quote %q: %v", name, err)
	}
	return out
}

func TestCreateAndGetQuote(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Sukamaju", nil)
	deal := seedDeal(t, ctx, pool, ten.ID, acc.ID, "Langganan", nil)

	quote := seedQuote(t, ctx, pool, ten.ID, acc.ID, &deal.ID, "Penawaran-1")
	if quote.EntityCode == nil || *quote.EntityCode != "QUO-001" {
		t.Errorf("entity_code pertama harus QUO-001, got %v", quote.EntityCode)
	}
	if quote.AccountID != acc.ID {
		t.Errorf("account_id harus terikat, got %d want %d", quote.AccountID, acc.ID)
	}
	if quote.DealID == nil || *quote.DealID != deal.ID {
		t.Errorf("deal_id harus terikat, got %v want %d", quote.DealID, deal.ID)
	}
	if quote.QuoteStatus != "Draft" {
		t.Errorf("status awal harus Draft, got %q", quote.QuoteStatus)
	}

	var got Quote
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		got, e = q.GetQuote(ctx, quote.ID)
		return e
	}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != quote.ID || got.QuoteName == nil || *got.QuoteName != "Penawaran-1" {
		t.Errorf("get harus kembalikan quote sama, got id=%d name=%v", got.ID, got.QuoteName)
	}
}

func TestListQuotesForDeal_KeysetDanFilter(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)
	dealA := seedDeal(t, ctx, pool, ten.ID, acc.ID, "Deal-A", nil)
	dealB := seedDeal(t, ctx, pool, ten.ID, acc.ID, "Deal-B", nil)

	// Tiga quote di deal A, satu di deal B (harus TAK muncul saat list deal A).
	for _, n := range []string{"A-Satu", "A-Dua", "A-Tiga"} {
		seedQuote(t, ctx, pool, ten.ID, acc.ID, &dealA.ID, n)
	}
	seedQuote(t, ctx, pool, ten.ID, acc.ID, &dealB.ID, "B-Satu")

	listA := func(cur pgtype.Timestamptz, id int64, size int32) []Quote {
		var rows []Quote
		if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
			var e error
			rows, e = q.ListQuotesForDeal(ctx, ListQuotesForDealParams{
				DealID: &dealA.ID, CursorCreatedAt: cur, CursorID: id, PageSize: size,
			})
			return e
		}); err != nil {
			t.Fatalf("list: %v", err)
		}
		return rows
	}

	c0, id0 := firstCursor()
	p1 := listA(c0, id0, 2)
	if len(p1) != 2 || *p1[0].QuoteName != "A-Tiga" || *p1[1].QuoteName != "A-Dua" {
		t.Fatalf("halaman 1 harus [A-Tiga,A-Dua], got %v", namesOfQuotes(p1))
	}
	last := p1[1]
	p2 := listA(last.CreatedAt, last.ID, 2)
	if len(p2) != 1 || *p2[0].QuoteName != "A-Satu" {
		t.Fatalf("halaman 2 harus [A-Satu] (B-Satu terisolasi), got %v", namesOfQuotes(p2))
	}
}

func TestQuoteItems_OrderingDanSubtotal(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)
	deal := seedDeal(t, ctx, pool, ten.ID, acc.ID, "Deal", nil)
	quote := seedQuote(t, ctx, pool, ten.ID, acc.ID, &deal.ID, "Q")

	// Tambah item dengan line_no acak → ListQuoteItems harus mengurutkannya.
	add := func(lineNo int16, subtotal string) {
		if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
			_, e := q.AddQuoteItem(ctx, AddQuoteItemParams{
				QuoteID:   quote.ID,
				TenantID:  ten.ID,
				Quantity:  1,
				UnitPrice: numeric(t, subtotal),
				Subtotal:  numeric(t, subtotal),
				LineNo:    i16ptr(lineNo),
			})
			return e
		}); err != nil {
			t.Fatalf("add item: %v", err)
		}
	}
	add(2, "200.00")
	add(1, "100.00")
	add(3, "300.00")

	var items []QuoteItem
	var subtotal pgtype.Numeric
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		if items, e = q.ListQuoteItems(ctx, quote.ID); e != nil {
			return e
		}
		subtotal, e = q.QuoteItemsSubtotal(ctx, quote.ID)
		return e
	}); err != nil {
		t.Fatalf("list/subtotal: %v", err)
	}
	if len(items) != 3 || *items[0].LineNo != 1 || *items[1].LineNo != 2 || *items[2].LineNo != 3 {
		t.Fatalf("item harus urut line_no [1,2,3], got %v", lineNosOf(items))
	}
	if got := numStr(t, subtotal); got != "600.00" {
		t.Errorf("subtotal agregat harus 600.00, got %q", got)
	}
}

// TestQuoteItem_SnapshotHargaBeku — acceptance M4: unit_price item dibekukan saat
// dibuat; mengubah plans.base_price SESUDAHNYA tak boleh menyentuhnya.
func TestQuoteItem_SnapshotHargaBeku(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)
	deal := seedDeal(t, ctx, pool, ten.ID, acc.ID, "Deal", nil)
	quote := seedQuote(t, ctx, pool, ten.ID, acc.ID, &deal.ID, "Q")
	planID := seedPlan(t, ctx, pool, ten.ID, "PKG-INTI", "1000.00")

	// Item mengambil snapshot harga plan (1000.00) saat dibuat.
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, e := q.AddQuoteItem(ctx, AddQuoteItemParams{
			QuoteID:   quote.ID,
			TenantID:  ten.ID,
			PlanID:    &planID,
			Quantity:  1,
			UnitPrice: numeric(t, "1000.00"),
			Subtotal:  numeric(t, "1000.00"),
			LineNo:    i16ptr(1),
		})
		return e
	}); err != nil {
		t.Fatalf("add item: %v", err)
	}

	// Katalog plan naik jadi 2000.00 SESUDAH item dibuat.
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, e := q.db.Exec(ctx, `UPDATE plans SET base_price = $1 WHERE id = $2`, "2000.00", planID)
		return e
	}); err != nil {
		t.Fatalf("update plan: %v", err)
	}

	// unit_price item TETAP 1000.00 — tak ikut plan berubah.
	var items []QuoteItem
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		items, e = q.ListQuoteItems(ctx, quote.ID)
		return e
	}); err != nil {
		t.Fatalf("list items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("harus 1 item, got %d", len(items))
	}
	if got := numStr(t, items[0].UnitPrice); got != "1000.00" {
		t.Errorf("unit_price harus BEKU 1000.00 walau plan jadi 2000.00, got %q", got)
	}
}

func TestUpdateQuoteTotals(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)
	deal := seedDeal(t, ctx, pool, ten.ID, acc.ID, "Deal", nil)
	quote := seedQuote(t, ctx, pool, ten.ID, acc.ID, &deal.ID, "Q")

	var got Quote
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		if e := q.UpdateQuoteTotals(ctx, UpdateQuoteTotalsParams{
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
	if numStr(t, got.GrandTotal) != "1500.00" || numStr(t, got.TaxAmount) != "150.00" {
		t.Errorf("total harus tersimpan (grand=1500.00,tax=150.00), got grand=%q tax=%q",
			numStr(t, got.GrandTotal), numStr(t, got.TaxAmount))
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

// ── util diagnostik pesan test ──────────────────────────────────────────────

func namesOfQuotes(rows []Quote) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		if r.QuoteName != nil {
			out[i] = *r.QuoteName
		}
	}
	return out
}

func lineNosOf(rows []QuoteItem) []int16 {
	out := make([]int16, len(rows))
	for i, r := range rows {
		if r.LineNo != nil {
			out[i] = *r.LineNo
		}
	}
	return out
}
