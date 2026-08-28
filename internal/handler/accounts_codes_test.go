package handler

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// accounts_codes_test.go — bukti alokasi DUA kode desa saat create lewat
// AccountCreate (jalur end-to-end handler): village_code Kemendagri OTOMATIS dari
// Kecamatan (Kecamatan kini WAJIB), dan entity_code sistem otomatis-by-default
// tapi bisa ditimpa manual — keduanya tanpa tabrakan. Query pendukungnya diuji
// terpisah di paket db (regions_test.go, account_codes_test.go).

// twoDistricts mengambil dua Kecamatan (level 3) BERBEDA dari master regions
// (seed migrasi 00026) beserta kode Kemendagri-nya — dipakai menegakkan bahwa
// village_code otomatis mengikuti kode Kecamatan terpilih & deret nomornya
// terpisah per Kecamatan.
func twoDistricts(t *testing.T, env *testEnv) (id1 int64, code1 string, id2 int64, code2 string) {
	t.Helper()
	ctx := t.Context()
	// ListDistrictsByRegency balikin Region lengkap (dgn Code) — ListAllRegions
	// tak memuat kode. Cari kab/kota pertama yang punya >=2 Kecamatan supaya dua
	// prefix village_code berbeda bisa diuji (deret nomor terpisah per prefix).
	provinces, err := env.q.ListProvinces(ctx)
	if err != nil || len(provinces) == 0 {
		t.Fatalf("list provinces: %v (len=%d)", err, len(provinces))
	}
	regencies, err := env.q.ListRegenciesByProvince(ctx, &provinces[0].ID)
	if err != nil || len(regencies) == 0 {
		t.Fatalf("list regencies: %v (len=%d)", err, len(regencies))
	}
	for _, reg := range regencies {
		id := reg.ID
		districts, err := env.q.ListDistrictsByRegency(ctx, &id)
		if err != nil {
			t.Fatalf("list districts: %v", err)
		}
		if len(districts) >= 2 {
			return districts[0].ID, districts[0].Code, districts[1].ID, districts[1].Code
		}
	}
	t.Fatal("tak ada kab/kota dgn >=2 Kecamatan di provinsi pertama — seed 00026 tak wajar")
	return 0, "", 0, ""
}

// createAccountForm menjalankan AccountCreate dgn field yang diberi dan
// mengembalikan recorder-nya.
func createAccountForm(t *testing.T, env *testEnv, uid int64, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := accountsReq(http.MethodPost, "/w/test/accounts", form, "")
	return env.runAccount(uid, "owner", "sales", req, env.h.AccountCreate)
}

// accountByName mencari desa terseed berdasarkan nama dari daftar semua desa.
func accountByName(t *testing.T, env *testEnv, name string) db.Account {
	t.Helper()
	for _, a := range env.allAccounts(t) {
		if a.VillageName == name {
			return a
		}
	}
	t.Fatalf("desa %q tak ditemukan setelah create", name)
	return db.Account{}
}

// TestAccountCreate_VillageCodeAutoPerDistrict: village_code = <kode
// Kecamatan>.0001, create kedua di Kecamatan SAMA → .0002, create di Kecamatan
// BEDA → mulai lagi dari .0001 (deret nomor terpisah per prefix Kecamatan).
func TestAccountCreate_VillageCodeAutoPerDistrict(t *testing.T) {
	env, uid := setupAccounts(t)
	d1, code1, d2, code2 := twoDistricts(t, env)

	mk := func(name string, districtID int64) {
		f := accountFormValues(name, "prospect")
		f.Set("district_id", strconv.FormatInt(districtID, 10))
		if rec := createAccountForm(t, env, uid, f); rec.Code != http.StatusSeeOther {
			t.Fatalf("create %q gagal: %d\n%s", name, rec.Code, rec.Body.String())
		}
	}

	mk("Desa Satu", d1)
	mk("Desa Dua", d1)
	mk("Desa Tiga", d2)

	if got := deref(accountByName(t, env, "Desa Satu").VillageCode); got != code1+".0001" {
		t.Errorf("desa pertama village_code = %q, want %q", got, code1+".0001")
	}
	if got := deref(accountByName(t, env, "Desa Dua").VillageCode); got != code1+".0002" {
		t.Errorf("desa kedua (Kecamatan sama) village_code = %q, want %q", got, code1+".0002")
	}
	if got := deref(accountByName(t, env, "Desa Tiga").VillageCode); got != code2+".0001" {
		t.Errorf("desa di Kecamatan beda village_code = %q, want %q (deret ulang dari .0001)", got, code2+".0001")
	}
}

// TestAccountCreate_DistrictRequired: tanpa Kecamatan → redirect
// err=district_required dan TAK ada baris tersimpan (village_code tak bisa
// dirakit tanpa Kecamatan).
func TestAccountCreate_DistrictRequired(t *testing.T) {
	env, uid := setupAccounts(t)
	f := accountFormValues("Desa Tanpa Kecamatan", "prospect")
	rec := createAccountForm(t, env, uid, f)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=district_required") {
		t.Errorf("harus err=district_required, got %q", loc)
	}
	if rows := env.allAccounts(t); len(rows) != 0 {
		t.Errorf("Kecamatan wajib — tak boleh menyimpan baris, ada %d", len(rows))
	}
}

// TestAccountCreate_EntityCodeAutoDefault: field kode sistem dikosongkan → dibuat
// otomatis (DESA-001 di format default).
func TestAccountCreate_EntityCodeAutoDefault(t *testing.T) {
	env, uid := setupAccounts(t)
	d1, _, _, _ := twoDistricts(t, env)

	f := accountFormValues("Desa Auto", "prospect")
	f.Set("district_id", strconv.FormatInt(d1, 10))
	if rec := createAccountForm(t, env, uid, f); rec.Code != http.StatusSeeOther {
		t.Fatalf("create gagal: %d\n%s", rec.Code, rec.Body.String())
	}
	if got := deref(accountByName(t, env, "Desa Auto").EntityCode); got != "DESA-001" {
		t.Errorf("entity_code otomatis = %q, want DESA-001", got)
	}
}

// TestAccountCreate_EntityCodeManualOverride: field kode sistem diisi → dipakai
// APA ADANYA; create kedua dgn override SAMA → err=entity_code_dup.
func TestAccountCreate_EntityCodeManualOverride(t *testing.T) {
	env, uid := setupAccounts(t)
	d1, _, _, _ := twoDistricts(t, env)

	f := accountFormValues("Desa Manual", "prospect")
	f.Set("district_id", strconv.FormatInt(d1, 10))
	f.Set("entity_code", "KODE-KHUSUS")
	if rec := createAccountForm(t, env, uid, f); rec.Code != http.StatusSeeOther {
		t.Fatalf("create override gagal: %d\n%s", rec.Code, rec.Body.String())
	}
	if got := deref(accountByName(t, env, "Desa Manual").EntityCode); got != "KODE-KHUSUS" {
		t.Errorf("override entity_code = %q, want KODE-KHUSUS (apa adanya)", got)
	}

	// Override sama lagi → tabrakan dikenali spesifik (bukan "internal error").
	dup := accountFormValues("Desa Manual 2", "prospect")
	dup.Set("district_id", strconv.FormatInt(d1, 10))
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
	d1, _, _, _ := twoDistricts(t, env)

	f := accountFormValues("Desa Panjang", "prospect")
	f.Set("district_id", strconv.FormatInt(d1, 10))
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
// dan counter tetap maju (create ketiga → DESA-003).
func TestAccountCreate_EntityCodeAutoSkipsManualSlot(t *testing.T) {
	env, uid := setupAccounts(t)
	d1, _, _, _ := twoDistricts(t, env)
	mkAuto := func(name string) {
		f := accountFormValues(name, "prospect")
		f.Set("district_id", strconv.FormatInt(d1, 10))
		if rec := createAccountForm(t, env, uid, f); rec.Code != http.StatusSeeOther {
			t.Fatalf("create auto %q gagal: %d\n%s", name, rec.Code, rec.Body.String())
		}
	}

	// Override manual merebut "DESA-001" (persis format auto).
	manual := accountFormValues("Desa Rebut", "prospect")
	manual.Set("district_id", strconv.FormatInt(d1, 10))
	manual.Set("entity_code", "DESA-001")
	if rec := createAccountForm(t, env, uid, manual); rec.Code != http.StatusSeeOther {
		t.Fatalf("create manual gagal: %d\n%s", rec.Code, rec.Body.String())
	}

	mkAuto("Desa Lompat")
	if got := deref(accountByName(t, env, "Desa Lompat").EntityCode); got != "DESA-002" {
		t.Errorf("auto harus melompati slot DESA-001 yang direbut → DESA-002, got %q", got)
	}
	mkAuto("Desa Lanjut")
	if got := deref(accountByName(t, env, "Desa Lanjut").EntityCode); got != "DESA-003" {
		t.Errorf("counter harus tetap maju → DESA-003, got %q", got)
	}
}
