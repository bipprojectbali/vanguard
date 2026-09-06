package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// accounts_crud_test.go — jalur CRUD happy-path Desa (create/update/soft-delete)
// plus validasi backend. BL-66: create memilih Desa master (village_id level 4);
// village_code Kemendagri + nama diturunkan dari master, entity_code sistem
// otomatis. Tabrakan kode & Desa-wajib diuji di accounts_codes_test.go; sumbu
// izin (F2/F3/F4) & keyset di accounts_test.go / accounts_fls_test.go; helper
// bersama (villages/firstVillage/withVillage) di sana.

// TestAccounts_CreateSuccess: create sebagai sales → 303 ke detail dengan
// ok=created, baris tersimpan dengan pembuat sebagai owner, entity_code +
// village_code + nama diturunkan dari master Desa, audit tercatat.
func TestAccounts_CreateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)
	// Desa dipilih via dropdown (village_id); nama+village_code TAK diketik —
	// diturunkan handler dari master.
	form := withVillage(accountFormValues("prospect"), v)
	form.Set("contact_phone", "0812-1111-2222")
	req := accountsReq(http.MethodPost, "/w/test/accounts", form, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountCreate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Errorf("redirect harus ok=created, got %q", loc)
	}
	rows := env.allAccounts(t)
	if len(rows) != 1 {
		t.Fatalf("harus 1 baris, ada %d", len(rows))
	}
	a := rows[0]
	if a.VillageName != v.Name {
		t.Errorf("nama = %q, want %q (nama master Desa)", a.VillageName, v.Name)
	}
	if a.AccountOwner == nil || *a.AccountOwner != uid {
		t.Errorf("pembuat harus jadi owner awal (F3), got %v", a.AccountOwner)
	}
	if a.EntityCode == nil || *a.EntityCode == "" {
		t.Error("entity_code harus dialokasikan saat create")
	}
	if got := deref(a.VillageCode); got != v.Code {
		t.Errorf("village_code = %q, want %q (kode Kemendagri master)", got, v.Code)
	}
	if a.DistrictID == nil || *a.DistrictID != v.DistrictID {
		t.Errorf("district_id harus mengikuti Kecamatan induk master, got %v want %d", a.DistrictID, v.DistrictID)
	}
	env.assertAudited(t, "account.create")
}

// TestAccounts_CreateRejectsUnknownVillage: village_id yang tak ada / bukan
// level 4 di master regions ditolak dengan pesan spesifik ("village_id"),
// bukan 500 mentah — AccountCreate memanggil GetVillageRegion (level=4) yang
// balikin ErrNoRows utk id tak valid.
func TestAccounts_CreateRejectsUnknownVillage(t *testing.T) {
	env, uid := setupAccounts(t)
	form := accountFormValues("prospect")
	form.Set("village_id", "999999999") // tak pernah ada di seed regions.
	req := accountsReq(http.MethodPost, "/w/test/accounts", form, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountCreate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=village_id") {
		t.Errorf("redirect harus err=village_id, got %q", loc)
	}
	if rows := env.allAccounts(t); len(rows) != 0 {
		t.Errorf("Desa tak dikenal harus membatalkan create, ada %d baris", len(rows))
	}
}

// firstDistrictID mengambil satu id Kecamatan (level 3) dari master regions
// yang di-seed migrasi 00026 — dipakai test leads yang masih 3-level (village_id
// hanya di Accounts, BL-66) tanpa menghardcode ID yang bisa berubah bila urutan
// seed migration berubah.
func firstDistrictID(t *testing.T, env *testEnv) int64 {
	t.Helper()
	rows, err := env.q.ListAllRegions(t.Context())
	if err != nil {
		t.Fatalf("list regions: %v", err)
	}
	for _, r := range rows {
		if r.Level == 3 {
			return r.ID
		}
	}
	t.Fatal("master regions kosong — migrasi 00026 belum ter-seed")
	return 0
}

// TestAccounts_CreateRejectsInvalid: input yang melanggar validasi backend
// (bukan cuma atribut form) ditolak → redirect err + tak menyentuh DB. Kasus di
// sini gagal SEBELUM village_id diperiksa (urutan parse: account_type/angka
// dulu), jadi tak perlu village_id.
func TestAccounts_CreateRejectsInvalid(t *testing.T) {
	cases := []struct {
		name    string
		form    url.Values
		wantErr string
	}{
		{"tipe asing", accountFormValues("bukan_tipe"), "err=account_type"},
		{"penduduk negatif", withField(accountFormValues("prospect"), "population", "-5"), "err=number"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, uid := setupAccounts(t)
			req := accountsReq(http.MethodPost, "/w/test/accounts", c.form, "")
			rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountCreate)
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, c.wantErr) {
				t.Errorf("harus %q, got %q", c.wantErr, loc)
			}
			if rows := env.allAccounts(t); len(rows) != 0 {
				t.Errorf("input invalid tak boleh menyimpan, ada %d baris", len(rows))
			}
		})
	}
}

// TestAccounts_UpdateSuccess: update sebagai owner (business admin) memilih Desa
// master baru (village_id) → nama+village_code diturunkan dari master, tipe
// tersimpan, redirect ok=saved, audit tercatat.
func TestAccounts_UpdateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Lama", &uid, nil, nil)
	v := firstVillage(t, env)

	form := withVillage(accountFormValues("customer"), v)
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID), form, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Errorf("harus ok=saved, got %q (status %d)", loc, rec.Code)
	}
	got, _ := env.q.GetAccount(t.Context(), a.ID)
	if got.VillageName != v.Name || got.AccountType != "customer" {
		t.Errorf("update tak tersimpan: %q / %q", got.VillageName, got.AccountType)
	}
	if code := deref(got.VillageCode); code != v.Code {
		t.Errorf("village_code harus diturunkan dari master, got %q want %q", code, v.Code)
	}
	env.assertAudited(t, "account.update")
}

// TestAccounts_SoftDelete: delete → baris hilang dari GetAccount (deleted_at),
// redirect ke daftar dengan ok=deleted, audit tercatat.
func TestAccounts_SoftDelete(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Hapus", &uid, nil, nil)

	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID)+"/delete", url.Values{}, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountDelete)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=deleted") {
		t.Errorf("harus ok=deleted, got %q (status %d)", loc, rec.Code)
	}
	if _, err := env.q.GetAccount(t.Context(), a.ID); err == nil {
		t.Error("baris ter-soft-delete tak boleh lagi terbaca GetAccount")
	}
	env.assertAudited(t, "account.delete")
}
