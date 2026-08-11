package db

import (
	"context"
	"sync"
	"testing"

	"go_starter/internal/codes"
)

// entity_codes_test.go — bukti generator kode entitas. Sifat yang bila rusak
// menghancurkan gunanya sebagai PENANDA UNIK, dan gagalnya senyap (kode tampak
// wajar sampai dua baris bertabrakan):
//
//   (1) Nomor mulai 1 dan naik berurutan per (tenant, entity).
//   (2) Counter TERPISAH per tenant — tiap workspace mulai dari 001 (konsisten
//       dengan isolasi RLS).
//   (3) Format tersimpan dipakai; absennya baris format = default bawaan, BUKAN
//       error (workspace baru berkode tanpa seed).
//   (4) Alokasi ATOMIK: N create bersamaan menghasilkan N nomor BERBEDA — tak ada
//       tabrakan di bawah konkurensi (inti kenapa counter di DB, bukan SELECT max+1).

// seedTenant membuat satu tenant lewat owner-pool (bypass RLS) dan mengembalikan
// id-nya. Dipakai test kode entitas yang butuh tenant nyata untuk FK & RLS.
func seedTenant(t *testing.T, ctx context.Context, slug string) int64 {
	t.Helper()
	ten, err := New(pkgPool).CreateTenant(ctx, CreateTenantParams{Name: slug, Slug: slug})
	if err != nil {
		t.Fatalf("seed tenant %s: %v", slug, err)
	}
	return ten.ID
}

func truncateCodes(t *testing.T, ctx context.Context) {
	t.Helper()
	if _, err := pkgPool.Exec(ctx,
		"TRUNCATE code_formats, code_sequences, accounts, tenants RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate codes: %v", err)
	}
}

// TestGenerateEntityCode_DefaultBerurutan: tanpa format tersimpan, kode memakai
// default bawaan (DESA-…) dan nomornya 1,2,3 berurutan mulai dari 1.
func TestGenerateEntityCode_DefaultBerurutan(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCodes(t, ctx)
	tid := seedTenant(t, ctx, "alpha")

	want := []string{"DESA-001", "DESA-002", "DESA-003"}
	for i, w := range want {
		var got string
		if err := WithTenant(ctx, pool, tid, func(q *Queries) error {
			var e error
			got, e = q.GenerateEntityCode(ctx, tid, codes.EntityAccount)
			return e
		}); err != nil {
			t.Fatalf("generate #%d: %v", i+1, err)
		}
		if got != w {
			t.Errorf("kode #%d = %q, want %q", i+1, got, w)
		}
	}
}

// TestGenerateEntityCode_PerTenantTerpisah: dua tenant punya counter sendiri —
// keduanya mulai dari 001. Bila counter global, tenant kedua akan mulai dari 002.
func TestGenerateEntityCode_PerTenantTerpisah(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCodes(t, ctx)
	a := seedTenant(t, ctx, "satu")
	b := seedTenant(t, ctx, "dua")

	var ca, cb string
	if err := WithTenant(ctx, pool, a, func(q *Queries) error {
		var e error
		ca, e = q.GenerateEntityCode(ctx, a, codes.EntityAccount)
		return e
	}); err != nil {
		t.Fatalf("tenant A: %v", err)
	}
	if err := WithTenant(ctx, pool, b, func(q *Queries) error {
		var e error
		cb, e = q.GenerateEntityCode(ctx, b, codes.EntityAccount)
		return e
	}); err != nil {
		t.Fatalf("tenant B: %v", err)
	}
	if ca != "DESA-001" || cb != "DESA-001" {
		t.Errorf("counter tak terpisah per tenant: A=%q B=%q (keduanya harus DESA-001)", ca, cb)
	}
}

// TestGenerateEntityCode_PerEntitasTerpisah: account & lead punya deret sendiri di
// tenant yang sama. account naik tak menggeser lead.
func TestGenerateEntityCode_PerEntitasTerpisah(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCodes(t, ctx)
	tid := seedTenant(t, ctx, "beta")

	var acc1, acc2, lead1 string
	if err := WithTenant(ctx, pool, tid, func(q *Queries) error {
		var e error
		if acc1, e = q.GenerateEntityCode(ctx, tid, codes.EntityAccount); e != nil {
			return e
		}
		if lead1, e = q.GenerateEntityCode(ctx, tid, codes.EntityLead); e != nil {
			return e
		}
		acc2, e = q.GenerateEntityCode(ctx, tid, codes.EntityAccount)
		return e
	}); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if acc1 != "DESA-001" || acc2 != "DESA-002" {
		t.Errorf("deret account salah: %q, %q (want DESA-001, DESA-002)", acc1, acc2)
	}
	if lead1 != "LEAD-001" {
		t.Errorf("deret lead salah: %q (want LEAD-001) — deret entitas tercampur", lead1)
	}
}

// TestGenerateEntityCode_FormatTersimpan: sesudah UpsertCodeFormat, kode memakai
// prefix/separator/padding yang disimpan, bukan default.
func TestGenerateEntityCode_FormatTersimpan(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCodes(t, ctx)
	tid := seedTenant(t, ctx, "gamma")

	var got string
	if err := WithTenant(ctx, pool, tid, func(q *Queries) error {
		if e := q.UpsertCodeFormat(ctx, UpsertCodeFormatParams{
			TenantID:  tid,
			Entity:    string(codes.EntityAccount),
			Prefix:    "VILL",
			Separator: "/",
			Padding:   5,
			CreatedBy: nil,
		}); e != nil {
			return e
		}
		var e error
		got, e = q.GenerateEntityCode(ctx, tid, codes.EntityAccount)
		return e
	}); err != nil {
		t.Fatalf("upsert+generate: %v", err)
	}
	if got != "VILL/00001" {
		t.Errorf("kode = %q, want VILL/00001 (format tersimpan diabaikan?)", got)
	}
}

// TestGenerateEntityCode_TanpaTabrakanKonkuren: INTI kenapa counter atomik di DB.
// N alokasi bersamaan (masing-masing transaksi sendiri) harus menghasilkan N
// nomor BERBEDA. Kalau dihitung SELECT max+1 di Go, sebagian akan bertabrakan.
func TestGenerateEntityCode_TanpaTabrakanKonkuren(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCodes(t, ctx)
	tid := seedTenant(t, ctx, "delta")

	const n = 30
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		seen = make(map[string]int)
		errs []error
	)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var code string
			err := WithTenant(ctx, pool, tid, func(q *Queries) error {
				var e error
				code, e = q.GenerateEntityCode(ctx, tid, codes.EntityAccount)
				return e
			})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, err)
				return
			}
			seen[code]++
		}()
	}
	wg.Wait()

	for _, e := range errs {
		t.Errorf("alokasi konkuren gagal: %v", e)
	}
	if len(seen) != n {
		t.Fatalf("harusnya %d kode unik, dapat %d — ADA TABRAKAN (counter tak atomik)", n, len(seen))
	}
	for code, c := range seen {
		if c != 1 {
			t.Errorf("kode %q muncul %d kali — duplikat", code, c)
		}
	}
}
