package db

import (
	"context"
	"math"
	"testing"

	"go_starter/internal/authz"
	"go_starter/internal/codes"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// accounts_test.go — bukti query HUB Desa (M2-2). Tiga lapis yang, bila rusak,
// tak terlihat dari perilaku aplikasi sampai data sudah bocor/hilang:
//
//   (1) Roundtrip create→get: entity_code (DESA-001) dialokasikan DI DALAM tx dan
//       ikut tersimpan; village_code dari user boleh kosong.
//   (2) ListAccounts = keyset + ownership F3 dalam satu query. Keyset harus bisa
//       menjangkau halaman kedua (bukan cuma LIMIT); ownership harus fail-closed
//       (flag semua false → nol baris, bukan bocor).
//   (3) Penugasan CSM & soft-delete = jalur TERPISAH dari edit profil — edit tak
//       boleh diam-diam mengubah owner/CSM, soft-delete idempotent & menyembunyikan.
//
// Seed lewat CreateAccount asli (jalur yang diuji), di dalam WithTenant (RLS +
// SET LOCAL ROLE app_rw aktif — persis jalur query aplikasi).

// firstCursor = cursor halaman pertama untuk keyset (created_at, id) < ($1,$2):
// +Infinity/maxint agar "lebih besar dari semua baris" → semua baris lolos.
func firstCursor() (pgtype.Timestamptz, int64) {
	return pgtype.Timestamptz{Valid: true, InfinityModifier: pgtype.Infinity}, math.MaxInt64
}

// seedAccount membuat satu desa lewat CreateAccount asli (dengan entity_code dari
// GenerateEntityCode) di dalam tx ber-tenant. Mengembalikan baris hasil.
func seedAccount(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID int64, name string, mut func(*CreateAccountParams)) Account {
	t.Helper()
	var out Account
	if err := WithTenant(ctx, pool, tenantID, func(q *Queries) error {
		code, err := q.GenerateEntityCode(ctx, tenantID, codes.EntityAccount)
		if err != nil {
			return err
		}
		p := CreateAccountParams{
			TenantID:    tenantID,
			EntityCode:  &code,
			VillageName: name,
			AccountType: "prospect",
		}
		if mut != nil {
			mut(&p)
		}
		out, err = q.CreateAccount(ctx, p)
		return err
	}); err != nil {
		t.Fatalf("seed account %q: %v", name, err)
	}
	return out
}

func TestCreateAndGetAccount(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, err := q.CreateTenant(ctx, CreateTenantParams{Name: "Desa+", Slug: "desaplus"})
	if err != nil {
		t.Fatalf("tenant: %v", err)
	}

	vc := "3201012001"
	acc := seedAccount(t, ctx, pool, ten.ID, "Sukamaju", func(p *CreateAccountParams) {
		p.VillageCode = &vc
	})

	if acc.EntityCode == nil || *acc.EntityCode != "DESA-001" {
		t.Errorf("entity_code pertama harus DESA-001, got %v", acc.EntityCode)
	}
	if acc.VillageCode == nil || *acc.VillageCode != vc {
		t.Errorf("village_code harus tersimpan apa adanya, got %v", acc.VillageCode)
	}

	// GetAccount mengambil baris hidup yang sama (RLS menjamin tenant).
	var got Account
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		got, e = q.GetAccount(ctx, acc.ID)
		return e
	}); err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ID != acc.ID || got.VillageName != "Sukamaju" {
		t.Errorf("get harus kembalikan desa yang sama, got id=%d name=%q", got.ID, got.VillageName)
	}
}

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

func TestUpdateAccount_TakSentuhKepemilikan(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	owner, _ := q.CreateUser(ctx, CreateUserParams{Email: "own@x", PassHash: strPtr("x")})

	vc := "3201012001"
	acc := seedAccount(t, ctx, pool, ten.ID, "Lama", func(p *CreateAccountParams) {
		p.AccountOwner = &owner.ID
		p.VillageCode = &vc
	})

	var upd Account
	newName := "Baru"
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		upd, e = q.UpdateAccount(ctx, UpdateAccountParams{
			ID:          acc.ID,
			VillageName: newName,
			AccountType: "customer",
		})
		return e
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if upd.VillageName != "Baru" || upd.AccountType != "customer" {
		t.Errorf("profil harus terupdate, got name=%q type=%q", upd.VillageName, upd.AccountType)
	}
	// entity_code, village_code, dan account_owner TAK boleh berubah lewat edit profil.
	if upd.EntityCode == nil || *upd.EntityCode != *acc.EntityCode {
		t.Errorf("entity_code tak boleh berubah saat edit profil, got %v", upd.EntityCode)
	}
	if upd.VillageCode == nil || *upd.VillageCode != vc {
		t.Errorf("village_code tak boleh berubah saat edit profil, got %v", upd.VillageCode)
	}
	if upd.AccountOwner == nil || *upd.AccountOwner != owner.ID {
		t.Errorf("account_owner tak boleh berubah saat edit profil, got %v", upd.AccountOwner)
	}
}

func TestAssignAccountCSM(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	csm, _ := q.CreateUser(ctx, CreateUserParams{Email: "c@x", PassHash: strPtr("x")})
	backup, _ := q.CreateUser(ctx, CreateUserParams{Email: "b@x", PassHash: strPtr("x")})
	acc := seedAccount(t, ctx, pool, ten.ID, "Desa", nil)

	// Tugaskan binaan + cadangan.
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.AssignAccountCSM(ctx, AssignAccountCSMParams{
			ID: acc.ID, AssignedCsm: &csm.ID, BackupCsm: &backup.ID,
		})
	}); err != nil {
		t.Fatalf("assign: %v", err)
	}
	got := getAcc(t, ctx, pool, ten.ID, acc.ID)
	if got.AssignedCsm == nil || *got.AssignedCsm != csm.ID {
		t.Errorf("assigned_csm harus terset, got %v", got.AssignedCsm)
	}
	if got.BackupCsm == nil || *got.BackupCsm != backup.ID {
		t.Errorf("backup_csm harus terset, got %v", got.BackupCsm)
	}

	// NULL = lepas penugasan.
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.AssignAccountCSM(ctx, AssignAccountCSMParams{ID: acc.ID})
	}); err != nil {
		t.Fatalf("unassign: %v", err)
	}
	got = getAcc(t, ctx, pool, ten.ID, acc.ID)
	if got.AssignedCsm != nil || got.BackupCsm != nil {
		t.Errorf("penugasan harus terlepas (NULL), got assigned=%v backup=%v", got.AssignedCsm, got.BackupCsm)
	}
}

func TestSoftDeleteAccount(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	acc := seedAccount(t, ctx, pool, ten.ID, "Desa", nil)

	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.SoftDeleteAccount(ctx, SoftDeleteAccountParams{ID: acc.ID})
	}); err != nil {
		t.Fatalf("soft-delete: %v", err)
	}

	// Get harus gagal (baris disembunyikan filter deleted_at IS NULL).
	err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, e := q.GetAccount(ctx, acc.ID)
		return e
	})
	if err == nil {
		t.Errorf("get atas desa ter-soft-delete harus gagal (tak ada baris hidup)")
	}

	// List juga tak memuatnya.
	c0, id0 := firstCursor()
	var rows []Account
	if e := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		rows, e = q.ListAccounts(ctx, ListAccountsParams{
			CursorCreatedAt: c0, CursorID: id0, ScopeAll: true, PageSize: 50,
		})
		return e
	}); e != nil {
		t.Fatalf("list: %v", e)
	}
	if len(rows) != 0 {
		t.Errorf("list tak boleh memuat desa ter-soft-delete, got %d", len(rows))
	}

	// Idempotent: soft-delete kedua tak error (hanya menyentuh baris hidup).
	if e := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		return q.SoftDeleteAccount(ctx, SoftDeleteAccountParams{ID: acc.ID})
	}); e != nil {
		t.Errorf("soft-delete kedua harus idempotent, got %v", e)
	}
}

func TestCreateAccount_VillageCodeUniquePerTenant(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "T", Slug: "t"})
	vc := "3201012001"
	seedAccount(t, ctx, pool, ten.ID, "Pertama", func(p *CreateAccountParams) { p.VillageCode = &vc })

	// village_code duplikat di tenant sama → ditolak idx_accounts_code (unique partial).
	err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		code, e := q.GenerateEntityCode(ctx, ten.ID, codes.EntityAccount)
		if e != nil {
			return e
		}
		_, e = q.CreateAccount(ctx, CreateAccountParams{
			TenantID: ten.ID, EntityCode: &code, VillageName: "Kedua",
			AccountType: "prospect", VillageCode: &vc,
		})
		return e
	})
	if err == nil {
		t.Errorf("village_code duplikat di tenant sama harus ditolak unique index")
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

// getAcc = GetAccount ringkas untuk test.
func getAcc(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, id int64) Account {
	t.Helper()
	var got Account
	if err := WithTenant(ctx, pool, tenantID, func(q *Queries) error {
		var e error
		got, e = q.GetAccount(ctx, id)
		return e
	}); err != nil {
		t.Fatalf("get %d: %v", id, err)
	}
	return got
}

func namesOf(rows []Account) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.VillageName
	}
	return out
}
