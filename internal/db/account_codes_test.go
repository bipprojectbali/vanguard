package db

import (
	"context"
	"testing"
)

// account_codes_test.go — bukti query pendukung ALOKASI dua kode desa otomatis
// (dipakai handler generateVillageCode/allocEntityCode). Yang dijaga, bila rusak,
// tak terlihat sampai dua desa bertabrakan kode atau nomor dipakai ulang:
//
//   (1) MaxVillageSeqForPrefix: nomor urut TERTINGGI per (tenant, prefix Kecamatan)
//       dihitung atas SEMUA baris — termasuk soft-deleted (nomor tak dipakai ulang)
//       — mengabaikan village_code lama tak-berformat, dan TAK menghitung lintas
//       prefix Kecamatan lain.
//   (2) VillageCodeExists / AccountEntityCodeExists: keberadaan kode persis per
//       tenant (semua baris) — dasar pola cek-sebelum-INSERT yang tak meracuni tx.

// softDeleteAccountByID mem-soft-delete satu desa di dalam tx ber-tenant (RLS
// aktif) — dipakai membuktikan query kode menghitung baris soft-deleted juga.
func softDeleteAccountByID(t *testing.T, ctx context.Context, tenantID, id int64) {
	t.Helper()
	if err := WithTenant(ctx, pkgPool, tenantID, func(q *Queries) error {
		return q.SoftDeleteAccount(ctx, SoftDeleteAccountParams{ID: id})
	}); err != nil {
		t.Fatalf("soft-delete account %d: %v", id, err)
	}
}

// seedVillage membuat satu desa dgn village_code tertentu (entity_code
// auto lewat seedAccount) dan mengembalikan id-nya.
func seedVillage(t *testing.T, ctx context.Context, tenantID int64, name, villageCode string) int64 {
	t.Helper()
	vc := villageCode
	a := seedAccount(t, ctx, pkgPool, tenantID, name, func(p *CreateAccountParams) {
		p.VillageCode = &vc
	})
	return a.ID
}

func maxVillageSeq(t *testing.T, ctx context.Context, tenantID int64, prefix string) int {
	t.Helper()
	p := prefix + "%"
	var got int32
	if err := WithTenant(ctx, pkgPool, tenantID, func(q *Queries) error {
		var e error
		got, e = q.MaxVillageSeqForPrefix(ctx, MaxVillageSeqForPrefixParams{TenantID: tenantID, Prefix: &p})
		return e
	}); err != nil {
		t.Fatalf("max village seq (%q): %v", prefix, err)
	}
	return int(got)
}

// TestMaxVillageSeqForPrefix: membangun keadaan bertahap dan menegakkan tiap sifat
// yang jadi dasar pemilihan nomor urut village_code otomatis berikutnya.
func TestMaxVillageSeqForPrefix(t *testing.T) {
	ctx := context.Background()
	truncateCRM(t, ctx)
	q := New(pkgPool)
	ten, err := q.CreateTenant(ctx, CreateTenantParams{Name: "Seq", Slug: "seq"})
	if err != nil {
		t.Fatalf("tenant: %v", err)
	}
	const prefix = "11.01.01."

	// Belum ada baris → 0 (COALESCE), jadi pemanggil mulai dari 0+1 = 0001.
	if got := maxVillageSeq(t, ctx, ten.ID, prefix); got != 0 {
		t.Errorf("kosong harus 0, got %d", got)
	}

	// Dua baris konform → MAX segmen ke-4.
	seedVillage(t, ctx, ten.ID, "Desa 1", prefix+"0001")
	seedVillage(t, ctx, ten.ID, "Desa 5", prefix+"0005")
	if got := maxVillageSeq(t, ctx, ten.ID, prefix); got != 5 {
		t.Errorf("max harus 5, got %d", got)
	}

	// Baris nomor lebih tinggi lalu di-soft-delete → TETAP dihitung (nomor desa
	// yang pernah ada tak boleh dipakai ulang).
	id7 := seedVillage(t, ctx, ten.ID, "Desa 7", prefix+"0007")
	softDeleteAccountByID(t, ctx, ten.ID, id7)
	if got := maxVillageSeq(t, ctx, ten.ID, prefix); got != 7 {
		t.Errorf("soft-deleted harus tetap dihitung (max 7), got %d", got)
	}

	// village_code lama tak-berformat (segmen ke-4 bukan angka) → DIABAIKAN guard
	// regex, cast ::int tak meledak. Max tetap 7.
	seedVillage(t, ctx, ten.ID, "Desa Legacy", prefix+"abc")
	if got := maxVillageSeq(t, ctx, ten.ID, prefix); got != 7 {
		t.Errorf("village_code tak-berformat harus diabaikan (max 7), got %d", got)
	}

	// Prefix Kecamatan LAIN dgn nomor lebih tinggi → TAK ikut terhitung.
	seedVillage(t, ctx, ten.ID, "Desa Lain", "12.01.01.0009")
	if got := maxVillageSeq(t, ctx, ten.ID, prefix); got != 7 {
		t.Errorf("prefix Kecamatan lain tak boleh dihitung (max 7), got %d", got)
	}
	if got := maxVillageSeq(t, ctx, ten.ID, "12.01.01."); got != 9 {
		t.Errorf("prefix lain harus 9, got %d", got)
	}
}

// TestVillageCodeExists: keberadaan village_code persis per tenant, atas SEMUA
// baris (termasuk soft-deleted) dan terisolasi antar-tenant.
func TestVillageCodeExists(t *testing.T) {
	ctx := context.Background()
	truncateCRM(t, ctx)
	q := New(pkgPool)
	ta, err := q.CreateTenant(ctx, CreateTenantParams{Name: "A", Slug: "a"})
	if err != nil {
		t.Fatalf("tenant A: %v", err)
	}
	tb, err := q.CreateTenant(ctx, CreateTenantParams{Name: "B", Slug: "b"})
	if err != nil {
		t.Fatalf("tenant B: %v", err)
	}
	const vc = "11.01.01.0001"

	exists := func(tenantID int64, code string) bool {
		t.Helper()
		var out bool
		c := code
		if err := WithTenant(ctx, pkgPool, tenantID, func(q *Queries) error {
			var e error
			out, e = q.VillageCodeExists(ctx, VillageCodeExistsParams{TenantID: tenantID, VillageCode: &c})
			return e
		}); err != nil {
			t.Fatalf("village code exists: %v", err)
		}
		return out
	}

	if exists(ta.ID, vc) {
		t.Error("village_code belum diseed harus tak ada")
	}
	id := seedVillage(t, ctx, ta.ID, "Desa A", vc)
	if !exists(ta.ID, vc) {
		t.Error("village_code terseed harus ada")
	}
	// Tenant lain dgn kode sama → tak terlihat (unik PER tenant).
	if exists(tb.ID, vc) {
		t.Error("village_code milik tenant A tak boleh terlihat dari tenant B")
	}
	// Soft-delete → tetap dianggap ADA (nomor tak dipakai ulang).
	softDeleteAccountByID(t, ctx, ta.ID, id)
	if !exists(ta.ID, vc) {
		t.Error("village_code soft-deleted harus tetap dianggap ada")
	}
}

// TestAccountEntityCodeExists: keberadaan entity_code (kode sistem) persis per
// tenant — dasar allocEntityCode melewati slot yang direbut override manual.
func TestAccountEntityCodeExists(t *testing.T) {
	ctx := context.Background()
	truncateCRM(t, ctx)
	q := New(pkgPool)
	ta, err := q.CreateTenant(ctx, CreateTenantParams{Name: "A", Slug: "a"})
	if err != nil {
		t.Fatalf("tenant A: %v", err)
	}
	tb, err := q.CreateTenant(ctx, CreateTenantParams{Name: "B", Slug: "b"})
	if err != nil {
		t.Fatalf("tenant B: %v", err)
	}

	exists := func(tenantID int64, code string) bool {
		t.Helper()
		var out bool
		c := code
		if err := WithTenant(ctx, pkgPool, tenantID, func(q *Queries) error {
			var e error
			out, e = q.AccountEntityCodeExists(ctx, AccountEntityCodeExistsParams{TenantID: tenantID, EntityCode: &c})
			return e
		}); err != nil {
			t.Fatalf("entity code exists: %v", err)
		}
		return out
	}

	// seedAccount mengalokasi entity_code otomatis pertama = DESA-001.
	if exists(ta.ID, "DESA-001") {
		t.Error("entity_code belum diseed harus tak ada")
	}
	seedAccount(t, ctx, pkgPool, ta.ID, "Desa A", nil)
	if !exists(ta.ID, "DESA-001") {
		t.Error("entity_code terseed (DESA-001) harus ada")
	}
	if exists(tb.ID, "DESA-001") {
		t.Error("entity_code milik tenant A tak boleh terlihat dari tenant B")
	}
}
