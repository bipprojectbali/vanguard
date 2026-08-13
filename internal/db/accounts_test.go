package db

import (
	"context"
	"math"
	"testing"

	"go_starter/internal/codes"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// accounts_test.go — bukti query HUB Desa (M2-2): roundtrip create→get, edit
// profil, penugasan CSM, dan soft-delete. Daftar & filter kepemilikan F3 dipisah
// ke accounts_list_test.go (paket sama). Yang dijaga di sini, bila rusak, tak
// terlihat dari perilaku aplikasi sampai data sudah bocor/hilang:
//
//   (1) Roundtrip create→get: entity_code (DESA-001) dialokasikan DI DALAM tx dan
//       ikut tersimpan; village_code dari user boleh kosong.
//   (2) Penugasan CSM & soft-delete = jalur TERPISAH dari edit profil — edit tak
//       boleh diam-diam mengubah owner/CSM, soft-delete idempotent & menyembunyikan.
//   (3) village_code unik per-tenant (unique partial index).
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

// TestFindDuplicateAccountsByNameRegion — kandidat duplikat desa (M4-6 follow-
// up, soft-warning saat konversi). Empat perilaku yang bila rusak bikin
// peringatan salah/bocor lintas-tenant:
//
//	(1) match case-insensitive + trim ("SUKAMAJU " ketemu "Sukamaju")
//	(2) regency kosong (NULL) → tak menyaring; regency terisi → ikut menyaring
//	(3) isolasi tenant: desa nama sama di tenant lain tak pernah muncul
//	(4) desa ter-soft-delete dikecualikan
func TestFindDuplicateAccountsByNameRegion(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	tenA, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "A", Slug: "a"})
	tenB, _ := q.CreateTenant(ctx, CreateTenantParams{Name: "B", Slug: "b"})

	regBandung := "Bandung"
	regBogor := "Bogor"
	seedAccount(t, ctx, pool, tenA.ID, "Sukamaju", func(p *CreateAccountParams) {
		p.Regency = &regBandung
	})
	seedAccount(t, ctx, pool, tenA.ID, "Sukamaju", func(p *CreateAccountParams) {
		p.Regency = &regBogor
	})
	deleted := seedAccount(t, ctx, pool, tenA.ID, "Sukamaju", nil)
	if e := WithTenant(ctx, pool, tenA.ID, func(q *Queries) error {
		return q.SoftDeleteAccount(ctx, SoftDeleteAccountParams{ID: deleted.ID})
	}); e != nil {
		t.Fatalf("soft-delete: %v", e)
	}
	seedAccount(t, ctx, pool, tenB.ID, "Sukamaju", nil) // tenant lain, harus TAK terlihat.

	// (1)+(2): nama beda kapitalisasi/spasi, tanpa filter regency → dua kandidat
	// hidup (Bandung & Bogor), soft-deleted TAK ikut (4).
	var rows []FindDuplicateAccountsByNameRegionRow
	if e := WithTenant(ctx, pool, tenA.ID, func(q *Queries) error {
		var e error
		rows, e = q.FindDuplicateAccountsByNameRegion(ctx, FindDuplicateAccountsByNameRegionParams{
			TenantID: tenA.ID, VillageName: "  SUKAMAJU  ",
		})
		return e
	}); e != nil {
		t.Fatalf("find (tanpa regency): %v", e)
	}
	if len(rows) != 2 {
		t.Fatalf("harus 2 kandidat hidup (soft-deleted dikecualikan), got %d", len(rows))
	}

	// (2) regency terisi → hanya menyaring ke Bandung.
	if e := WithTenant(ctx, pool, tenA.ID, func(q *Queries) error {
		var e error
		rows, e = q.FindDuplicateAccountsByNameRegion(ctx, FindDuplicateAccountsByNameRegionParams{
			TenantID: tenA.ID, VillageName: "Sukamaju", Regency: &regBandung,
		})
		return e
	}); e != nil {
		t.Fatalf("find (regency Bandung): %v", e)
	}
	if len(rows) != 1 || rows[0].Regency == nil || *rows[0].Regency != regBandung {
		t.Errorf("regency terisi harus menyaring ke Bandung saja, got %d baris", len(rows))
	}

	// (3) isolasi tenant: query di tenant B tak boleh melihat baris tenant A.
	if e := WithTenant(ctx, pool, tenB.ID, func(q *Queries) error {
		var e error
		rows, e = q.FindDuplicateAccountsByNameRegion(ctx, FindDuplicateAccountsByNameRegionParams{
			TenantID: tenB.ID, VillageName: "Sukamaju",
		})
		return e
	}); e != nil {
		t.Fatalf("find (tenant B): %v", e)
	}
	if len(rows) != 1 {
		t.Errorf("tenant B harus hanya lihat desanya sendiri (1 baris), got %d", len(rows))
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
