package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// accounts_codes_test.go — bukti alokasi kode saat create lewat AccountCreate
// (jalur end-to-end handler). BL-66: village_code = kode Kemendagri ASLI dari
// master Desa terpilih (regions level 4), BUKAN deret nomor sistem; satu Desa =
// satu account (idx_accounts_code). entity_code sistem tetap otomatis-by-default
// tapi bisa ditimpa manual. Query pendukungnya diuji terpisah di paket db
// (regions_test.go, account_codes_test.go).

// createAccountForm menjalankan AccountCreate dgn field yang diberi dan
// mengembalikan recorder-nya.
func createAccountForm(t *testing.T, env *testEnv, uid int64, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := accountsReq(http.MethodPost, "/w/test/accounts", form, "")
	return env.runAccount(uid, "owner", "sales", req, env.h.AccountCreate)
}

// accountByVillageCode mencari account terseed berdasarkan village_code (unik
// per tenant, idx_accounts_code) — pengganti lookup-by-nama yang tak lagi andal
// sejak nama account = nama master Desa (bisa berulang antar Kecamatan).
func accountByVillageCode(t *testing.T, env *testEnv, code string) db.Account {
	t.Helper()
	for _, a := range env.allAccounts(t) {
		if a.VillageCode != nil && *a.VillageCode == code {
			return a
		}
	}
	t.Fatalf("account dgn village_code %q tak ditemukan setelah create", code)
	return db.Account{}
}

// TestAccountCreate_VillageCodeFromMaster: village_code + nama account diturunkan
// APA ADANYA dari master Desa terpilih (kode Kemendagri asli), bukan dirakit
// ulang oleh handler.
func TestAccountCreate_VillageCodeFromMaster(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)

	f := withVillage(accountFormValues("prospect"), v)
	if rec := createAccountForm(t, env, uid, f); rec.Code != http.StatusSeeOther {
		t.Fatalf("create gagal: %d\n%s", rec.Code, rec.Body.String())
	}

	acc := accountByVillageCode(t, env, v.Code)
	if acc.VillageName != v.Name {
		t.Errorf("village_name = %q, want %q (nama master Desa)", acc.VillageName, v.Name)
	}
	if got := deref(acc.VillageCode); got != v.Code {
		t.Errorf("village_code = %q, want %q (kode Kemendagri master)", got, v.Code)
	}
	if acc.DistrictID == nil || *acc.DistrictID != v.DistrictID {
		t.Errorf("district_id = %v, want %d (Kecamatan induk master)", acc.DistrictID, v.DistrictID)
	}
}

// TestAccountCreate_DuplicateVillageRejected: Desa yang sama dipilih dua kali →
// create kedua ditolak spesifik (err=village_code_dup, bukan "internal error")
// dan hanya SATU baris tersimpan (satu Desa = satu account).
func TestAccountCreate_DuplicateVillageRejected(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)

	if rec := createAccountForm(t, env, uid, withVillage(accountFormValues("prospect"), v)); rec.Code != http.StatusSeeOther {
		t.Fatalf("create pertama gagal: %d\n%s", rec.Code, rec.Body.String())
	}
	rec := createAccountForm(t, env, uid, withVillage(accountFormValues("customer"), v))
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=village_code_dup") {
		t.Errorf("Desa duplikat harus err=village_code_dup, got %q", loc)
	}
	if rows := env.allAccounts(t); len(rows) != 1 {
		t.Errorf("satu Desa = satu account — harus tetap 1 baris, ada %d", len(rows))
	}
}

// TestAccountCreate_VillageRequired: tanpa village_id → redirect
// err=village_required dan TAK ada baris tersimpan (nama+village_code tak bisa
// diturunkan tanpa Desa terpilih).
func TestAccountCreate_VillageRequired(t *testing.T) {
	env, uid := setupAccounts(t)
	f := accountFormValues("prospect")
	rec := createAccountForm(t, env, uid, f)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=village_required") {
		t.Errorf("harus err=village_required, got %q", loc)
	}
	if rows := env.allAccounts(t); len(rows) != 0 {
		t.Errorf("Desa wajib — tak boleh menyimpan baris, ada %d", len(rows))
	}
}

// TestAccountCreate_EntityCodeAutoDefault: field kode sistem dikosongkan → dibuat
// otomatis (DESA-001 di format default).
func TestAccountCreate_EntityCodeAutoDefault(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)

	f := withVillage(accountFormValues("prospect"), v)
	if rec := createAccountForm(t, env, uid, f); rec.Code != http.StatusSeeOther {
		t.Fatalf("create gagal: %d\n%s", rec.Code, rec.Body.String())
	}
	if got := deref(accountByVillageCode(t, env, v.Code).EntityCode); got != "DESA-001" {
		t.Errorf("entity_code otomatis = %q, want DESA-001", got)
	}
}

// TestAccountCreate_EntityCodeManualOverride: field kode sistem diisi → dipakai
// APA ADANYA; create kedua (Desa BEDA) dgn override SAMA → err=entity_code_dup.
// Dua Desa distinct wajib agar tabrakan yang diuji entity_code, bukan
// village_code.
func TestAccountCreate_EntityCodeManualOverride(t *testing.T) {
	env, uid := setupAccounts(t)
	vs := villages(t, env, 2)

	f := withVillage(accountFormValues("prospect"), vs[0])
	f.Set("entity_code", "KODE-KHUSUS")
	if rec := createAccountForm(t, env, uid, f); rec.Code != http.StatusSeeOther {
		t.Fatalf("create override gagal: %d\n%s", rec.Code, rec.Body.String())
	}
	if got := deref(accountByVillageCode(t, env, vs[0].Code).EntityCode); got != "KODE-KHUSUS" {
		t.Errorf("override entity_code = %q, want KODE-KHUSUS (apa adanya)", got)
	}

	// Override sama lagi di Desa berbeda → tabrakan dikenali spesifik.
	dup := withVillage(accountFormValues("prospect"), vs[1])
	dup.Set("entity_code", "KODE-KHUSUS")
	rec := createAccountForm(t, env, uid, dup)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=entity_code_dup") {
		t.Errorf("override bertabrakan harus err=entity_code_dup, got %q", loc)
	}
}

// TestAccountCreate_EntityCodeTooLong: override melebihi 32 karakter ditolak
// backend (err=entity_code) sebelum menyentuh DB.
func TestAccountCreate_EntityCodeTooLong(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)

	f := withVillage(accountFormValues("prospect"), v)
	f.Set("entity_code", strings.Repeat("X", 33))
	rec := createAccountForm(t, env, uid, f)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=entity_code") {
		t.Errorf("override kepanjangan harus err=entity_code, got %q", loc)
	}
	if rows := env.allAccounts(t); len(rows) != 0 {
		t.Errorf("input invalid tak boleh menyimpan, ada %d baris", len(rows))
	}
}

// TestAccountCreate_EntityCodeAutoSkipsManualSlot: override manual menetapkan
// kode YANG SAMA dgn yang akan diproduksi counter otomatis (DESA-001). Create
// otomatis berikutnya harus MELEWATI slot itu → DESA-002 (bukan tabrakan/deadlock),
// dan counter tetap maju (create ketiga → DESA-003). Tiga Desa distinct wajib
// (idx_accounts_code) agar yang diuji murni deret entity_code.
func TestAccountCreate_EntityCodeAutoSkipsManualSlot(t *testing.T) {
	env, uid := setupAccounts(t)
	vs := villages(t, env, 3)
	mkAuto := func(v villageRow) {
		f := withVillage(accountFormValues("prospect"), v)
		if rec := createAccountForm(t, env, uid, f); rec.Code != http.StatusSeeOther {
			t.Fatalf("create auto Desa %q gagal: %d\n%s", v.Code, rec.Code, rec.Body.String())
		}
	}

	// Override manual merebut "DESA-001" (persis format auto).
	manual := withVillage(accountFormValues("prospect"), vs[0])
	manual.Set("entity_code", "DESA-001")
	if rec := createAccountForm(t, env, uid, manual); rec.Code != http.StatusSeeOther {
		t.Fatalf("create manual gagal: %d\n%s", rec.Code, rec.Body.String())
	}

	mkAuto(vs[1])
	if got := deref(accountByVillageCode(t, env, vs[1].Code).EntityCode); got != "DESA-002" {
		t.Errorf("auto harus melompati slot DESA-001 yang direbut → DESA-002, got %q", got)
	}
	mkAuto(vs[2])
	if got := deref(accountByVillageCode(t, env, vs[2].Code).EntityCode); got != "DESA-003" {
		t.Errorf("counter harus tetap maju → DESA-003, got %q", got)
	}
}
