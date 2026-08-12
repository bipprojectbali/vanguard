package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// accounts_crud_test.go — jalur CRUD happy-path Desa (create/update/soft-delete)
// plus validasi backend & keunikan village_code. Sumbu izin (F2/F3/F4) & keyset
// diuji di accounts_test.go / accounts_fls_test.go; helper bersama di sana.

// TestAccounts_CreateSuccess: create sebagai sales → 303 ke detail dengan
// ok=created, baris tersimpan dengan pembuat sebagai owner, entity_code terisi,
// audit tercatat.
func TestAccounts_CreateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	form := accountFormValues("Desa Sukamaju", "prospect")
	form.Set("village_code", "3201012001")
	form.Set("province", "Jawa Barat")
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
	if a.VillageName != "Desa Sukamaju" {
		t.Errorf("nama = %q", a.VillageName)
	}
	if a.AccountOwner == nil || *a.AccountOwner != uid {
		t.Errorf("pembuat harus jadi owner awal (F3), got %v", a.AccountOwner)
	}
	if a.EntityCode == nil || *a.EntityCode == "" {
		t.Error("entity_code harus dialokasikan saat create")
	}
	env.assertAudited(t, "account.create")
}

// TestAccounts_CreateRejectsInvalid: input yang melanggar validasi backend
// (bukan cuma atribut form) ditolak → redirect err + tak menyentuh DB.
func TestAccounts_CreateRejectsInvalid(t *testing.T) {
	cases := []struct {
		name    string
		form    url.Values
		wantErr string
	}{
		{"nama kosong", accountFormValues("", "prospect"), "err=village_name"},
		{"tipe asing", accountFormValues("Desa X", "bukan_tipe"), "err=account_type"},
		{"penduduk negatif", withField(accountFormValues("Desa X", "prospect"), "population", "-5"), "err=number"},
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

// TestAccounts_VillageCodeDuplicate: village_code UNIQUE per tenant — tabrakan
// dikenali sebagai galat spesifik (village_code_dup), bukan "internal error".
func TestAccounts_VillageCodeDuplicate(t *testing.T) {
	env, uid := setupAccounts(t)
	first := accountFormValues("Desa Satu", "prospect")
	first.Set("village_code", "3201012001")
	req1 := accountsReq(http.MethodPost, "/w/test/accounts", first, "")
	if rec := env.runAccount(uid, "owner", "sales", req1, env.h.AccountCreate); rec.Code != http.StatusSeeOther {
		t.Fatalf("create pertama gagal: %d\n%s", rec.Code, rec.Body.String())
	}

	dup := accountFormValues("Desa Dua", "prospect")
	dup.Set("village_code", "3201012001") // sama
	req2 := accountsReq(http.MethodPost, "/w/test/accounts", dup, "")
	rec := env.runAccount(uid, "owner", "sales", req2, env.h.AccountCreate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=village_code_dup") {
		t.Errorf("tabrakan village_code harus err=village_code_dup, got %q", loc)
	}
}

// TestAccounts_UpdateSuccess: update sebagai owner (business admin) → tersimpan,
// redirect ok=saved, audit tercatat.
func TestAccounts_UpdateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Lama", &uid, nil, nil)

	form := accountFormValues("Desa Baru", "customer")
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID), form, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Errorf("harus ok=saved, got %q (status %d)", loc, rec.Code)
	}
	got, _ := env.q.GetAccount(t.Context(), a.ID)
	if got.VillageName != "Desa Baru" || got.AccountType != "customer" {
		t.Errorf("update tak tersimpan: %q / %q", got.VillageName, got.AccountType)
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
