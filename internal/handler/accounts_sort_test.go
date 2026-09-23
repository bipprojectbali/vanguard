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
