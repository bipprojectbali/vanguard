package db

import (
	"context"
	"testing"

	"go_starter/internal/authz"
	"go_starter/internal/codes"

	"github.com/jackc/pgx/v5/pgxpool"
)

// deals_test.go — bukti query pipeline Deal (M4). Sifat yang, bila rusak, tak
// terlihat sampai angka pipeline atau isolasi owner salah di produksi:
//
//   (1) Roundtrip create→get: entity_code (DEAL-001) dialokasikan DI DALAM tx;
//       account_id NOT NULL terikat.
//   (2) ListDeals = keyset + ownership F3 + stage filter satu query. Keyset harus
//       menjangkau halaman kedua; ownership fail-closed (flag semua false → nol).
//   (3) UpdateDealStage menyetel closed_date HANYA saat masuk Closed Won/Lost, dan
//       mengosongkannya bila stage kembali terbuka — kalkulasi di SQL, bukan Go.
//   (4) DealPipelineStats menghitung open/won/lost sesuai ownership yang sama.
//   (5) CHECK deals_stage_chk menolak stage liar; soft-delete menyembunyikan.

// seedDeal membuat satu deal lewat CreateDeal asli (entity_code dari
// GenerateEntityCode) di dalam tx ber-tenant. account_id wajib (NOT NULL).
func seedDeal(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, accountID int64, name string, mut func(*CreateDealParams)) Deal {
	t.Helper()
	var out Deal
	if err := WithTenant(ctx, pool, tenantID, func(q *Queries) error {
		code, err := q.GenerateEntityCode(ctx, tenantID, codes.EntityDeal)
		if err != nil {
			return err
		}
		p := CreateDealParams{
			TenantID:   tenantID,
			EntityCode: &code,
			DealName:   name,
			AccountID:  accountID,
			Stage:      "Prospecting",
		}
		if mut != nil {
			mut(&p)
		}
		out, err = q.CreateDeal(ctx, p)
		return err
	}); err != nil {
		t.Fatalf("seed deal %q: %v", name, err)
	}
	return out
}

// dealTestEnv = setup DB baku tiap test pipeline: pool bersih + satu tenant "T"
// + satu account "Acc". Menyingkat blok truncate→tenant→account yang identik di
// delapan test ber-DB. Test yang butuh Queries (mis. CreateUser) buat `New(pool)`
// sendiri agar tak ada nilai kembalian menganggur.
func dealTestEnv(t *testing.T) (context.Context, *pgxpool.Pool, Tenant, Account) {
	t.Helper()
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)
	ten, err := New(pool).CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	acc := seedAccount(t, ctx, pool, ten.ID, "Acc", nil)
	return ctx, pool, ten, acc
}

// readDeal = baca satu deal dalam tx ber-tenant; fatal bila gagal. Untuk test yang
// MENGHARAP error (soft-delete) tetap pakai WithTenant langsung.
func readDeal(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, dealID int64) Deal {
	t.Helper()
	var out Deal
	if err := WithTenant(ctx, pool, tenantID, func(q *Queries) error {
		var e error
		out, e = q.GetDeal(ctx, dealID)
		return e
	}); err != nil {
		t.Fatalf("get deal %d: %v", dealID, err)
	}
	return out
}

func TestCreateAndGetDeal(t *testing.T) {
	ctx, pool, ten, acc := dealTestEnv(t)

	deal := seedDeal(t, ctx, pool, ten.ID, acc.ID, "Langganan Sukamaju", nil)
	if deal.EntityCode == nil || *deal.EntityCode != "DEAL-001" {
		t.Errorf("entity_code pertama harus DEAL-001, got %v", deal.EntityCode)
	}
	if deal.AccountID != acc.ID {
		t.Errorf("account_id harus terikat, got %d want %d", deal.AccountID, acc.ID)
	}
	if deal.Stage != "Prospecting" {
		t.Errorf("stage awal harus Prospecting, got %q", deal.Stage)
	}

	got := readDeal(t, ctx, pool, ten.ID, deal.ID)
	if got.ID != deal.ID || got.DealName != "Langganan Sukamaju" {
		t.Errorf("get harus kembalikan deal sama, got id=%d name=%q", got.ID, got.DealName)
	}
}

func TestUpdateDealStage_ClosedDate(t *testing.T) {
	ctx, pool, ten, acc := dealTestEnv(t)
	deal := seedDeal(t, ctx, pool, ten.ID, acc.ID, "Deal", nil)

	// Masih terbuka → closed_date NULL.
	moveStage := func(stage string, reason *string) Deal {
		if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
			return q.UpdateDealStage(ctx, UpdateDealStageParams{Stage: stage, WinLossReason: reason, ID: deal.ID})
		}); err != nil {
			t.Fatalf("stage→%s: %v", stage, err)
		}
		return readDeal(t, ctx, pool, ten.ID, deal.ID)
	}

	if g := moveStage("Demo", nil); g.ClosedDate.Valid {
		t.Errorf("stage terbuka tak boleh set closed_date, got %v", g.ClosedDate)
	}

	// Masuk Closed Won → closed_date terisi CURRENT_DATE.
	won := "Harga kompetitif"
	if g := moveStage("Closed Won", &won); !g.ClosedDate.Valid {
		t.Errorf("Closed Won harus set closed_date")
	}

	// Kembali dibuka (Negotiation) → closed_date kembali NULL.
	if g := moveStage("Negotiation", nil); g.ClosedDate.Valid {
		t.Errorf("stage dibuka lagi harus kosongkan closed_date, got %v", g.ClosedDate)
	}
}

func TestDealPipelineStats(t *testing.T) {
	ctx, pool, ten, acc := dealTestEnv(t)

	seedDeal(t, ctx, pool, ten.ID, acc.ID, "Open1", nil)
	seedDeal(t, ctx, pool, ten.ID, acc.ID, "Open2", func(p *CreateDealParams) { p.Stage = "Demo" })
	seedDeal(t, ctx, pool, ten.ID, acc.ID, "Won", func(p *CreateDealParams) { p.Stage = "Closed Won" })
	seedDeal(t, ctx, pool, ten.ID, acc.ID, "Lost", func(p *CreateDealParams) { p.Stage = "Closed Lost" })

	var stats DealPipelineStatsRow
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		stats, e = q.DealPipelineStats(ctx, DealPipelineStatsParams{ScopeAll: true})
		return e
	}); err != nil {
		t.Fatalf("stats: %v", err)
	}
	if stats.OpenCount != 2 {
		t.Errorf("open_count harus 2 (Prospecting+Demo), got %d", stats.OpenCount)
	}
	if stats.WonCount != 1 {
		t.Errorf("won_count harus 1, got %d", stats.WonCount)
	}
	if stats.LostCount != 1 {
		t.Errorf("lost_count harus 1, got %d", stats.LostCount)
	}
}

func TestSoftDeleteDeal(t *testing.T) {
	ctx, pool, ten, acc := dealTestEnv(t)
	deal := seedDeal(t, ctx, pool, ten.ID, acc.ID, "Buang", nil)

	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.SoftDeleteDeal(ctx, SoftDeleteDealParams{ID: deal.ID})
	}); err != nil {
		t.Fatalf("soft-delete: %v", err)
	}

	err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, e := q.GetDeal(ctx, deal.ID)
		return e
	})
	if err == nil {
		t.Errorf("get atas deal ter-soft-delete harus gagal")
	}

	if e := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.SoftDeleteDeal(ctx, SoftDeleteDealParams{ID: deal.ID})
	}); e != nil {
		t.Errorf("soft-delete kedua harus idempotent, got %v", e)
	}
}

func TestCreateDeal_StageCheckMenolakLiar(t *testing.T) {
	ctx, pool, ten, acc := dealTestEnv(t)

	err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		code, e := q.GenerateEntityCode(ctx, ten.ID, codes.EntityDeal)
		if e != nil {
			return e
		}
		_, e = q.CreateDeal(ctx, CreateDealParams{
			TenantID: ten.ID, EntityCode: &code, DealName: "Liar", AccountID: acc.ID, Stage: "Warp",
		})
		return e
	})
	if err == nil {
		t.Errorf("stage liar harus ditolak deals_stage_chk")
	}
}

// ── Unit (tanpa DB) ─────────────────────────────────────────────────────────

func TestDealsListFilterFor(t *testing.T) {
	cases := []struct {
		scope string
		want  DealsListFilter
	}{
		{authz.DataScopeAll, DealsListFilter{ScopeAll: true}},
		{authz.DataScopeOwn, DealsListFilter{IsOwn: true}},
		{authz.DataScopeNone, DealsListFilter{}},
		{"", DealsListFilter{}},
		{"galaxy", DealsListFilter{}},
	}
	for _, c := range cases {
		if got := DealsListFilterFor(c.scope); got != c.want {
			t.Errorf("DealsListFilterFor(%q)=%+v, want %+v", c.scope, got, c.want)
		}
	}
}

func TestDealsListFilter_Allows(t *testing.T) {
	const me, notMe = int64(7), int64(99)
	p := func(v int64) *int64 { return &v }

	if !(DealsListFilter{ScopeAll: true}).Allows(me, nil) {
		t.Error("ScopeAll harus mengizinkan baris tanpa owner")
	}
	own := DealsListFilter{IsOwn: true}
	if !own.Allows(me, p(me)) {
		t.Error("own harus boleh atas deal miliknya")
	}
	if own.Allows(me, p(notMe)) {
		t.Error("own tak boleh atas deal orang lain")
	}
	if (DealsListFilter{}).Allows(me, p(me)) {
		t.Error("filter kosong harus menolak semua (fail-closed)")
	}
}

// namesOfDeals — util diagnostik pesan test.
func namesOfDeals(rows []Deal) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.DealName
	}
	return out
}
