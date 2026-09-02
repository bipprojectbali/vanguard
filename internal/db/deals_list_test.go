package db

import "testing"

// deals_list_test.go — bukti ListDeals (keyset + ownership F3 + stage/mine
// filter). Dipisah dari deals_test.go demi batas File Health (Test 400 baris);
// helper bersama (dealTestEnv, seedDeal, readDeal, namesOfDeals) tetap di sana.

func TestListDeals_KeysetPagination(t *testing.T) {
	ctx, pool, ten, acc := dealTestEnv(t)

	for _, n := range []string{"Satu", "Dua", "Tiga"} {
		seedDeal(t, ctx, pool, ten.ID, acc.ID, n, nil)
	}

	c0, id0 := firstCursor()
	var p1 []Deal
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		p1, e = q.ListDeals(ctx, ListDealsParams{CursorCreatedAt: c0, CursorID: id0, ScopeAll: true, PageSize: 2})
		return e
	}); err != nil {
		t.Fatalf("list p1: %v", err)
	}
	if len(p1) != 2 || p1[0].DealName != "Tiga" || p1[1].DealName != "Dua" {
		t.Fatalf("halaman 1 harus [Tiga,Dua], got %v", namesOfDeals(p1))
	}

	last := p1[1]
	var p2 []Deal
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		p2, e = q.ListDeals(ctx, ListDealsParams{CursorCreatedAt: last.CreatedAt, CursorID: last.ID, ScopeAll: true, PageSize: 2})
		return e
	}); err != nil {
		t.Fatalf("list p2: %v", err)
	}
	if len(p2) != 1 || p2[0].DealName != "Satu" {
		t.Fatalf("halaman 2 harus [Satu], got %v", namesOfDeals(p2))
	}
}

func TestListDeals_OwnershipFailClosed(t *testing.T) {
	ctx, pool, ten, acc := dealTestEnv(t)
	q := New(pool)
	sales, _ := q.CreateUser(ctx, CreateUserParams{Email: "s@x", PassHash: strPtr("x")})
	other, _ := q.CreateUser(ctx, CreateUserParams{Email: "o@x", PassHash: strPtr("x")})

	seedDeal(t, ctx, pool, ten.ID, acc.ID, "Milik-Sales", func(p *CreateDealParams) { p.DealOwner = &sales.ID })
	seedDeal(t, ctx, pool, ten.ID, acc.ID, "Milik-Lain", func(p *CreateDealParams) { p.DealOwner = &other.ID })

	listWith := func(f DealsListFilter, uid int64) []Deal {
		var rows []Deal
		c0, id0 := firstCursor()
		if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
			var e error
			rows, e = q.ListDeals(ctx, ListDealsParams{
				CursorCreatedAt: c0, CursorID: id0,
				ScopeAll: f.ScopeAll, IsOwn: f.IsOwn, Uid: &uid, PageSize: 50,
			})
			return e
		}); err != nil {
			t.Fatalf("list: %v", err)
		}
		return rows
	}

	if got := listWith(DealsListFilter{ScopeAll: true}, sales.ID); len(got) != 2 {
		t.Errorf("scope_all harus 2 deal, got %d", len(got))
	}
	if got := listWith(DealsListFilter{IsOwn: true}, sales.ID); len(got) != 1 || got[0].DealName != "Milik-Sales" {
		t.Errorf("own/sales harus 1 (Milik-Sales), got %d", len(got))
	}
	// Fail-closed: flag semua false → NOL baris (kriteria F3).
	if got := listWith(DealsListFilter{}, sales.ID); len(got) != 0 {
		t.Errorf("tanpa flag harus 0 baris (fail-closed), got %d", len(got))
	}
}

// TestListDeals_MineOnly — sumbu mine_only (BL-10, toggle "Deal Saya") memaksa
// deal_owner = uid walau aktor ScopeAll, di KETIGA jalur (Tabel, Kanban, KPI)
// agar papan & ringkasan seiring. Cermin TestListLeads_StatusFilterAndMineOnly.
func TestListDeals_MineOnly(t *testing.T) {
	ctx, pool, ten, acc := dealTestEnv(t)
	q := New(pool)
	sales, _ := q.CreateUser(ctx, CreateUserParams{Email: "s@x", PassHash: strPtr("x")})
	other, _ := q.CreateUser(ctx, CreateUserParams{Email: "o@x", PassHash: strPtr("x")})

	seedDeal(t, ctx, pool, ten.ID, acc.ID, "Ku-Open", func(p *CreateDealParams) { p.DealOwner = &sales.ID })
	seedDeal(t, ctx, pool, ten.ID, acc.ID, "Ku-Won", func(p *CreateDealParams) {
		p.DealOwner = &sales.ID
		p.Stage = "Closed Won"
	})
	seedDeal(t, ctx, pool, ten.ID, acc.ID, "Lain-Open", func(p *CreateDealParams) { p.DealOwner = &other.ID })

	// ── Tabel (ListDeals) ─────────────────────────────────────────────────────
	c0, id0 := firstCursor()
	listDeals := func(mine bool) []Deal {
		var rows []Deal
		if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
			var e error
			rows, e = q.ListDeals(ctx, ListDealsParams{
				CursorCreatedAt: c0, CursorID: id0, ScopeAll: true,
				Uid: &sales.ID, MineOnly: mine, PageSize: 50,
			})
			return e
		}); err != nil {
			t.Fatalf("list deals (mine=%v): %v", mine, err)
		}
		return rows
	}
	if got := listDeals(false); len(got) != 3 {
		t.Errorf("ScopeAll tanpa mine harus 3 deal, got %d", len(got))
	}
	if got := listDeals(true); len(got) != 2 {
		t.Errorf("mine_only sales harus 2 (miliknya saja), got %d", len(got))
	}

	// ── Kanban (ListDealsForPipeline) ─────────────────────────────────────────
	var pipe []Deal
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		pipe, e = q.ListDealsForPipeline(ctx, ListDealsForPipelineParams{
			ScopeAll: true, Uid: &sales.ID, MineOnly: true, PageSize: 200,
		})
		return e
	}); err != nil {
		t.Fatalf("pipeline mine: %v", err)
	}
	if len(pipe) != 2 {
		t.Errorf("pipeline mine_only harus 2 kartu, got %d", len(pipe))
	}

	// ── KPI (DealPipelineStats) bergerak bersama papan ────────────────────────
	statsFor := func(mine bool) DealPipelineStatsRow {
		var s DealPipelineStatsRow
		if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
			var e error
			s, e = q.DealPipelineStats(ctx, DealPipelineStatsParams{
				ScopeAll: true, Uid: &sales.ID, MineOnly: mine,
			})
			return e
		}); err != nil {
			t.Fatalf("stats mine=%v: %v", mine, err)
		}
		return s
	}
	if all := statsFor(false); all.OpenCount != 2 {
		t.Errorf("open_count tanpa mine harus 2 (Ku-Open+Lain-Open), got %d", all.OpenCount)
	}
	mineStats := statsFor(true)
	if mineStats.OpenCount != 1 {
		t.Errorf("open_count mine harus 1 (Ku-Open saja), got %d", mineStats.OpenCount)
	}
	if mineStats.WonCount != 1 {
		t.Errorf("won_count mine harus 1 (Ku-Won), got %d", mineStats.WonCount)
	}
}

func TestListDeals_StageFilter(t *testing.T) {
	ctx, pool, ten, acc := dealTestEnv(t)

	seedDeal(t, ctx, pool, ten.ID, acc.ID, "Awal", nil) // Prospecting
	seedDeal(t, ctx, pool, ten.ID, acc.ID, "Nego", func(p *CreateDealParams) { p.Stage = "Negotiation" })

	c0, id0 := firstCursor()
	var neg []Deal
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		neg, e = q.ListDeals(ctx, ListDealsParams{
			CursorCreatedAt: c0, CursorID: id0, ScopeAll: true,
			StageFilter: "Negotiation", PageSize: 50,
		})
		return e
	}); err != nil {
		t.Fatalf("list stage: %v", err)
	}
	if len(neg) != 1 || neg[0].DealName != "Nego" {
		t.Errorf("stage_filter Negotiation harus 1 (Nego), got %d", len(neg))
	}
}
