package db

import (
	"context"
	"testing"

	"go_starter/internal/authz"

	"github.com/jackc/pgx/v5/pgtype"
)

// accounts_list_test.go — bukti DAFTAR desa & filter kepemilikan F3 (dipisah dari
// accounts_test.go yang menjaga CRUD/siklus hidup). Satu concern: "desa siapa yang
// tampil". Dua lapis yang, bila rusak, membocorkan atau menyembunyikan baris:
//
//   (1) ListAccounts = keyset + ownership dalam satu query. Keyset harus bisa
//       menjangkau halaman kedua (bukan cuma LIMIT); ownership fail-closed (flag
//       semua false → nol baris, bukan bocor).
//   (2) AccountsListFilterFor/Allows = perakitan flag & keputusan per-baris, SATU
//       sumber dengan yang disaring ListAccounts — diuji sejajar agar "yang tampil
//       di daftar" & "yang boleh dibuka di detail" tak pernah beda jawaban.
//
// Helper firstCursor/seedAccount tinggal di accounts_test.go (paket sama).

func TestListAccounts_KeysetPagination(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})

	// 3 desa; created_at berurutan (entity seq menjamin urutan insert).
	for _, n := range []string{"Satu", "Dua", "Tiga"} {
		seedAccount(t, ctx, pool, ten.ID, n, nil)
	}

	list := func(after pgtype.Timestamptz, afterID int64, size int32) []Account {
		var rows []Account
		if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
			var e error
			rows, e = q.ListAccounts(ctx, ListAccountsParams{
				CursorCreatedAt: after,
				CursorID:        afterID,
				ScopeAll:        true, // admin/manager: lihat semua
				PageSize:        size,
			})
			return e
		}); err != nil {
			t.Fatalf("list: %v", err)
		}
		return rows
	}

	// Halaman 1: page_size=2 → 2 baris terbaru (Tiga, Dua) karena DESC.
	c0, id0 := firstCursor()
	p1 := list(c0, id0, 2)
	if len(p1) != 2 {
		t.Fatalf("halaman 1 harus 2 baris, got %d", len(p1))
	}
	if p1[0].VillageName != "Tiga" || p1[1].VillageName != "Dua" {
		t.Errorf("urutan DESC salah: %q,%q", p1[0].VillageName, p1[1].VillageName)
	}

	// Halaman 2: cursor dari baris terakhir halaman 1 → sisa 1 baris (Satu).
	// Inilah bukti keyset BISA menjangkau halaman berikutnya (bukan cuma LIMIT).
	last := p1[1]
	p2 := list(last.CreatedAt, last.ID, 2)
	if len(p2) != 1 || p2[0].VillageName != "Satu" {
		t.Fatalf("halaman 2 harus 1 baris (Satu), got %d %+v", len(p2), namesOf(p2))
	}
}

func TestListAccounts_OwnershipFlags(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	sales, _ := q.CreateUser(ctx, CreateUserParams{Email: "s@x", PassHash: strPtr("x")})
	csm, _ := q.CreateUser(ctx, CreateUserParams{Email: "c@x", PassHash: strPtr("x")})
	other, _ := q.CreateUser(ctx, CreateUserParams{Email: "o@x", PassHash: strPtr("x")})

	// Sales punya "Milik-Sales"; CSM membina "Binaan" & cadangan di "Cadangan";
	// "Asing" tak menyentuh keduanya.
	seedAccount(t, ctx, pool, ten.ID, "Milik-Sales", func(p *CreateAccountParams) { p.AccountOwner = &sales.ID })
	seedAccount(t, ctx, pool, ten.ID, "Binaan", func(p *CreateAccountParams) { p.AssignedCsm = &csm.ID })
	seedAccount(t, ctx, pool, ten.ID, "Cadangan", func(p *CreateAccountParams) { p.BackupCsm = &csm.ID })
	seedAccount(t, ctx, pool, ten.ID, "Asing", func(p *CreateAccountParams) { p.AccountOwner = &other.ID })

	listWith := func(f AccountsListFilter, uid int64) []Account {
		var rows []Account
		c0, id0 := firstCursor()
		if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
			var e error
			rows, e = q.ListAccounts(ctx, ListAccountsParams{
				CursorCreatedAt: c0, CursorID: id0,
				// IsOwn = union kepemilikan → kedua flag SQL true (account_owner OR
				// assigned_csm OR backup_csm = uid).
				ScopeAll: f.ScopeAll, IsSales: f.IsOwn, IsCsm: f.IsOwn,
				Uid:      &uid,
				PageSize: 50,
			})
			return e
		}); err != nil {
			t.Fatalf("list: %v", err)
		}
		return rows
	}

	// Admin/Manager (ScopeAll): 4 desa.
	if got := listWith(AccountsListFilter{ScopeAll: true}, sales.ID); len(got) != 4 {
		t.Errorf("scope_all harus 4 desa, got %d %v", len(got), namesOf(got))
	}
	// Cakupan 'own', aktor sales: union kepemilikan → hanya "Milik-Sales"
	// (account_owner-nya). Sales murni tak muncul di kolom CSM, jadi union = 1.
	if got := listWith(AccountsListFilter{IsOwn: true}, sales.ID); len(got) != 1 || got[0].VillageName != "Milik-Sales" {
		t.Errorf("own/sales harus 1 (Milik-Sales), got %v", namesOf(got))
	}
	// Cakupan 'own', aktor CSM: "Binaan" (assigned) + "Cadangan" (backup) = 2.
	if got := listWith(AccountsListFilter{IsOwn: true}, csm.ID); len(got) != 2 {
		t.Errorf("own/csm harus 2 (binaan+cadangan), got %v", namesOf(got))
	}
	// Semua flag false ('none'/role liar): NOL baris — fail-closed, bukan bocor.
	if got := listWith(AccountsListFilter{}, sales.ID); len(got) != 0 {
		t.Errorf("tanpa flag harus 0 baris (fail-closed), got %v", namesOf(got))
	}
}

// TestListAccounts_UnownedFilter: tab "Belum ada Owner". Filter unowned adalah
// tapis SEJAJAR di atas cakupan — false tak membatasi apa pun; true menyisakan
// HANYA desa tanpa account_owner. Diuji terpisah dari flag ownership karena ia
// mengunci sumbu berbeda (ada/tidaknya pemilik, bukan siapa pemiliknya).
func TestListAccounts_UnownedFilter(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	owner, _ := q.CreateUser(ctx, CreateUserParams{Email: "own@x", PassHash: strPtr("x")})

	// Dua ber-owner, satu tanpa owner.
	seedAccount(t, ctx, pool, ten.ID, "Ber-Owner-A", func(p *CreateAccountParams) { p.AccountOwner = &owner.ID })
	seedAccount(t, ctx, pool, ten.ID, "Ber-Owner-B", func(p *CreateAccountParams) { p.AccountOwner = &owner.ID })
	seedAccount(t, ctx, pool, ten.ID, "Tanpa-Owner", nil)

	list := func(unowned bool) []Account {
		var rows []Account
		c0, id0 := firstCursor()
		if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
			var e error
			rows, e = q.ListAccounts(ctx, ListAccountsParams{
				CursorCreatedAt: c0, CursorID: id0,
				ScopeAll: true, Unowned: unowned,
				PageSize: 50,
			})
			return e
		}); err != nil {
			t.Fatalf("list: %v", err)
		}
		return rows
	}

	// unowned=false → tak membatasi: ketiga desa tampil (bukti default tak menyaring).
	if got := list(false); len(got) != 3 {
		t.Errorf("unowned=false harus 3 desa, got %d %v", len(got), namesOf(got))
	}
	// unowned=true → hanya "Tanpa-Owner".
	if got := list(true); len(got) != 1 || got[0].VillageName != "Tanpa-Owner" {
		t.Errorf("unowned=true harus 1 (Tanpa-Owner), got %v", namesOf(got))
	}
}

// ── Unit (tanpa DB) — perakitan flag & keputusan per-baris ──────────────────

// TestAccountsListFilterFor: data_scope → flag list-query. Satu sumber dengan
// AccountsScopeFor, jadi diuji sejajar dengan pemetaan cakupan. Cakupan dibaca
// dari kolom (F3 editable), bukan ditebak dari nama role.
func TestAccountsListFilterFor(t *testing.T) {
	cases := []struct {
		scope string
		want  AccountsListFilter
	}{
		{authz.DataScopeAll, AccountsListFilter{ScopeAll: true}},
		{authz.DataScopeOwn, AccountsListFilter{IsOwn: true}},
		{authz.DataScopeNone, AccountsListFilter{}}, // nol baris
		{"", AccountsListFilter{}},
		{"galaxy", AccountsListFilter{}}, // liar → fail-closed
	}
	for _, c := range cases {
		if got := AccountsListFilterFor(c.scope); got != c.want {
			t.Errorf("AccountsListFilterFor(%q)=%+v, want %+v", c.scope, got, c.want)
		}
	}
}

// TestAccountsListFilter_Allows: keputusan per-baris (dipakai handler untuk detail
// GetAccount) harus konsisten dengan yang disaring ListAccounts.
func TestAccountsListFilter_Allows(t *testing.T) {
	const me, notMe = int64(7), int64(99)
	p := func(v int64) *int64 { return &v }

	// ScopeAll: apa pun kolomnya → boleh.
	if !(AccountsListFilter{ScopeAll: true}).Allows(me, nil, nil, nil) {
		t.Error("ScopeAll harus mengizinkan baris tanpa kepemilikan")
	}
	// Cakupan 'own' = union: boleh bila uid ada di SALAH SATU kolom kepemilikan
	// (owner ATAU assigned_csm ATAU backup_csm). Satu makna "desa yang
	// ditugaskan padaku", tanpa membedakan peran yang menautkannya.
	own := AccountsListFilter{IsOwn: true}
	if !own.Allows(me, p(me), nil, nil) {
		t.Error("own harus boleh lewat account_owner")
	}
	if !own.Allows(me, nil, p(me), nil) {
		t.Error("own harus boleh lewat assigned_csm")
	}
	if !own.Allows(me, nil, nil, p(me)) {
		t.Error("own harus boleh lewat backup_csm")
	}
	if own.Allows(me, p(notMe), p(notMe), p(notMe)) {
		t.Error("own tak boleh atas desa orang lain di semua kolom")
	}
	// Kosong ('none'): tak pernah boleh.
	if (AccountsListFilter{}).Allows(me, p(me), p(me), p(me)) {
		t.Error("filter kosong harus menolak semua (fail-closed)")
	}
}

// namesOf memetakan baris ke nama desa untuk pesan galat daftar yang terbaca.
func namesOf(rows []Account) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.VillageName
	}
	return out
}
