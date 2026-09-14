package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// accounts_sort_test.go — BL-157c: daftar Accounts (Desa) terurut ke-6
// kolomnya via ?sort=col&dir=asc|desc, klon arsitektur BL-157a/157b
// (subscriptions_sort_test.go / sales_leads_sort_test.go). Tak menguji ulang
// SEMUA kolom simetris — representatif per pola cursor (NOT NULL teks:
// village; NOT NULL raw enum: type; nullable via JOIN 2-hop: regency; nullable
// via JOIN 3-hop: province; nullable via JOIN users: owner/csm) + fallback +
// F3 + tab + pagination, karena mekanisme cursor generik (sortcursor.go) sudah
// teruji tuntas di 157a/157b.
//
// (Kontrak URL header/tab/pager sort di sisi VIEW diuji di
// panel/accounts_sort_test.go.)

// seedAccountRegion = varian seed desa dengan kontrol penuh atas account_type
// & district_id (untuk uji sort Tipe/Regency/Province) — seedAccount/
// seedAccountFull (accounts_test.go) tak cukup fleksibel karena account_type
// dipatok "prospect" & district_id selalu nil.
func (e *testEnv) seedAccountRegion(t *testing.T, name string, owner *int64, accountType string, districtID *int64) db.Account {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityAccount)
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	if accountType == "" {
		accountType = "prospect"
	}
	a, err := e.q.CreateAccount(t.Context(), db.CreateAccountParams{
		TenantID:     e.tenantID,
		EntityCode:   &code,
		VillageName:  name,
		AccountType:  accountType,
		AccountOwner: owner,
		DistrictID:   districtID,
		CreatedBy:    owner,
	})
	if err != nil {
		t.Fatalf("seed account %s: %v", name, err)
	}
	return a
}

// twoDistrictsDiffProvince mengembalikan dua district_id REAL dari DUA
// provinsi berbeda (seed migrasi 00039) + ancestry (nama Kab/Kota & Provinsi)
// masing-masing — dipakai uji sort Regency/Province TANPA menghardkode nama
// wilayah yang bisa berbeda antar seed data.
func twoDistrictsDiffProvince(t *testing.T, env *testEnv) (d1, d2 int64, a1, a2 db.GetRegionAncestryRow) {
	t.Helper()
	ctx := t.Context()
	provinces, err := env.q.ListProvinces(ctx)
	if err != nil || len(provinces) < 2 {
		t.Fatalf("list provinces: %v (len=%d)", err, len(provinces))
	}
	districtIn := func(provinceID int64) int64 {
		pid := provinceID
		regencies, err := env.q.ListRegenciesByProvince(ctx, &pid)
		if err != nil || len(regencies) == 0 {
			t.Fatalf("list regencies utk provinsi %d: %v (len=%d)", provinceID, err, len(regencies))
		}
		for _, reg := range regencies {
			rid := reg.ID
			districts, err := env.q.ListDistrictsByRegency(ctx, &rid)
			if err != nil {
				t.Fatalf("list districts: %v", err)
			}
			if len(districts) > 0 {
				return districts[0].ID
			}
		}
		t.Fatalf("provinsi %d tak punya kecamatan — seed 00039 tak wajar", provinceID)
		return 0
	}
	d1 = districtIn(provinces[0].ID)
	d2 = districtIn(provinces[len(provinces)-1].ID)
	a1, err = env.q.GetRegionAncestry(ctx, d1)
	if err != nil {
		t.Fatalf("ancestry d1: %v", err)
	}
	a2, err = env.q.GetRegionAncestry(ctx, d2)
	if err != nil {
		t.Fatalf("ancestry d2: %v", err)
	}
	if a1.RegencyName == a2.RegencyName || a1.ProvinceName == a2.ProvinceName {
		t.Fatalf("dua provinsi berbeda kebetulan punya nama regency/provinsi sama — pilih pasangan lain")
	}
	return
}

func TestAccountsList_SortVillageAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAccount(t, "Desa Zebra", &uid, nil, nil)
	env.seedAccount(t, "Desa Awal", &uid, nil, nil)
	env.seedAccount(t, "Desa Mekar", &uid, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts?sort=village&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "Desa Awal")
	iMekar := strings.Index(body, "Desa Mekar")
	iZebra := strings.Index(body, "Desa Zebra")
	if iAwal < 0 || iMekar < 0 || iZebra < 0 {
		t.Fatalf("ketiga desa harus tampil:\n%s", body)
	}
	if !(iAwal < iMekar && iMekar < iZebra) {
		t.Errorf("dir=asc harus urut Awal < Mekar < Zebra, dapat posisi %d/%d/%d", iAwal, iMekar, iZebra)
	}
}

func TestAccountsList_SortVillageDesc(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAccount(t, "Desa Zebra", &uid, nil, nil)
	env.seedAccount(t, "Desa Awal", &uid, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts?sort=village&dir=desc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iAwal := strings.Index(body, "Desa Awal")
	iZebra := strings.Index(body, "Desa Zebra")
	if iAwal < 0 || iZebra < 0 {
		t.Fatalf("kedua desa harus tampil:\n%s", body)
	}
	if !(iZebra < iAwal) {
		t.Errorf("dir=desc harus urut Zebra < Awal, dapat posisi %d/%d", iZebra, iAwal)
	}
}

// ?sort= tak dikenal → TAK error, jatuh ke jalur default (created_at DESC).
func TestAccountsList_SortUnknownFallsBackToDefault(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAccount(t, "Desa Fallback", &uid, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts?sort=bogus", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Desa Fallback") {
		t.Errorf("?sort= tak dikenal harus tetap render daftar (jatuh ke default)")
	}
}

// Tipe (NOT NULL, raw enum alfabetis: customer < former_customer < prospect —
// bukan urutan label Indonesia yang tampil).
func TestAccountsList_SortTypeAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAccountRegion(t, "Desa TypeCustomer", &uid, "customer", nil)
	env.seedAccountRegion(t, "Desa TypeFormer", &uid, "former_customer", nil)
	env.seedAccountRegion(t, "Desa TypeProspect", &uid, "prospect", nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts?sort=type&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iC := strings.Index(body, "Desa TypeCustomer")
	iF := strings.Index(body, "Desa TypeFormer")
	iP := strings.Index(body, "Desa TypeProspect")
	if iC < 0 || iF < 0 || iP < 0 {
		t.Fatalf("ketiga baris tipe harus tampil:\n%s", body)
	}
	if !(iC < iF && iF < iP) {
		t.Errorf("dir=asc harus urut customer < former_customer < prospect, dapat posisi %d/%d/%d", iC, iF, iP)
	}
}

// Kab/Kota (district_id nullable, kunci sort = nama regency via LEFT JOIN
// 2-hop). asc→NULL akhir, desc→NULL awal.
func TestAccountsList_SortRegencyAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	d1, d2, a1, a2 := twoDistrictsDiffProvince(t, env)
	lowID, highID := d1, d2
	if a1.RegencyName > a2.RegencyName {
		lowID, highID = d2, d1
	}
	env.seedAccountRegion(t, "Desa RegencyTinggi", &uid, "", &highID)
	env.seedAccountRegion(t, "Desa RegencyRendah", &uid, "", &lowID)
	env.seedAccountRegion(t, "Desa RegencyNull", &uid, "", nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts?sort=regency&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iLow := strings.Index(body, "Desa RegencyRendah")
	iHigh := strings.Index(body, "Desa RegencyTinggi")
	iNull := strings.Index(body, "Desa RegencyNull")
	if iLow < 0 || iHigh < 0 || iNull < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iLow < iHigh && iHigh < iNull) {
		t.Errorf("dir=asc harus urut regency-rendah < regency-tinggi < NULL(akhir), dapat posisi %d/%d/%d", iLow, iHigh, iNull)
	}
}

func TestAccountsList_SortRegencyDescNullsFirst(t *testing.T) {
	env, uid := setupAccounts(t)
	d1, d2, a1, a2 := twoDistrictsDiffProvince(t, env)
	lowID, highID := d1, d2
	if a1.RegencyName > a2.RegencyName {
		lowID, highID = d2, d1
	}
	env.seedAccountRegion(t, "Desa RegencyTinggi", &uid, "", &highID)
	env.seedAccountRegion(t, "Desa RegencyRendah", &uid, "", &lowID)
	env.seedAccountRegion(t, "Desa RegencyNull", &uid, "", nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts?sort=regency&dir=desc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iLow := strings.Index(body, "Desa RegencyRendah")
	iHigh := strings.Index(body, "Desa RegencyTinggi")
	iNull := strings.Index(body, "Desa RegencyNull")
	if iLow < 0 || iHigh < 0 || iNull < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iNull < iHigh && iHigh < iLow) {
		t.Errorf("dir=desc harus urut NULL(awal) < regency-tinggi < regency-rendah, dapat posisi %d/%d/%d", iNull, iHigh, iLow)
	}
}

// Provinsi (district_id nullable, kunci sort = nama provinsi via LEFT JOIN
// 3-hop) — representatif membuktikan hop tambahan bekerja; pola NULL sudah
// tuntas diuji di Regency.
func TestAccountsList_SortProvinceAsc(t *testing.T) {
	env, uid := setupAccounts(t)
	d1, d2, a1, a2 := twoDistrictsDiffProvince(t, env)
	lowID, highID := d1, d2
	if a1.ProvinceName > a2.ProvinceName {
		lowID, highID = d2, d1
	}
	env.seedAccountRegion(t, "Desa ProvinsiTinggi", &uid, "", &highID)
	env.seedAccountRegion(t, "Desa ProvinsiRendah", &uid, "", &lowID)

	req := accountsReq(http.MethodGet, "/w/test/accounts?sort=province&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iLow := strings.Index(body, "Desa ProvinsiRendah")
	iHigh := strings.Index(body, "Desa ProvinsiTinggi")
	if iLow < 0 || iHigh < 0 {
		t.Fatalf("kedua baris harus tampil:\n%s", body)
	}
	if !(iLow < iHigh) {
		t.Errorf("dir=asc harus urut provinsi-rendah < provinsi-tinggi, dapat posisi %d/%d", iLow, iHigh)
	}
}

// Owner (account_owner nullable, kunci sort = nama/email via LEFT JOIN users).
func TestAccountsList_SortOwnerAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	memA := env.seedMember(t, "acc-owner-awal@x", "member", env.tenantID)
	memZ := env.seedMember(t, "acc-owner-zebra@x", "member", env.tenantID)

	env.seedAccount(t, "Desa OwnerAwal", &memA.ID, nil, nil)
	env.seedAccount(t, "Desa OwnerZebra", &memZ.ID, nil, nil)
	env.seedAccount(t, "Desa OwnerNull", nil, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts?sort=owner&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Desa OwnerAwal")
	iZ := strings.Index(body, "Desa OwnerZebra")
	iN := strings.Index(body, "Desa OwnerNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iA < iZ && iZ < iN) {
		t.Errorf("dir=asc harus urut email-Awal < email-Zebra < NULL(akhir), dapat posisi %d/%d/%d", iA, iZ, iN)
	}
}

// CS (assigned_csm nullable, kunci sort = nama/email via LEFT JOIN users —
// join TERPISAH dari owner, join hanya ke assigned_csm bukan backup_csm sesuai
// kolom yang ditampilkan).
func TestAccountsList_SortCsmAscNullsLast(t *testing.T) {
	env, uid := setupAccounts(t)
	memA := env.seedMember(t, "acc-csm-awal@x", "member", env.tenantID)
	memZ := env.seedMember(t, "acc-csm-zebra@x", "member", env.tenantID)

	env.seedAccount(t, "Desa CsmAwal", &uid, &memA.ID, nil)
	env.seedAccount(t, "Desa CsmZebra", &uid, &memZ.ID, nil)
	env.seedAccount(t, "Desa CsmNull", &uid, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts?sort=csm&dir=asc", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	iA := strings.Index(body, "Desa CsmAwal")
	iZ := strings.Index(body, "Desa CsmZebra")
	iN := strings.Index(body, "Desa CsmNull")
	if iA < 0 || iZ < 0 || iN < 0 {
		t.Fatalf("ketiga baris harus tampil:\n%s", body)
	}
	if !(iA < iZ && iZ < iN) {
		t.Errorf("dir=asc harus urut email-Awal < email-Zebra < NULL(akhir), dapat posisi %d/%d/%d", iA, iZ, iN)
	}
}

// F3 ownership tetap dihormati di jalur sort (representatif, predikat
// ownership TAK berubah antar kolom — cukup 1 test lintas kolom yang sudah ada).
func TestAccountsList_SortVillageRespectsF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain-acc-sort@x", "member", env.tenantID)

	env.seedAccount(t, "Desa MilikSaya", &uid, nil, nil)
	env.seedAccount(t, "Desa MilikOrang", &lain.ID, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts?sort=village&dir=asc", nil, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "MilikSaya") {
		t.Errorf("sales harus melihat desa MILIKNYA walau sort aktif")
	}
	if strings.Contains(body, "MilikOrang") {
		t.Errorf("sort tak boleh menembus F3: sales melihat desa milik orang lain")
	}
}

// Kombinasi ?sort=village&view=unowned tetap menghormati filter tab
// (account_owner IS NULL) — sumbu ORTOGONAL dari ownership scope F3.
func TestAccountsList_SortVillageWithViewUnowned(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAccount(t, "Desa BerOwner", &uid, nil, nil)
	env.seedAccount(t, "Desa TanpaOwner", nil, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts?sort=village&dir=asc&view=unowned", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "TanpaOwner") {
		t.Errorf("view=unowned harus tetap menampilkan desa tanpa owner walau sort aktif")
	}
	if strings.Contains(body, "BerOwner") {
		t.Errorf("view=unowned tak boleh menampilkan desa ber-owner walau sort aktif")
	}
}

func TestAccountsList_SortVillagePagination(t *testing.T) {
	env, uid := setupAccounts(t)
	var lastName string
	for i := 0; i <= pageSize; i++ {
		name := "Desa V" + padded(i)
		env.seedAccount(t, name, &uid, nil, nil)
		lastName = name
	}

	first := accountsReq(http.MethodGet, "/w/test/accounts?sort=village&dir=asc", nil, "")
	rec1 := env.runAccount(uid, "owner", "sales", first, env.h.AccountsList)
	if rec1.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec1.Code, rec1.Body.String())
	}
	body1 := rec1.Body.String()
	if strings.Contains(body1, lastName) {
		t.Fatalf("desa urutan akhir (asc) seharusnya belum tampil di halaman pertama")
	}
	after := sortAfter(body1, "/w/test/accounts")
	if after == "" {
		t.Fatalf("halaman pertama harus menawarkan jalan ke berikutnya:\n%s", body1)
	}

	second := accountsReq(http.MethodGet,
		"/w/test/accounts?sort=village&dir=asc&after="+after, nil, "")
	rec2 := env.runAccount(uid, "owner", "sales", second, env.h.AccountsList)
	if rec2.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), lastName) {
		t.Errorf("desa urutan akhir harus muncul di halaman kedua — daftar masih terpotong")
	}
}
