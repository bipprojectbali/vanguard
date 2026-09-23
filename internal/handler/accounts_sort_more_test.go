package handler

import (
	"net/http"
	"strings"
	"testing"
)

// accounts_sort_more_test.go — lanjutan accounts_sort_test.go
// (dipecah krn ambang File Health; lihat komentar file itu utk konteks BL-157c).

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
