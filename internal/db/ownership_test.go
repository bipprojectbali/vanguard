package db

import (
	"context"
	"testing"
)

// ownership_test.go — bukti isolasi ANTAR-DESA (F3). RLS sudah membuktikan
// isolasi antar-WORKSPACE (crm_foundation_test.go); di sini pertanyaannya beda:
// dalam SATU workspace, apakah seorang sales hanya melihat desa yang ditugaskan
// padanya, bukan desa milik sales lain?
//
// Kriteria penerimaan F3 (tasks.md): "query TANPA filter ini = test gagal". Maka
// TestOwnershipFilter_TanpaFilterBocor menegakkan justru itu — query polos
// membocorkan desa orang lain; hanya klausa ownership yang menahannya.

func TestOwnershipFilter_TanpaFilterBocor(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, err := q.CreateTenant(ctx, CreateTenantParams{Name: "Desa+", Slug: "desaplus"})
	if err != nil {
		t.Fatalf("tenant: %v", err)
	}
	salesA, err := q.CreateUser(ctx, CreateUserParams{Email: "salesA@x", PassHash: strPtr("x")})
	if err != nil {
		t.Fatalf("salesA: %v", err)
	}
	salesB, err := q.CreateUser(ctx, CreateUserParams{Email: "salesB@x", PassHash: strPtr("x")})
	if err != nil {
		t.Fatalf("salesB: %v", err)
	}

	// 3 desa: 2 milik salesA (account_owner), 1 milik salesB — semua di tenant sama.
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		for _, o := range []struct {
			name  string
			owner int64
		}{{"Sukamaju", salesA.ID}, {"Sukamundur", salesA.ID}, {"Sukasenang", salesB.ID}} {
			if _, e := q.db.Exec(ctx,
				`INSERT INTO accounts (tenant_id, village_name, account_type, account_owner) VALUES ($1,$2,'prospect',$3)`,
				ten.ID, o.name, o.owner); e != nil {
				return e
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed desa: %v", err)
	}

	countWith := func(clause string, args ...any) int {
		var n int
		sql := `SELECT count(*) FROM accounts`
		if clause != "" {
			sql += ` WHERE ` + clause
		}
		if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
			return q.db.QueryRow(ctx, sql, args...).Scan(&n)
		}); err != nil {
			t.Fatalf("count [%s]: %v", clause, err)
		}
		return n
	}

	// (1) TANPA filter ownership: salesA melihat SEMUA 3 desa — termasuk milik
	// salesB. Inilah kebocoran yang F3 tutup; kalau ini bukan 3, premisnya salah.
	if got := countWith(""); got != 3 {
		t.Fatalf("tanpa filter harus lihat semua 3 desa (premis kebocoran), got %d", got)
	}

	// (2) DENGAN klausa ownership cakupan 'own' (union kepemilikan): salesA hanya 2
	// (miliknya lewat account_owner), salesB hanya 1. Union mencakup account_owner
	// juga, jadi sales murni tetap terjaring lewat kolom itu.
	clauseA, argsA, _ := AccountsOwnershipClause("own", salesA.ID, 1)
	if clauseA == "" || clauseA == "FALSE" {
		t.Fatalf("own harus menghasilkan klausa kepemilikan, got %q", clauseA)
	}
	if got := countWith(clauseA, argsA...); got != 2 {
		t.Errorf("salesA dengan filter harus lihat 2 desanya, got %d", got)
	}
	clauseB, argsB, _ := AccountsOwnershipClause("own", salesB.ID, 1)
	if got := countWith(clauseB, argsB...); got != 1 {
		t.Errorf("salesB dengan filter harus lihat 1 desanya, got %d", got)
	}
}

// TestOwnershipFilter_CSMBinaanDanCadangan: role bercakupan 'own' melihat desa
// binaan (assigned_csm) DAN desa tempat ia jadi cadangan (backup_csm) — §8.1.
// Klausa union merujuk satu placeholder tiga kali, jadi satu arg cukup.
func TestOwnershipFilter_CSMBinaanDanCadangan(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	csm, _ := q.CreateUser(ctx, CreateUserParams{Email: "csm@x", PassHash: strPtr("x")})
	other, _ := q.CreateUser(ctx, CreateUserParams{Email: "other@x", PassHash: strPtr("x")})

	// binaan: assigned_csm=csm; cadangan: backup_csm=csm; asing: keduanya other.
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		rows := []struct {
			name             string
			assigned, backup int64
		}{
			{"Binaan", csm.ID, other.ID},
			{"Cadangan", other.ID, csm.ID},
			{"Asing", other.ID, other.ID},
		}
		for _, r := range rows {
			if _, e := q.db.Exec(ctx,
				`INSERT INTO accounts (tenant_id, village_name, account_type, assigned_csm, backup_csm) VALUES ($1,$2,'customer',$3,$4)`,
				ten.ID, r.name, r.assigned, r.backup); e != nil {
				return e
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	clause, args, next := AccountsOwnershipClause("own", csm.ID, 1)
	if next != 2 {
		t.Errorf("cakupan own harus pakai TEPAT 1 placeholder (di-refer tiga kali), nextIdx=%d", next)
	}
	if len(args) != 1 {
		t.Errorf("cakupan own harus 1 arg untuk 3 kolom, got %d", len(args))
	}
	var n int
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.db.QueryRow(ctx, `SELECT count(*) FROM accounts WHERE `+clause, args...).Scan(&n)
	}); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 2 {
		t.Errorf("CSM harus lihat binaan+cadangan = 2 desa, got %d", n)
	}
}

// TestAccountsScopeFor menegakkan pemetaan data_scope→cakupan, termasuk default
// DEFENSIF: 'none' & nilai kosong/liar → ScopeNone (nol baris, bukan bocor).
// Cakupan kini dibaca dari KOLOM data_scope (F3 editable), bukan ditebak dari
// nama role — role custom bisa bercakupan apa saja.
func TestAccountsScopeFor(t *testing.T) {
	cases := []struct {
		scope string
		want  OwnershipScope
	}{
		{"all", ScopeAll},
		{"own", ScopeOwn},
		{"none", ScopeNone},   // desa hanya lewat konteks tiket, bukan daftar umum
		{"", ScopeNone},       // belum diberi peran → fail-closed
		{"galaxy", ScopeNone}, // nilai liar → aman, bukan panik
	}
	for _, c := range cases {
		if got := AccountsScopeFor(c.scope); got != c.want {
			t.Errorf("AccountsScopeFor(%q)=%d, want %d", c.scope, got, c.want)
		}
	}
	// ScopeAll = tanpa batas (clause kosong); ScopeNone = FALSE (nol baris).
	if cl, _, _ := AccountsOwnershipClause("all", 1, 1); cl != "" {
		t.Errorf("all harus tanpa klausa (lihat semua), got %q", cl)
	}
	if cl, _, _ := AccountsOwnershipClause("none", 1, 1); cl != "FALSE" {
		t.Errorf("none harus FALSE (nol baris), got %q", cl)
	}
}
