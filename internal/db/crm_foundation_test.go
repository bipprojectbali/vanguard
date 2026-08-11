package db

import (
	"context"
	"strings"
	"testing"
)

// crm_foundation_test.go — bukti fondasi CRM (migrasi 00005). Dua sifat yang, bila
// rusak, tak akan terlihat dari perilaku aplikasi sampai data sudah bocor/rusak:
//
//   (1) RLS mengikat di `accounts` (HUB) — desa satu workspace tak bocor ke
//       workspace lain, dan "lupa WHERE tenant_id" tetap 0 baris karena DB yang
//       menolak, bukan aplikasi yang ingat.
//   (2) CHECK business_role menolak nilai di luar enum — sumbu role CRM tak bisa
//       diisi nilai liar lewat jalur mana pun.
//
// Seed pakai OWNER pool (pkgPool, bypass RLS); pembuktian isolasi lewat WithTenant
// (yang `SET LOCAL ROLE app_rw` → RLS BENAR-BENAR aktif, persis jalur query app).

// truncateCRM mengosongkan tabel yang disentuh test ini. CASCADE dari tenants
// sudah menular ke accounts/contacts/plans/sla_policies (semua ON DELETE CASCADE),
// tapi didaftarkan eksplisit agar niatnya terbaca.
func truncateCRM(t *testing.T, ctx context.Context) {
	t.Helper()
	if _, err := pkgPool.Exec(ctx,
		"TRUNCATE contacts, accounts, plans, sla_policies, memberships, users, tenants RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

// TestAccountsRLSIsolation: desa (accounts) milik tenant A tak terlihat dari
// transaksi ber-scope tenant B, dan terlihat dari tenant A. Ini menegakkan bahwa
// "desa = DATA di dalam workspace" tetap terisolasi ANTAR-workspace oleh RLS —
// bukan hanya oleh WHERE di aplikasi.
func TestAccountsRLSIsolation(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ta, err := q.CreateTenant(ctx, CreateTenantParams{Name: "A", Slug: "a"})
	if err != nil {
		t.Fatalf("tenant A: %v", err)
	}
	tb, err := q.CreateTenant(ctx, CreateTenantParams{Name: "B", Slug: "b"})
	if err != nil {
		t.Fatalf("tenant B: %v", err)
	}

	// Insert satu desa ke tenant A lewat jalur ber-tenant (RLS + WITH CHECK aktif).
	if err := WithTenant(ctx, pool, ta.ID, func(q *Queries) error {
		_, e := q.db.Exec(ctx,
			`INSERT INTO accounts (tenant_id, village_name, account_type) VALUES ($1,$2,$3)`,
			ta.ID, "Desa Sukamaju", "prospect")
		return e
	}); err != nil {
		t.Fatalf("insert desa A: %v", err)
	}

	// Dari tenant B: desa A TAK terlihat — query sengaja tanpa WHERE tenant_id,
	// jadi RLS yang menyaring.
	var seenByB int
	if err := WithTenant(ctx, pool, tb.ID, func(q *Queries) error {
		return q.db.QueryRow(ctx, `SELECT count(*) FROM accounts`).Scan(&seenByB)
	}); err != nil {
		t.Fatalf("count dari B: %v", err)
	}
	if seenByB != 0 {
		t.Errorf("tenant B melihat %d desa milik A — RLS BOCOR", seenByB)
	}

	// Dari tenant A: terlihat tepat 1.
	var seenByA int
	if err := WithTenant(ctx, pool, ta.ID, func(q *Queries) error {
		return q.db.QueryRow(ctx, `SELECT count(*) FROM accounts`).Scan(&seenByA)
	}); err != nil {
		t.Fatalf("count dari A: %v", err)
	}
	if seenByA != 1 {
		t.Errorf("tenant A harus melihat 1 desanya sendiri, got %d", seenByA)
	}
}

// TestAccountsWithCheckMenolakTenantLain: WITH CHECK policy menolak menulis baris
// dengan tenant_id ≠ scope aktif. Tanpa ini, satu tenant bisa menanam data ke
// tenant lain (isolasi tulis, bukan hanya baca).
func TestAccountsWithCheckMenolakTenantLain(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ta, err := q.CreateTenant(ctx, CreateTenantParams{Name: "A", Slug: "a"})
	if err != nil {
		t.Fatalf("tenant A: %v", err)
	}
	tb, err := q.CreateTenant(ctx, CreateTenantParams{Name: "B", Slug: "b"})
	if err != nil {
		t.Fatalf("tenant B: %v", err)
	}

	// Scope A, tapi tenant_id=B → harus ditolak WITH CHECK.
	err = WithTenant(ctx, pool, ta.ID, func(q *Queries) error {
		_, e := q.db.Exec(ctx,
			`INSERT INTO accounts (tenant_id, village_name, account_type) VALUES ($1,$2,$3)`,
			tb.ID, "Desa Selundup", "prospect")
		return e
	})
	if err == nil {
		t.Fatal("menulis desa ke tenant lain harus DITOLAK WITH CHECK RLS")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "row-level security") &&
		!strings.Contains(strings.ToLower(err.Error()), "policy") {
		t.Errorf("penolakan harus karena RLS policy, got: %v", err)
	}
}

// TestBusinessRoleNoDBCheck: sejak 00007 peran CRM bisa diedit per-workspace,
// jadi CHECK lama (5 nama hardcoded) DIHAPUS — validasi pindah ke app-layer yang
// tenant-aware (nama harus ada di business_roles workspace itu). Test ini mengunci
// bahwa DB TAK LAGI menyaring nilai: nama custom ('ceo') & NULL diterima di level
// kolom; penjaganya kini query app, bukan constraint. (Composite FK ke
// business_roles sengaja TIDAK dipasang: menghapus peran yang sedang dipegang
// harus meng-UN-ASSIGN member, bukan cascade-menghapus membership-nya.)
func TestBusinessRoleNoDBCheck(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, err := q.CreateTenant(ctx, CreateTenantParams{Name: "A", Slug: "a"})
	if err != nil {
		t.Fatalf("tenant: %v", err)
	}
	u, err := q.CreateUser(ctx, CreateUserParams{Email: "u@a", PassHash: strPtr("x")})
	if err != nil {
		t.Fatalf("user: %v", err)
	}
	m, err := q.CreateMembership(ctx, CreateMembershipParams{UserID: u.ID, TenantID: ten.ID, Role: "owner"})
	if err != nil {
		t.Fatalf("membership: %v", err)
	}

	// Nama custom → DITERIMA di DB (validasi app-layer, bukan CHECK).
	if _, err := pool.Exec(ctx,
		`UPDATE memberships SET business_role = 'ceo' WHERE id = $1`, m.ID); err != nil {
		t.Errorf("business_role custom harus diterima di level DB (CHECK sudah dihapus), got: %v", err)
	}

	// Nama bawaan → tetap diterima.
	for _, role := range []string{"admin", "manager", "sales", "csm", "support"} {
		if _, err := pool.Exec(ctx,
			`UPDATE memberships SET business_role = $1 WHERE id = $2`, role, m.ID); err != nil {
			t.Errorf("business_role=%q harus diterima, got: %v", role, err)
		}
	}

	// NULL (belum diberi peran CRM) → diterima.
	if _, err := pool.Exec(ctx,
		`UPDATE memberships SET business_role = NULL WHERE id = $1`, m.ID); err != nil {
		t.Errorf("business_role=NULL harus diterima (belum diberi peran), got: %v", err)
	}
}

// strPtr — helper lokal (rls_test.go punya ptr() sendiri; hindari tabrakan nama).
func strPtr(s string) *string { return &s }
