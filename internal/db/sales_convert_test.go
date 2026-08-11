package db

import (
	"context"
	"testing"

	"go_starter/internal/codes"

	"github.com/jackc/pgx/v5/pgxpool"
)

// sales_convert_test.go — INTI Modul 4 di sisi query: konversi Lead Qualified →
// Desa (Account) + Kontak (Contact) + Deal HARUS all-or-nothing. Handler
// LeadConvert menyandarkan atomicity sepenuhnya pada SATU tx ber-tenant (Scope
// middleware): tak ada savepoint, tak ada kompensasi manual. Postgres membatalkan
// SELURUH tx pada galat query apa pun, jadi bila langkah ke-N gagal, langkah 1..N-1
// yang sudah "sukses" ikut lenyap.
//
// Uji itu di batas SEBENARNYA — `WithTenant` (rollback saat fn kembalikan error,
// commit saat nil) — bukan di pool-bound Queries yang autocommit tiap statement
// (di sana rollback tak akan pernah terlihat, dan test membuktikan hal yang salah).
//
// Catatan cakupan: konversi selalu MEMBUAT desa baru & tak membawa village_code,
// jadi ia tak bisa bertabrakan duplikat — keunikan desa (village_code) adalah
// urusan lapisan Account, sudah dijaga TestAccounts_VillageCodeDuplicate.

// convertSeq menjalankan urutan konversi PERSIS seperti handler LeadConvert di
// dalam satu *Queries (satu tx). badStage=true menyuntik nilai stage liar di
// langkah TERAKHIR (deal) untuk memicu galat CHECK SETELAH account+contact sukses
// — cara paling deterministik membuktikan langkah awal ikut ter-rollback.
func convertSeq(ctx context.Context, q *Queries, tenantID, leadID int64, badStage bool) (Account, Contact, Deal, error) {
	var acc Account
	var contact Contact
	var deal Deal

	accCode, err := q.GenerateEntityCode(ctx, tenantID, codes.EntityAccount)
	if err != nil {
		return acc, contact, deal, err
	}
	acc, err = q.CreateAccount(ctx, CreateAccountParams{
		TenantID:    tenantID,
		EntityCode:  &accCode,
		VillageName: "Desa Konversi",
		AccountType: "prospect",
	})
	if err != nil {
		return acc, contact, deal, err
	}

	first := "Budi"
	contact, err = q.CreateContact(ctx, CreateContactParams{
		TenantID:         tenantID,
		AccountID:        acc.ID,
		FirstName:        first,
		IsPrimaryContact: true,
	})
	if err != nil {
		return acc, contact, deal, err
	}

	dealCode, err := q.GenerateEntityCode(ctx, tenantID, codes.EntityDeal)
	if err != nil {
		return acc, contact, deal, err
	}
	stage := "Prospecting"
	if badStage {
		stage = "TahapLiar" // di luar enum → CHECK stage gagal
	}
	deal, err = q.CreateDeal(ctx, CreateDealParams{
		TenantID:         tenantID,
		EntityCode:       &dealCode,
		DealName:         "Deal Konversi",
		AccountID:        acc.ID,
		PrimaryContactID: &contact.ID,
		Stage:            stage,
	})
	if err != nil {
		return acc, contact, deal, err
	}

	err = q.MarkLeadConverted(ctx, MarkLeadConvertedParams{
		ConvertedAccountID: &acc.ID,
		ConvertedContactID: &contact.ID,
		ConvertedDealID:    &deal.ID,
		ID:                 leadID,
	})
	return acc, contact, deal, err
}

// countAccounts / countDeals menghitung baris hidup lewat query list scope-all di
// dalam WithTenant — RLS FORCE menolak SELECT tanpa GUC tenant, jadi menghitung
// harus lewat jalur ber-tenant yang sama seperti aplikasi.
func countAccounts(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID int64) int {
	t.Helper()
	c0, id0 := firstCursor()
	var rows []Account
	if err := WithTenant(ctx, pool, tenantID, func(q *Queries) error {
		var e error
		rows, e = q.ListAccounts(ctx, ListAccountsParams{
			CursorCreatedAt: c0, CursorID: id0, ScopeAll: true, PageSize: 100,
		})
		return e
	}); err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	return len(rows)
}

func countDeals(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID int64) int {
	t.Helper()
	c0, id0 := firstCursor()
	var rows []Deal
	if err := WithTenant(ctx, pool, tenantID, func(q *Queries) error {
		var e error
		rows, e = q.ListDeals(ctx, ListDealsParams{
			CursorCreatedAt: c0, CursorID: id0, ScopeAll: true, PageSize: 100,
		})
		return e
	}); err != nil {
		t.Fatalf("list deals: %v", err)
	}
	return len(rows)
}

func getLeadTx(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tenantID, id int64) Lead {
	t.Helper()
	var got Lead
	if err := WithTenant(ctx, pool, tenantID, func(q *Queries) error {
		var e error
		got, e = q.GetLead(ctx, id)
		return e
	}); err != nil {
		t.Fatalf("get lead: %v", err)
	}
	return got
}

// TestConvertLead_AtomikRollbackPenuh: langkah deal gagal (stage liar) → SELURUH
// konversi batal. Desa + kontak yang sudah "sukses" ikut lenyap, dan lead TETAP
// belum converted (mark tak pernah tercapai). Gagal-sebagian = rollback penuh.
func TestConvertLead_AtomikRollbackPenuh(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, err := q.CreateTenant(ctx, CreateTenantParams{Name: "Desa+", Slug: "desaplus"})
	if err != nil {
		t.Fatalf("tenant: %v", err)
	}
	lead := seedLead(t, ctx, pool, ten.ID, "Sukamaju", func(p *CreateLeadParams) {
		p.LeadStatus = "Qualified"
	})

	// Urutan dengan stage liar di langkah terakhir → CHECK gagal → error → rollback.
	convErr := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, _, _, e := convertSeq(ctx, q, ten.ID, lead.ID, true)
		return e
	})
	if convErr == nil {
		t.Fatal("konversi dengan stage liar harus gagal, bukan sukses senyap")
	}

	// Semua sisi-efek batal.
	if n := countAccounts(t, ctx, pool, ten.ID); n != 0 {
		t.Errorf("rollback: desa harus 0, ada %d (account bocor dari tx yang batal)", n)
	}
	if n := countDeals(t, ctx, pool, ten.ID); n != 0 {
		t.Errorf("rollback: deal harus 0, ada %d", n)
	}
	got := getLeadTx(t, ctx, pool, ten.ID, lead.ID)
	if got.Converted {
		t.Error("rollback: lead harus TETAP belum converted")
	}
	if got.ConvertedAccountID != nil || got.ConvertedDealID != nil {
		t.Errorf("rollback: tautan converted_* harus kosong, got acc=%v deal=%v",
			got.ConvertedAccountID, got.ConvertedDealID)
	}
}

// TestConvertLead_AtomikCommitPenuh: urutan sah → commit. Ketiga entitas ada &
// lead ditandai converted dengan tautan menunjuk baris yang baru dibuat (bukti
// mark ikut dalam tx yang sama, bukan langkah terpisah yang bisa menggantung).
func TestConvertLead_AtomikCommitPenuh(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, err := q.CreateTenant(ctx, CreateTenantParams{Name: "Desa+", Slug: "desaplus"})
	if err != nil {
		t.Fatalf("tenant: %v", err)
	}
	lead := seedLead(t, ctx, pool, ten.ID, "Sukamaju", func(p *CreateLeadParams) {
		p.LeadStatus = "Qualified"
	})

	var acc Account
	var contact Contact
	var deal Deal
	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		var e error
		acc, contact, deal, e = convertSeq(ctx, q, ten.ID, lead.ID, false)
		return e
	}); err != nil {
		t.Fatalf("konversi sah harus commit, got err: %v", err)
	}

	if n := countAccounts(t, ctx, pool, ten.ID); n != 1 {
		t.Errorf("commit: harus 1 desa, ada %d", n)
	}
	if n := countDeals(t, ctx, pool, ten.ID); n != 1 {
		t.Errorf("commit: harus 1 deal, ada %d", n)
	}

	got := getLeadTx(t, ctx, pool, ten.ID, lead.ID)
	if !got.Converted {
		t.Fatal("commit: lead harus ditandai converted")
	}
	if got.ConvertedAccountID == nil || *got.ConvertedAccountID != acc.ID {
		t.Errorf("commit: converted_account_id harus tunjuk desa baru %d, got %v", acc.ID, got.ConvertedAccountID)
	}
	if got.ConvertedContactID == nil || *got.ConvertedContactID != contact.ID {
		t.Errorf("commit: converted_contact_id harus tunjuk kontak baru %d, got %v", contact.ID, got.ConvertedContactID)
	}
	if got.ConvertedDealID == nil || *got.ConvertedDealID != deal.ID {
		t.Errorf("commit: converted_deal_id harus tunjuk deal baru %d, got %v", deal.ID, got.ConvertedDealID)
	}
}

// TestConvertLead_MarkGuardHanyaQualified: MarkLeadConverted menjaga di WHERE —
// lead non-Qualified tak tersentuh walau urutan lain sukses. Ini yang membuat
// guard handler (cek status sebelum tx) berlapis, bukan satu-satunya pertahanan:
// bila lead bukan Qualified, mark meng-update 0 baris dan lead tak pernah tertaut
// ke desa/deal yang terlanjur dibuat.
func TestConvertLead_MarkGuardHanyaQualified(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	truncateCRM(t, ctx)

	q := New(pool)
	ten, err := q.CreateTenant(ctx, CreateTenantParams{Name: "Desa+", Slug: "desaplus"})
	if err != nil {
		t.Fatalf("tenant: %v", err)
	}
	// Lead masih 'New' (belum Qualified) — mark harus no-op.
	lead := seedLead(t, ctx, pool, ten.ID, "BelumLayak", nil)

	if err := WithTenant(ctx, pool, ten.ID, func(q *Queries) error {
		_, _, _, e := convertSeq(ctx, q, ten.ID, lead.ID, false)
		return e
	}); err != nil {
		t.Fatalf("urutan tak boleh galat (mark hanya no-op): %v", err)
	}

	got := getLeadTx(t, ctx, pool, ten.ID, lead.ID)
	if got.Converted {
		t.Error("lead non-Qualified tak boleh ditandai converted oleh mark")
	}
}
