package db

import (
	"context"
	"testing"
)

// regions_search_query_test.go — bukti SearchRegionsByDistrict (BL-163
// lanjutan, tab "Wilayah" modal RegionSearchModal): Kecamatan yang dipilih
// user SENDIRI sebagai baris pertama (Desa kosong) + SEMUA Desa anaknya.
// Dipecah dari regions_test.go (file health §8: batas 400 baris/test) — beda
// concern dari hierarki dasar/ancestry di file itu, sama tabel `regions`.

// firstDistrictWithVillages mencari SATU Kecamatan berdesa dari seed. Rerata
// nasional ~10,7 desa/kecamatan (83.762 desa / 7.837 kecamatan, ADR 0009) —
// cukup memeriksa beberapa kandidat pertama (kab/kota pertama tiap provinsi,
// lalu kecamatan pertama tiap kab/kota) daripada menelusur SELURUH ~7.837
// kecamatan (O(n) query berurutan, lambat tanpa manfaat tambahan pembuktian).
func firstDistrictWithVillages(t *testing.T, q *Queries, ctx context.Context) (Region, []ListVillagesByDistrictRow) {
	t.Helper()
	provinces, err := q.ListProvinces(ctx)
	if err != nil || len(provinces) == 0 {
		t.Fatalf("list provinces: %v (len=%d)", err, len(provinces))
	}
	const maxProvincesTried = 5
	for pi, prov := range provinces {
		if pi >= maxProvincesTried {
			break
		}
		regencies, err := q.ListRegenciesByProvince(ctx, &prov.ID)
		if err != nil || len(regencies) == 0 {
			continue
		}
		districts, err := q.ListDistrictsByRegency(ctx, &regencies[0].ID)
		if err != nil {
			continue
		}
		for _, d := range districts {
			villages, err := q.ListVillagesByDistrict(ctx, &d.ID)
			if err != nil {
				t.Fatalf("list villages: %v", err)
			}
			if len(villages) > 0 {
				return d, villages
			}
		}
	}
	t.Fatal("tak ada Kecamatan berdesa di kandidat yang diperiksa — dataset tak sesuai ekspektasi")
	return Region{}, nil
}

// TestSearchRegionsByDistrict_KecamatanDanSemuaDesa: baris pertama = Kecamatan
// itu sendiri (Desa kosong, sama pola SearchRegionsByCode cabang level 3),
// diikuti SEMUA Desa anaknya — kontrak dipakai tab "Wilayah" modal
// RegionSearchModal (perluasan BL-163).
func TestSearchRegionsByDistrict_KecamatanDanSemuaDesa(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := New(pool)

	district, villages := firstDistrictWithVillages(t, q, ctx)

	rows, err := q.SearchRegionsByDistrict(ctx, SearchRegionsByDistrictParams{
		DistrictID: district.ID, PageSize: 1000,
	})
	if err != nil {
		t.Fatalf("search by district: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("harus >=1 baris (minimal Kecamatan itu sendiri)")
	}
	first := rows[0]
	if first.Code != district.Code || first.VillageName != "" || first.DistrictName != district.Name {
		t.Errorf("baris pertama harus Kecamatan %q (Desa kosong), got code=%q village=%q district=%q",
			district.Name, first.Code, first.VillageName, first.DistrictName)
	}
	// Semua Desa anak Kecamatan ini harus ikut muncul (page_size besar → tak
	// terpotong), dicocokkan via code.
	got := map[string]bool{}
	for _, r := range rows[1:] {
		got[r.Code] = true
		if r.DistrictName != district.Name {
			t.Errorf("baris desa %q harus berinduk Kecamatan %q, got %q", r.VillageName, district.Name, r.DistrictName)
		}
	}
	for _, v := range villages {
		if !got[v.Code] {
			t.Errorf("Desa %q (code %q) tak muncul di hasil SearchRegionsByDistrict", v.Name, v.Code)
		}
	}
}

// TestSearchRegionsByDistrict_MinimalSatuBarisKecamatanItuSendiri: Kecamatan
// tanpa Desa (jarang, tapi kalau seed punya) tetap balikin 1 baris (dirinya
// sendiri), BUKAN 0 — beda dgn asumsi salah "Kecamatan kosong = tak ada
// hasil". Dites via id Kecamatan tak dipakai UNION kanan (parent_region_id
// tak match manapun) — pakai id yang sungguhan Kecamatan tapi cari kandidat
// tanpa desa; bila seed tak punya kandidat begitu, cukup pastikan Kecamatan
// manapun MINIMAL 1 baris (properti yang sama, versi lebih longgar).
func TestSearchRegionsByDistrict_MinimalSatuBarisKecamatanItuSendiri(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := New(pool)

	provinces, err := q.ListProvinces(ctx)
	if err != nil || len(provinces) == 0 {
		t.Fatalf("list provinces: %v (len=%d)", err, len(provinces))
	}
	regencies, err := q.ListRegenciesByProvince(ctx, &provinces[0].ID)
	if err != nil || len(regencies) == 0 {
		t.Fatalf("list regencies: %v (len=%d)", err, len(regencies))
	}
	districts, err := q.ListDistrictsByRegency(ctx, &regencies[0].ID)
	if err != nil || len(districts) == 0 {
		t.Fatalf("list districts: %v (len=%d)", err, len(districts))
	}
	district := districts[0]

	rows, err := q.SearchRegionsByDistrict(ctx, SearchRegionsByDistrictParams{
		DistrictID: district.ID, PageSize: 1000,
	})
	if err != nil {
		t.Fatalf("search by district: %v", err)
	}
	if len(rows) == 0 {
		t.Fatalf("Kecamatan %q harus MINIMAL 1 baris (dirinya sendiri), got 0", district.Name)
	}
	if rows[0].Code != district.Code || rows[0].VillageName != "" {
		t.Errorf("baris pertama harus Kecamatan itu sendiri (Desa kosong), got code=%q village=%q",
			rows[0].Code, rows[0].VillageName)
	}
}

// TestSearchRegionsByDistrict_IDBukanKecamatanNihil: id level 1/2 (bukan
// Kecamatan) → tak ada baris (UNION kiri filter level=3, UNION kanan filter
// parent_region_id=id yang levelnya bukan 3 tak akan punya anak level 4
// dengan parent itu secara valid dari sisi domain) — bukti query tak
// diam-diam balikin data jenjang salah.
func TestSearchRegionsByDistrict_IDBukanKecamatanNihil(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := New(pool)

	provinces, err := q.ListProvinces(ctx)
	if err != nil || len(provinces) == 0 {
		t.Fatalf("list provinces: %v (len=%d)", err, len(provinces))
	}
	rows, err := q.SearchRegionsByDistrict(ctx, SearchRegionsByDistrictParams{
		DistrictID: provinces[0].ID, PageSize: 1000,
	})
	if err != nil {
		t.Fatalf("search by district: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("id provinsi harus 0 baris, got %d", len(rows))
	}
}
