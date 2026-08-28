package db

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// engagements_test.go — bukti query Engagements/Check-ins (CRM Modul 6 slice
// 6.5, tambahan M2 rollup GetLatestEngagementForAccount). Sifat kritis: urutan
// TERBARU dipilih lewat scheduled_at (bukan created_at/insertion order) —
// beda kolom keyset dari kebanyakan query lain di app ini.

// seedEngagement membuat satu engagement lewat CreateEngagement asli di dalam
// tx ber-tenant. Tanpa entity_code (kolom tak ada di tabel ini).
func seedEngagement(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, accountID int64, scheduledAt time.Time, mut func(*CreateEngagementParams)) Engagement {
	t.Helper()
	var out Engagement
	if err := WithTenant(ctx, pool, tenantID, func(q *Queries) error {
		p := CreateEngagementParams{
			TenantID:       tenantID,
			AccountID:      accountID,
			Subject:        "Check-in",
			EngagementType: "check_in",
			ScheduledAt:    pgtype.Timestamptz{Time: scheduledAt, Valid: true},
			Status:         "planned",
		}
		if mut != nil {
			mut(&p)
		}
		var err error
		out, err = q.CreateEngagement(ctx, p)
		return err
	}); err != nil {
		t.Fatalf("seed engagement: %v", err)
	}
	return out
}

func TestGetLatestEngagementForAccount(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)
	csm, _ := q.CreateOAuthUser(ctx, CreateOAuthUserParams{Email: "csm@x", Name: strPtr("Budi CSM")})

	// Dibuat LEBIH DULU tapi dijadwalkan LEBIH BELAKANGAN — harus tetap menang
	// karena keyset memakai scheduled_at, bukan urutan insert/created_at.
	future := seedEngagement(t, ctx, pool, ten.ID, acc.ID, time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC), func(p *CreateEngagementParams) {
		p.Subject = "QBR Depan"
		p.EngagementType = "qbr"
		p.OwnerID = &csm.ID
	})
	seedEngagement(t, ctx, pool, ten.ID, acc.ID, time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC), func(p *CreateEngagementParams) {
		p.Subject = "Check-in Lama"
	})

	var got GetLatestEngagementForAccountRow
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		got, e = q.GetLatestEngagementForAccount(ctx, acc.ID)
		return e
	}); err != nil {
		t.Fatalf("get latest: %v", err)
	}
	if got.ID != future.ID || got.Subject != "QBR Depan" {
		t.Errorf("harus kembalikan engagement dgn scheduled_at TERBARU (QBR Depan), got %q", got.Subject)
	}
	if got.OwnerName == nil || *got.OwnerName != "Budi CSM" {
		t.Errorf("owner_name harus ikut lewat LEFT JOIN users, got %v", got.OwnerName)
	}

	// Desa lain belum punya engagement → pgx.ErrNoRows (empty-state).
	empty := seedAccount(t, ctx, pool, ten.ID, "TanpaEngagement", nil)
	err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, e := q.GetLatestEngagementForAccount(ctx, empty.ID)
		return e
	})
	if err == nil {
		t.Errorf("desa tanpa engagement harus mengembalikan error (pgx.ErrNoRows)")
	}
}
