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

// TestListAllRegions_ThreeLevelsOnly membuktikan kontrak WHERE level <= 3:
// ListAllRegions dipakai SEKALI per render form utk embed JSON cascading
// dropdown (static/regions.js, ADR 0009) yang HANYA butuh Provinsi/Kabupaten-
// Kota/Kecamatan — level 4 (Desa/Kelurahan, ~83.762 baris) dilayani jalur
// LAZY-FETCH terpisah (ListVillagesByDistrict/data-villages-url) karena
// volumenya "terlalu besar utk diembed". Regresi query ini (kehilangan filter
// level) pernah membengkakkan payload form ~11x (91.599 vs ~7.837 baris) dan
// jadi penyebab form Lead/Account baru lambat dimuat lewat jalur jaringan
// lambat — lihat comment ListAllRegions di queries/regions.sql.
func TestListAllRegions_ThreeLevelsOnly(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	q := New(pool)

	rows, err := q.ListAllRegions(ctx)
	if err != nil {
		t.Fatalf("list all regions: %v", err)
	}
	// 34+ provinsi + ratusan kab/kota + ribuan kecamatan (00026) — cek batas
	// bawah wajar, bukan angka pas, agar tak rapuh thd update seed.
	if len(rows) < 7000 {
		t.Fatalf("harus >=7000 baris (3 level, cahyadsn/wilayah), got %d", len(rows))
	}
	var l1, l2, l3 int
	for _, r := range rows {
		switch r.Level {
		case 1:
			l1++
		case 2:
			l2++
		case 3:
			l3++
		default:
			t.Fatalf("ListAllRegions harus level<=3 saja, ada level=%d (%s) — filter level di regions.sql hilang?", r.Level, r.Name)
		}
	}
	if l1 == 0 || l2 == 0 || l3 == 0 {
		t.Fatalf("ketiga level harus terisi, got provinsi=%d kab/kota=%d kecamatan=%d", l1, l2, l3)
	}
}

// TestRegionsSeed_FourLevelsInDB membuktikan poin (1) di komentar package:
// keempat level (termasuk Desa/Kelurahan level 4, BL-66/migrasi 00039)
// benar-benar ter-seed dgn jumlah wajar di DB — dicek via query mentah
// (bukan ListAllRegions, yang sejak fix di atas SENGAJA tak lagi
// mengembalikan level 4).
func TestRegionsSeed_FourLevelsInDB(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()

	var l1, l2, l3, l4 int
	err := pool.QueryRow(ctx,
		`SELECT count(*) FILTER (WHERE level = 1),
		        count(*) FILTER (WHERE level = 2),
		        count(*) FILTER (WHERE level = 3),
		        count(*) FILTER (WHERE level = 4)
		 FROM regions`,
	).Scan(&l1, &l2, &l3, &l4)
	if err != nil {
		t.Fatalf("count per level: %v", err)
	}
	if l1 < 30 || l2 < 400 || l3 < 5000 || l4 < 70000 {
		t.Fatalf("jumlah per level di luar batas wajar seed cahyadsn/wilayah, got provinsi=%d kab/kota=%d kecamatan=%d desa=%d", l1, l2, l3, l4)
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

