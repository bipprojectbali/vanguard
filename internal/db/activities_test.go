package db

import (
	"context"
	"testing"

	"go_starter/internal/authz"

	"github.com/jackc/pgx/v5/pgxpool"
)

// activities_test.go — bukti query pipeline aktivitas polimorfik (4.4 Sales
// Activity Log di atas tabel MODUL 7). Sifat yang, bila rusak, tak terlihat dari
// perilaku aplikasi sampai data bocor atau tercampur antar-modul:
//
//   (1) Roundtrip create→get: target polimorfik (target_type+target_id) tersimpan
//       apa adanya (target_id BUKAN FK — jejak bertahan walau target hard-delete).
//   (2) ListActivities = keyset (created_at DESC,id DESC) + ownership F3 +
//       context filter satu query. Keyset menjangkau halaman kedua; ownership
//       fail-closed (flag semua false → nol baris).
//   (3) context_filter memisahkan view modul: baris cs/general TAK muncul di view
//       'sales' — pemisah 4.4 dari CS 6.5 / M7 general.
//   (4) CHECK activities_kind_chk menolak kind liar; soft-delete menyembunyikan.

// seedActivity membuat satu aktivitas lewat CreateActivity asli di dalam tx
// ber-tenant. Default kind=note, target=account, context=sales; mut menyesuaikan.
func seedActivity(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, targetID int64, subject string, mut func(*CreateActivityParams)) Activity {
	t.Helper()
	var out Activity
	if err := WithTenant(ctx, pool, tenantID, func(q *Queries) error {
		p := CreateActivityParams{
			TenantID:        tenantID,
			Kind:            "note",
			Subject:         subject,
			TargetType:      "account",
			TargetID:        targetID,
			ActivityContext: strPtr("sales"),
		}
		if mut != nil {
			mut(&p)
		}
		var err error
		out, err = q.CreateActivity(ctx, p)
		return err
	}); err != nil {
		t.Fatalf("seed activity %q: %v", subject, err)
	}
	return out
}

func TestCreateAndGetActivity(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Sukamaju", nil)

	act := seedActivity(t, ctx, pool, ten.ID, acc.ID, "Kunjungan awal", func(p *CreateActivityParams) {
		p.Kind = "call"
		p.TargetType = "account"
		p.Direction = strPtr("Outbound")
		p.CallResult = strPtr("Connected")
	})
	if act.Kind != "call" {
		t.Errorf("kind harus call, got %q", act.Kind)
	}
	if act.TargetType != "account" || act.TargetID != acc.ID {
		t.Errorf("target harus account:%d, got %s:%d", acc.ID, act.TargetType, act.TargetID)
	}

	var got Activity
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		got, e = q.GetActivity(ctx, act.ID)
		return e
	}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != act.ID || got.Subject != "Kunjungan awal" {
		t.Errorf("get harus kembalikan aktivitas sama, got id=%d subject=%q", got.ID, got.Subject)
	}
}

func TestListActivities_KeysetPagination(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)

	for _, n := range []string{"Satu", "Dua", "Tiga"} {
		seedActivity(t, ctx, pool, ten.ID, acc.ID, n, nil)
	}

	c0, id0 := firstCursor()
	var p1 []Activity
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		p1, e = q.ListActivities(ctx, ListActivitiesParams{
			ContextFilter: "sales", CursorCreatedAt: c0, CursorID: id0, ScopeAll: true, PageSize: 2,
		})
		return e
	}); err != nil {
		t.Fatalf("list p1: %v", err)
	}
	if len(p1) != 2 || p1[0].Subject != "Tiga" || p1[1].Subject != "Dua" {
		t.Fatalf("halaman 1 harus [Tiga,Dua], got %v", subjectsOf(p1))
	}

	last := p1[1]
	var p2 []Activity
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		p2, e = q.ListActivities(ctx, ListActivitiesParams{
			ContextFilter: "sales", CursorCreatedAt: last.CreatedAt, CursorID: last.ID, ScopeAll: true, PageSize: 2,
		})
		return e
	}); err != nil {
		t.Fatalf("list p2: %v", err)
	}
	if len(p2) != 1 || p2[0].Subject != "Satu" {
		t.Fatalf("halaman 2 harus [Satu], got %v", subjectsOf(p2))
	}
}

func TestListActivities_OwnershipFailClosed(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)
	sales, _ := q.CreateUser(ctx, CreateUserParams{Email: "s@x", PassHash: strPtr("x")})
	other, _ := q.CreateUser(ctx, CreateUserParams{Email: "o@x", PassHash: strPtr("x")})

	seedActivity(t, ctx, pool, ten.ID, acc.ID, "Milik-Sales", func(p *CreateActivityParams) { p.OwnerID = &sales.ID })
	seedActivity(t, ctx, pool, ten.ID, acc.ID, "Milik-Lain", func(p *CreateActivityParams) { p.OwnerID = &other.ID })

	listWith := func(f ActivitiesListFilter, uid int64) []Activity {
		var rows []Activity
		c0, id0 := firstCursor()
		if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
			var e error
			rows, e = q.ListActivities(ctx, ListActivitiesParams{
				ContextFilter: "sales", CursorCreatedAt: c0, CursorID: id0,
				ScopeAll: f.ScopeAll, IsOwn: f.IsOwn, Uid: &uid, PageSize: 50,
			})
			return e
		}); err != nil {
			t.Fatalf("list: %v", err)
		}
		return rows
	}

	if got := listWith(ActivitiesListFilter{ScopeAll: true}, sales.ID); len(got) != 2 {
		t.Errorf("scope_all harus 2 aktivitas, got %d", len(got))
	}
	if got := listWith(ActivitiesListFilter{IsOwn: true}, sales.ID); len(got) != 1 || got[0].Subject != "Milik-Sales" {
		t.Errorf("own/sales harus 1 (Milik-Sales), got %d", len(got))
	}
	// Fail-closed: flag semua false → NOL baris (kriteria F3).
	if got := listWith(ActivitiesListFilter{}, sales.ID); len(got) != 0 {
		t.Errorf("tanpa flag harus 0 baris (fail-closed), got %d", len(got))
	}
}

func TestListActivities_ContextFilter(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)

	seedActivity(t, ctx, pool, ten.ID, acc.ID, "Sales-1", nil) // context sales
	seedActivity(t, ctx, pool, ten.ID, acc.ID, "CS-1", func(p *CreateActivityParams) { p.ActivityContext = strPtr("cs") })
	seedActivity(t, ctx, pool, ten.ID, acc.ID, "Gen-1", func(p *CreateActivityParams) { p.ActivityContext = strPtr("general") })

	c0, id0 := firstCursor()
	var sales []Activity
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		sales, e = q.ListActivities(ctx, ListActivitiesParams{
			ContextFilter: "sales", CursorCreatedAt: c0, CursorID: id0, ScopeAll: true, PageSize: 50,
		})
		return e
	}); err != nil {
		t.Fatalf("list sales: %v", err)
	}
	// View 'sales' HANYA baris sales — cs/general milik modul lain, tak boleh bocor.
	if len(sales) != 1 || sales[0].Subject != "Sales-1" {
		t.Errorf("context 'sales' harus 1 (Sales-1), got %v", subjectsOf(sales))
	}
}

func TestCreateActivity_KindCheckMenolakLiar(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)

	err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, e := q.CreateActivity(ctx, CreateActivityParams{
			TenantID: ten.ID, Kind: "warp", Subject: "Liar",
			TargetType: "account", TargetID: acc.ID, ActivityContext: strPtr("sales"),
		})
		return e
	})
	if err == nil {
		t.Errorf("kind liar harus ditolak activities_kind_chk")
	}
}

func TestSoftDeleteActivity(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)
	act := seedActivity(t, ctx, pool, ten.ID, acc.ID, "Buang", nil)

	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.SoftDeleteActivity(ctx, SoftDeleteActivityParams{ID: act.ID})
	}); err != nil {
		t.Fatalf("soft-delete: %v", err)
	}

	err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, e := q.GetActivity(ctx, act.ID)
		return e
	})
	if err == nil {
		t.Errorf("get atas aktivitas ter-soft-delete harus gagal")
	}

	if e := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.SoftDeleteActivity(ctx, SoftDeleteActivityParams{ID: act.ID})
	}); e != nil {
		t.Errorf("soft-delete kedua harus idempotent, got %v", e)
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
