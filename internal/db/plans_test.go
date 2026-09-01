package db

import (
	"context"
	"math"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

// plans_test.go — bukti query katalog master Plan (M5 slice 2). Sifat yang, bila
// rusak, tak terlihat sampai katalog salah di produksi:
//
//   (1) CreatePlan roundtrip: currency default 'IDR' terikat; is_active tersimpan.
//   (2) plan_code UNIK per tenant (idx_plans_code) — kode kembar ditolak DB.
//   (3) SetPlanActive memensiunkan: ListPlans (picker) menyembunyikan yang pensiun,
//       ListPlansAll (kelola) tetap menampilkannya.
//   (4) UpdatePlan menyunting profil TANPA menyentuh is_active (jalur terpisah).

func TestCreateAndGetPlan(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})

	var plan Plan
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		plan, e = q.CreatePlan(ctx, CreatePlanParams{
			TenantID: ten.ID, PlanName: "Paket Inti", PlanCode: "PKG-INTI",
			PlanCategory: "Core", IsActive: true, BasePrice: numeric(t, "1000.00"),
			Currency: "IDR",
		})
		return e
	}); err != nil {
		t.Fatalf("create plan: %v", err)
	}
	if plan.Currency != "IDR" || !plan.IsActive {
		t.Errorf("plan harus currency=IDR & aktif, got currency=%q active=%v", plan.Currency, plan.IsActive)
	}

	var got Plan
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		got, e = q.GetPlan(ctx, plan.ID)
		return e
	}); err != nil {
		t.Fatalf("get plan: %v", err)
	}
	if got.ID != plan.ID || got.PlanName != "Paket Inti" {
		t.Errorf("get harus kembalikan plan sama, got id=%d name=%q", got.ID, got.PlanName)
	}
}

func TestCreatePlan_DuplicateCodeRejected(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})

	mk := func() error {
		return WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
			_, e := q.CreatePlan(ctx, CreatePlanParams{
				TenantID: ten.ID, PlanName: "Paket", PlanCode: "PKG-DUP",
				PlanCategory: "Core", IsActive: true, Currency: "IDR",
			})
			return e
		})
	}
	if err := mk(); err != nil {
		t.Fatalf("create pertama: %v", err)
	}
	if err := mk(); err == nil {
		t.Errorf("plan_code kembar harus ditolak idx_plans_code")
	}
}

func TestSetPlanActive_HidesFromPicker(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})

	var plan Plan
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		plan, e = q.CreatePlan(ctx, CreatePlanParams{
			TenantID: ten.ID, PlanName: "Paket Pensiun", PlanCode: "PKG-OLD",
			PlanCategory: "Core", IsActive: true, Currency: "IDR",
		})
		return e
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Pensiunkan.
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.SetPlanActive(ctx, SetPlanActiveParams{IsActive: false, ID: plan.ID})
	}); err != nil {
		t.Fatalf("set inactive: %v", err)
	}

	var picker, all []Plan
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		if picker, e = q.ListPlans(ctx); e != nil {
			return e
		}
		all, e = q.ListPlansAll(ctx, ListPlansAllParams{
			CursorCreatedAt: pgtype.Timestamptz{Valid: true, InfinityModifier: pgtype.Infinity},
			CursorID:        math.MaxInt64,
			PageSize:        1000,
		})
		return e
	}); err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(picker) != 0 {
		t.Errorf("ListPlans (picker) harus sembunyikan plan pensiun, got %d", len(picker))
	}
	if len(all) != 1 || all[0].IsActive {
		t.Errorf("ListPlansAll harus tampilkan plan pensiun (is_active=false), got %d", len(all))
	}
}

func TestUpdatePlan_ProfileOnly(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})

	var plan, got Plan
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		plan, e = q.CreatePlan(ctx, CreatePlanParams{
			TenantID: ten.ID, PlanName: "Awal", PlanCode: "PKG-1",
			PlanCategory: "Core", IsActive: true, BasePrice: numeric(t, "500.00"),
			Currency: "IDR",
		})
		if e != nil {
			return e
		}
		got, e = q.UpdatePlan(ctx, UpdatePlanParams{
			PlanName: "Diperbarui", PlanCode: "PKG-1", PlanCategory: "Module",
			BasePrice: numeric(t, "750.00"), Currency: "IDR", ID: plan.ID,
		})
		return e
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got.PlanName != "Diperbarui" || got.PlanCategory != "Module" || numStr(t, got.BasePrice) != "750.00" {
		t.Errorf("update harus ubah profil, got name=%q cat=%q price=%q", got.PlanName, got.PlanCategory, numStr(t, got.BasePrice))
	}
	// is_active TAK tersentuh UpdatePlan (jalur SetPlanActive terpisah).
	if !got.IsActive {
		t.Errorf("is_active harus tetap true (UpdatePlan tak menyentuhnya)")
	}
}
