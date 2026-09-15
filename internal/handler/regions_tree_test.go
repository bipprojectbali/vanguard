package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"testing"

	"go_starter/internal/db"
)

// regions_tree_test.go — perluasan BL-163: endpoint lazy-fetch cascading
// Provinsi→Kabupaten/Kota→Kecamatan tab "Wilayah" modal RegionSearchModal.
// Fixture pakai data REAL seed migrasi (sama pola regions_search_test.go),
// bukan seed sendiri.

// regionsTreeReq membangun request GET /w/test/regions/tree?level=..&parent=...
func regionsTreeReq(level, parent string) *http.Request {
	q := "level=" + url.QueryEscape(level)
	if parent != "" {
		q += "&parent=" + url.QueryEscape(parent)
	}
	return accountsReq(http.MethodGet, "/w/test/regions/tree?"+q, nil, "")
}

// decodeRegions mem-parse body JSON []db.Region — gagal parse = fatal (respons
// bukan array Region yang valid, kontrak AccountVillages yang ditiru).
func decodeRegions(t *testing.T, body []byte) []db.Region {
	t.Helper()
	var rows []db.Region
	if err := json.Unmarshal(body, &rows); err != nil {
		t.Fatalf("decode JSON []db.Region: %v\nbody: %s", err, body)
	}
	return rows
}

// TestRegionsTree_Level1_Provinces: level=1 tanpa parent → daftar SEMUA
// provinsi (ListProvinces), tak butuh parent.
func TestRegionsTree_Level1_Provinces(t *testing.T) {
	env, uid := setupAccounts(t)

	want, err := env.q.ListProvinces(t.Context())
	if err != nil || len(want) == 0 {
		t.Fatalf("list provinces (langsung): %v (len=%d)", err, len(want))
	}

	req := regionsTreeReq("1", "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.RegionsTree)
	if rec.Code != http.StatusOK {
		t.Fatalf("level=1 harus 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got := decodeRegions(t, rec.Body.Bytes())
	if len(got) != len(want) {
		t.Fatalf("jumlah provinsi respons (%d) tak cocok query langsung (%d)", len(got), len(want))
	}
	for _, r := range got {
		if r.Level != 1 {
			t.Errorf("semua baris harus level=1, ada level=%d (%s)", r.Level, r.Name)
		}
	}
}

// TestRegionsTree_Level2_RequiresValidParent: level=2 tanpa parent → 400;
// parent bukan int64 sah → 400; parent valid → kab/kota di bawahnya.
func TestRegionsTree_Level2_RequiresValidParent(t *testing.T) {
	env, uid := setupAccounts(t)

	if rec := env.runAccount(uid, "member", "sales", regionsTreeReq("2", ""), env.h.RegionsTree); rec.Code != http.StatusBadRequest {
		t.Errorf("level=2 tanpa parent harus 400, got %d", rec.Code)
	}
	if rec := env.runAccount(uid, "member", "sales", regionsTreeReq("2", "bukan-angka"), env.h.RegionsTree); rec.Code != http.StatusBadRequest {
		t.Errorf("level=2 parent tak valid harus 400, got %d", rec.Code)
	}

	provinces, err := env.q.ListProvinces(t.Context())
	if err != nil || len(provinces) == 0 {
		t.Fatalf("list provinces: %v (len=%d)", err, len(provinces))
	}
	want, err := env.q.ListRegenciesByProvince(t.Context(), &provinces[0].ID)
	if err != nil || len(want) == 0 {
		t.Fatalf("list regencies (langsung): %v (len=%d)", err, len(want))
	}

	req := regionsTreeReq("2", strconv.FormatInt(provinces[0].ID, 10))
	rec := env.runAccount(uid, "member", "sales", req, env.h.RegionsTree)
	if rec.Code != http.StatusOK {
		t.Fatalf("level=2 parent valid harus 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got := decodeRegions(t, rec.Body.Bytes())
	if len(got) != len(want) {
		t.Fatalf("jumlah kab/kota respons (%d) tak cocok query langsung (%d)", len(got), len(want))
	}
}

// TestRegionsTree_Level3_RequiresValidParent: sama kontrak level=2, tapi utk
// Kecamatan di bawah satu kab/kota (ListDistrictsByRegency).
func TestRegionsTree_Level3_RequiresValidParent(t *testing.T) {
	env, uid := setupAccounts(t)

	if rec := env.runAccount(uid, "member", "sales", regionsTreeReq("3", ""), env.h.RegionsTree); rec.Code != http.StatusBadRequest {
		t.Errorf("level=3 tanpa parent harus 400, got %d", rec.Code)
	}
	if rec := env.runAccount(uid, "member", "sales", regionsTreeReq("3", "bukan-angka"), env.h.RegionsTree); rec.Code != http.StatusBadRequest {
		t.Errorf("level=3 parent tak valid harus 400, got %d", rec.Code)
	}

	v := firstVillage(t, env)
	anc, err := env.q.GetRegionAncestry(t.Context(), v.DistrictID)
	if err != nil {
		t.Fatalf("ancestry kecamatan fixture: %v", err)
	}
	want, err := env.q.ListDistrictsByRegency(t.Context(), &anc.RegencyID)
	if err != nil || len(want) == 0 {
		t.Fatalf("list districts (langsung): %v (len=%d)", err, len(want))
	}

	req := regionsTreeReq("3", strconv.FormatInt(anc.RegencyID, 10))
	rec := env.runAccount(uid, "member", "sales", req, env.h.RegionsTree)
	if rec.Code != http.StatusOK {
		t.Fatalf("level=3 parent valid harus 200, got %d: %s", rec.Code, rec.Body.String())
	}
	got := decodeRegions(t, rec.Body.Bytes())
	if len(got) != len(want) {
		t.Fatalf("jumlah kecamatan respons (%d) tak cocok query langsung (%d)", len(got), len(want))
	}
}

// TestRegionsTree_InvalidLevel_400: level di luar 1-3 (termasuk kosong/level
// 4 — Desa sengaja tak lewat endpoint ini, dataset terlalu besar utk cascading
// tab "Wilayah") → 400.
func TestRegionsTree_InvalidLevel_400(t *testing.T) {
	env, uid := setupAccounts(t)

	for _, level := range []string{"", "0", "4", "abc"} {
		req := regionsTreeReq(level, "")
		rec := env.runAccount(uid, "member", "sales", req, env.h.RegionsTree)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("level=%q harus 400, got %d", level, rec.Code)
		}
	}
}
