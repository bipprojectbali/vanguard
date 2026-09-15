package db

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

// regions_test.go — bukti master wilayah administratif GLOBAL (ADR 0009):
// Provinsi → Kabupaten/Kota → Kecamatan (00026) → Desa/Kelurahan (00039, BL-66),
// di-seed migrasi (bukan data yang ditulis test ini sendiri — tabel ini TANPA
// RLS/tenant_id, dibaca lewat New(pool) langsung, sama pola dgn q.CreateTenant di
// accounts_test.go).
//
// Yang dijaga di sini, bila rusak, tak terlihat dari perilaku aplikasi sampai
// dropdown wilayah kosong/salah jenjang di form:
//
//   (1) 4 level ter-seed dgn jumlah wajar (34+ provinsi, ratusan kab/kota, ribuan
//       kecamatan, puluhan ribu desa) — bukan cuma tabel kosong yang lolos migrasi.
//   (2) ListRegenciesByProvince/ListDistrictsByRegency benar-benar MENYARING ke
//       parent_region_id yang diminta, bukan balikin semua baris.
//   (3) GetRegionAncestry balikin 3 nama sekaligus (Kecamatan/Kab-Kota/Provinsi)
//       utk satu district_id — dipakai list/detail/dupe-banner yang tampil tanpa JS.

func TestListProvinces_SeededFromMigration(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := New(pool)

	rows, err := q.ListProvinces(ctx)
	if err != nil {
		t.Fatalf("list provinces: %v", err)
	}
	// Indonesia resmi 34+ provinsi (sumber cahyadsn/wilayah bisa nambah pemekaran
	// baru) — cek batas bawah wajar, bukan angka pas, agar tak rapuh thd update seed.
	if len(rows) < 30 {
		t.Fatalf("harus >=30 provinsi dari seed, got %d", len(rows))
	}
	for _, r := range rows {
		if r.Level != 1 {
			t.Errorf("ListProvinces harus level=1 semua, ada level=%d (%s)", r.Level, r.Name)
		}
	}
}

func TestListAllRegions_FourLevels(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := New(pool)

	rows, err := q.ListAllRegions(ctx)
	if err != nil {
		t.Fatalf("list all regions: %v", err)
	}
	// BL-66: migrasi 00039 menambah level 4 (Desa/Kelurahan, ~83.762 baris) di
	// atas 3 level 00026 (~7.837) → total puluhan ribu.
	if len(rows) < 80000 {
		t.Fatalf("harus >=80000 baris (4 level, cahyadsn/wilayah), got %d", len(rows))
	}
	var l1, l2, l3, l4 int
	for _, r := range rows {
		switch r.Level {
		case 1:
			l1++
		case 2:
			l2++
		case 3:
			l3++
		case 4:
			l4++
		default:
			t.Fatalf("level di luar 1-4 lolos seed: %d (%s)", r.Level, r.Name)
		}
	}
	if l1 == 0 || l2 == 0 || l3 == 0 || l4 == 0 {
		t.Fatalf("keempat level harus terisi, got provinsi=%d kab/kota=%d kecamatan=%d desa=%d", l1, l2, l3, l4)
	}
}

func TestListRegenciesByProvince_MenyaringParent(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := New(pool)

	provinces, err := q.ListProvinces(ctx)
	if err != nil || len(provinces) == 0 {
		t.Fatalf("list provinces: %v (len=%d)", err, len(provinces))
	}
	target := provinces[0]

	regencies, err := q.ListRegenciesByProvince(ctx, &target.ID)
	if err != nil {
		t.Fatalf("list regencies: %v", err)
	}
	if len(regencies) == 0 {
		t.Fatalf("provinsi %q harus punya >=1 kab/kota", target.Name)
	}
	for _, r := range regencies {
		if r.Level != 2 {
			t.Errorf("harus level=2, got %d", r.Level)
		}
		if r.ParentRegionID == nil || *r.ParentRegionID != target.ID {
			t.Errorf("kab/kota %q harus berinduk provinsi %d, got %v", r.Name, target.ID, r.ParentRegionID)
		}
	}
}

func TestListDistrictsByRegency_MenyaringParent(t *testing.T) {
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
	target := regencies[0]

	districts, err := q.ListDistrictsByRegency(ctx, &target.ID)
	if err != nil {
		t.Fatalf("list districts: %v", err)
	}
	if len(districts) == 0 {
		t.Fatalf("kab/kota %q harus punya >=1 kecamatan", target.Name)
	}
	for _, d := range districts {
		if d.Level != 3 {
			t.Errorf("harus level=3, got %d", d.Level)
		}
		if d.ParentRegionID == nil || *d.ParentRegionID != target.ID {
			t.Errorf("kecamatan %q harus berinduk kab/kota %d, got %v", d.Name, target.ID, d.ParentRegionID)
		}
	}
}

func TestGetRegionAncestry_TigaJenjangSekaligus(t *testing.T) {
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

	anc, err := q.GetRegionAncestry(ctx, district.ID)
	if err != nil {
		t.Fatalf("get region ancestry: %v", err)
	}
	if anc.DistrictID != district.ID || anc.DistrictName != district.Name {
		t.Errorf("kecamatan tak cocok: got id=%d name=%q, want id=%d name=%q",
			anc.DistrictID, anc.DistrictName, district.ID, district.Name)
	}
	if anc.RegencyID != regencies[0].ID || anc.RegencyName != regencies[0].Name {
		t.Errorf("kab/kota tak cocok: got id=%d name=%q, want id=%d name=%q",
			anc.RegencyID, anc.RegencyName, regencies[0].ID, regencies[0].Name)
	}
	if anc.ProvinceID != provinces[0].ID || anc.ProvinceName != provinces[0].Name {
		t.Errorf("provinsi tak cocok: got id=%d name=%q, want id=%d name=%q",
			anc.ProvinceID, anc.ProvinceName, provinces[0].ID, provinces[0].Name)
	}
}

func TestGetRegionAncestry_BukanKecamatanGagal(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := New(pool)

	provinces, err := q.ListProvinces(ctx)
	if err != nil || len(provinces) == 0 {
		t.Fatalf("list provinces: %v (len=%d)", err, len(provinces))
	}
	// id level=1 (provinsi) diminta sbg district_id → WHERE d.level=3 menolak,
	// pgx.ErrNoRows — bukti query tak diam-diam balikin baris jenjang salah.
	if _, err := q.GetRegionAncestry(ctx, provinces[0].ID); err == nil {
		t.Error("GetRegionAncestry(id provinsi) harus gagal (bukan level=3), tapi sukses")
	}
}

// TestGetDistrictCode_ReturnsLevel3Code: kode Kemendagri satu Kecamatan (level 3)
// dikembalikan apa adanya — dipakai sbg prefix village_code otomatis
// (generateVillageCode). Ambil satu Kecamatan sungguhan dari seed, bukan hardcode
// id yang bisa bergeser bila urutan seed berubah.
func TestGetDistrictCode_ReturnsLevel3Code(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := New(pool)

	// ListDistrictsByRegency balikin Region lengkap (termasuk Code) — beda dgn
	// ListAllRegions yang tak memuat kode. Telusur provinsi→kab/kota→kecamatan
	// utk dapat satu Kecamatan sungguhan beserta kode-nya.
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

	code, err := q.GetDistrictCode(ctx, district.ID)
	if err != nil {
		t.Fatalf("get district code: %v", err)
	}
	if code != district.Code {
		t.Errorf("kode = %q, want %q (kode Kecamatan sesungguhnya)", code, district.Code)
	}
}

// TestGetDistrictCode_NonDistrictErrNoRows: id yang BUKAN Kecamatan (provinsi
// level 1) → pgx.ErrNoRows, dipetakan pemanggil ke galat "district_id". Bukti
// filter level = 3 menolak jenjang salah, bukan diam-diam balikin kode provinsi.
func TestGetDistrictCode_NonDistrictErrNoRows(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := New(pool)

	provinces, err := q.ListProvinces(ctx)
	if err != nil || len(provinces) == 0 {
		t.Fatalf("list provinces: %v (len=%d)", err, len(provinces))
	}
	if _, err := q.GetDistrictCode(ctx, provinces[0].ID); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("GetDistrictCode(id provinsi) harus pgx.ErrNoRows, got %v", err)
	}
	// id yang tak ada sama sekali → juga ErrNoRows.
	if _, err := q.GetDistrictCode(ctx, 999999999); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("GetDistrictCode(id tak ada) harus pgx.ErrNoRows, got %v", err)
	}
}

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
