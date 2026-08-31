package db

import (
	"context"
	"testing"

	"go_starter/internal/authz"

	"github.com/jackc/pgx/v5/pgtype"
)

// ── ListActivitiesByTarget (M7-A) ────────────────────────────────────────────

// TestListActivitiesByTarget_SpansContexts membuktikan bahwa ListActivitiesByTarget
// TIDAK menyaring per context_filter: baris sales/cs/general SEMUANYA muncul di
// timeline entitas, berbeda dari ListActivities yang scope-per-modul.
func TestListActivitiesByTarget_SpansContexts(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Desa", nil)

	seedActivity(t, ctx, pool, ten.ID, acc.ID, "Sales-act", nil) // context=sales
	seedActivity(t, ctx, pool, ten.ID, acc.ID, "CS-act", func(p *CreateActivityParams) {
		p.ActivityContext = strPtr("cs")
	})
	seedActivity(t, ctx, pool, ten.ID, acc.ID, "Gen-act", func(p *CreateActivityParams) {
		p.ActivityContext = strPtr("general")
	})

	c0, id0 := firstCursor()
	var rows []Activity
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		rows, e = q.ListActivitiesByTarget(ctx, ListActivitiesByTargetParams{
			TargetType:      "account",
			TargetID:        acc.ID,
			CursorCreatedAt: c0,
			CursorID:        id0,
			PageSize:        50,
		})
		return e
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
	// Semua tiga konteks harus muncul.
	if len(rows) != 3 {
		t.Errorf("harus 3 baris (lintas konteks), got %d: %v", len(rows), subjectsOf(rows))
	}
}

// TestListActivitiesByTarget_IsolatedByTarget membuktikan isolasi: aktivitas di
// target lain (target_id atau target_type beda) TIDAK ikut dalam hasil.
func TestListActivitiesByTarget_IsolatedByTarget(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc1 := seedAccount(t, ctx, pool, ten.ID, "Desa-A", nil)
	acc2 := seedAccount(t, ctx, pool, ten.ID, "Desa-B", nil)

	seedActivity(t, ctx, pool, ten.ID, acc1.ID, "Aksi-di-A", nil)
	seedActivity(t, ctx, pool, ten.ID, acc2.ID, "Aksi-di-B", nil)
	// target_type "deal" dengan id yang sama dengan acc1 — TIDAK boleh bocor ke timeline acc1.
	seedActivity(t, ctx, pool, ten.ID, acc1.ID, "Deal-ghost", func(p *CreateActivityParams) {
		p.TargetType = "deal"
		p.TargetID = acc1.ID
	})

	c0, id0 := firstCursor()
	var rows []Activity
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		rows, e = q.ListActivitiesByTarget(ctx, ListActivitiesByTargetParams{
			TargetType:      "account",
			TargetID:        acc1.ID,
			CursorCreatedAt: c0,
			CursorID:        id0,
			PageSize:        50,
		})
		return e
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
	// Hanya "Aksi-di-A" — acc2 dan deal-ghost TIDAK boleh muncul.
	if len(rows) != 1 || rows[0].Subject != "Aksi-di-A" {
		t.Errorf("harus 1 (Aksi-di-A), got %d: %v", len(rows), subjectsOf(rows))
	}
}

// TestListActivitiesByTarget_KeysetPagination membuktikan keyset (created_at DESC,
// id DESC) bekerja: halaman 1 mengembalikan N terbaru, halaman 2 menyambung.
func TestListActivitiesByTarget_KeysetPagination(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Desa", nil)

	for _, n := range []string{"Alpha", "Beta", "Gamma"} {
		seedActivity(t, ctx, pool, ten.ID, acc.ID, n, nil)
	}

	c0, id0 := firstCursor()
	listPage := func(cursorAt pgtype.Timestamptz, cursorID int64, size int32) []Activity {
		var rows []Activity
		if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
			var e error
			rows, e = q.ListActivitiesByTarget(ctx, ListActivitiesByTargetParams{
				TargetType:      "account",
				TargetID:        acc.ID,
				CursorCreatedAt: cursorAt,
				CursorID:        cursorID,
				PageSize:        size,
			})
			return e
		}); err != nil {
			t.Fatalf("list: %v", err)
		}
		return rows
	}

	p1 := listPage(c0, id0, 2)
	if len(p1) != 2 || p1[0].Subject != "Gamma" || p1[1].Subject != "Beta" {
		t.Fatalf("p1 harus [Gamma,Beta], got %v", subjectsOf(p1))
	}
	last := p1[1]
	p2 := listPage(last.CreatedAt, last.ID, 2)
	if len(p2) != 1 || p2[0].Subject != "Alpha" {
		t.Fatalf("p2 harus [Alpha], got %v", subjectsOf(p2))
	}
}

// TestListActivitiesByTarget_SoftDeletedExcluded membuktikan bahwa aktivitas
// ter-soft-delete TIDAK muncul di timeline entitas.
func TestListActivitiesByTarget_SoftDeletedExcluded(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Desa", nil)

	live := seedActivity(t, ctx, pool, ten.ID, acc.ID, "Aktif", nil)
	dead := seedActivity(t, ctx, pool, ten.ID, acc.ID, "Terhapus", nil)

	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.SoftDeleteActivity(ctx, SoftDeleteActivityParams{ID: dead.ID})
	}); err != nil {
		t.Fatalf("soft-delete: %v", err)
	}

	c0, id0 := firstCursor()
	var rows []Activity
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		rows, e = q.ListActivitiesByTarget(ctx, ListActivitiesByTargetParams{
			TargetType:      "account",
			TargetID:        acc.ID,
			CursorCreatedAt: c0,
			CursorID:        id0,
			PageSize:        50,
		})
		return e
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != live.ID {
		t.Errorf("harus hanya baris aktif (id=%d), got %d: %v", live.ID, len(rows), subjectsOf(rows))
	}
}

// ── Unit (tanpa DB) ─────────────────────────────────────────────────────────

func TestActivitiesListFilterFor(t *testing.T) {
	cases := []struct {
		scope string
		want  ActivitiesListFilter
	}{
		{authz.DataScopeAll, ActivitiesListFilter{ScopeAll: true}},
		{authz.DataScopeOwn, ActivitiesListFilter{IsOwn: true}},
		{authz.DataScopeNone, ActivitiesListFilter{}},
		{"", ActivitiesListFilter{}},
		{"galaxy", ActivitiesListFilter{}},
	}
	for _, c := range cases {
		if got := ActivitiesListFilterFor(c.scope); got != c.want {
			t.Errorf("ActivitiesListFilterFor(%q)=%+v, want %+v", c.scope, got, c.want)
		}
	}
}

func TestActivitiesListFilter_Allows(t *testing.T) {
	const me, notMe = int64(7), int64(99)
	p := func(v int64) *int64 { return &v }

	if !(ActivitiesListFilter{ScopeAll: true}).Allows(me, nil) {
		t.Error("ScopeAll harus mengizinkan baris tanpa owner")
	}
	own := ActivitiesListFilter{IsOwn: true}
	if !own.Allows(me, p(me)) {
		t.Error("own harus boleh atas aktivitas miliknya")
	}
	if own.Allows(me, p(notMe)) {
		t.Error("own tak boleh atas aktivitas orang lain")
	}
	if (ActivitiesListFilter{}).Allows(me, p(me)) {
		t.Error("filter kosong harus menolak semua (fail-closed)")
	}
}

// subjectsOf — util diagnostik pesan test.
func subjectsOf(rows []Activity) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Subject
	}
	return out
}
