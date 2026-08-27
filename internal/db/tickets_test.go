package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// tickets_test.go — bukti CountTicketsByAccount (chip "Terkait" di detail
// Account, M2 rollup). Predikat breach HARUS sama persis dgn CountTicketKPIs
// (tickets tanpa kolom deleted_at — lihat header queries/tickets.sql).
//
// CreateTicket selalu menyetel status awal 'baru' (bukan dari params) — tiket
// diselesaikan lewat UpdateTicketStatus terpisah, bukan saat create.

// seedTicket membuat satu tiket lewat CreateTicket asli di dalam tx ber-tenant.
// Status awal selalu 'baru'.
func seedTicket(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, accountID int64, mut func(*CreateTicketParams)) Ticket {
	t.Helper()
	var out Ticket
	if err := WithTenant(ctx, pool, tenantID, func(q *Queries) error {
		p := CreateTicketParams{
			TenantID:  tenantID,
			AccountID: accountID,
			Subject:   "Masalah",
			Priority:  "sedang",
		}
		if mut != nil {
			mut(&p)
		}
		var err error
		out, err = q.CreateTicket(ctx, p)
		return err
	}); err != nil {
		t.Fatalf("seed ticket: %v", err)
	}
	return out
}

func TestCountTicketsByAccount(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)

	// Deadline sudah lewat TAPI diselesaikan → tak terhitung open maupun breach.
	resolved := seedTicket(t, ctx, pool, ten.ID, acc.ID, func(p *CreateTicketParams) {
		p.SlaDeadlineAt = pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true}
	})
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, e := q.UpdateTicketStatus(ctx, UpdateTicketStatusParams{Status: "selesai", ID: resolved.ID})
		return e
	}); err != nil {
		t.Fatalf("selesaikan tiket: %v", err)
	}

	// Terbuka ('baru'), deadline BELUM lewat → open, non-breach.
	seedTicket(t, ctx, pool, ten.ID, acc.ID, func(p *CreateTicketParams) {
		p.SlaDeadlineAt = pgtype.Timestamptz{Time: time.Now().Add(time.Hour), Valid: true}
	})
	// Terbuka ('baru'), deadline SUDAH lewat → open DAN breach.
	seedTicket(t, ctx, pool, ten.ID, acc.ID, func(p *CreateTicketParams) {
		p.SlaDeadlineAt = pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true}
	})

	var got CountTicketsByAccountRow
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		got, e = q.CountTicketsByAccount(ctx, acc.ID)
		return e
	}); err != nil {
		t.Fatalf("count: %v", err)
	}
	// 3 tiket total: 1 selesai (dikecualikan), 2 terbuka (1 non-breach, 1 breach).
	if got.OpenCount != 2 {
		t.Errorf("open_count harus 2 (selesai dikecualikan), got %d", got.OpenCount)
	}
	if got.BreachedCount != 1 {
		t.Errorf("breached_count harus 1 (hanya yg terbuka+lewat deadline), got %d", got.BreachedCount)
	}
}
