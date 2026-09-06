package main

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"go_starter/internal/db"
)

// regions.go — pemilihan kecamatan REAL + kolam Desa/Kelurahan REAL utk desa
// demo. `regions` sudah ter-seed migrasi 00026 (3 level: provinsi/kabupaten/
// kecamatan) + 00039 (level 4 Desa/Kelurahan Kemendagri asli). File ini HANYA
// baca (ListAllRegions/ListDistrictsByRegency/ListVillagesByDistrict), tak
// pernah insert. Provinsi dicari via strings.Contains nama (bukan ID hardcode)
// supaya portabel antar environment/re-seed data wilayah.

// district = satu kecamatan terpilih + label kabupaten/provinsi induknya
// (dipakai villagePool utk memuat Desa asli di bawahnya, dan sbg label
// deskriptif). BL-66: village_code kini kode Kemendagri ASLI dari Desa level 4
// (bukan segmen ke-4 fiktif), jadi kolam Desa-lah sumber kode — kecamatan hanya
// jalur menemukannya.
type district struct {
	ID       int64
	Name     string
	Regency  string
	Province string
}

// maxKecPerProvince membatasi berapa kecamatan diambil per provinsi target.
// >1 supaya kolam Desa cukup besar (satu kecamatan bisa cuma 3-5 desa; 40 akun
// + 3 hasil konversi demo butuh ≥43 Desa DISTINCT), tapi dibatasi agar tak
// memuat ribuan baris Desa ke memori seed.
const maxKecPerProvince = 3

// targetProvinces = ~8 provinsi tersebar (Jawa, Sumatera, Sulawesi, Bali,
// Nusa Tenggara) supaya data desa demo tak menumpuk di satu pulau.
var targetProvinces = []string{
	"jawa barat", "jawa tengah", "jawa timur",
	"sumatera barat", "sumatera utara",
	"sulawesi selatan", "bali", "nusa tenggara barat",
}

// pickDistricts memuat SELURUH regions sekali, lalu memilih hingga
// maxKecPerProvince kecamatan per provinsi target dari kabupaten pertama yang
// punya kecamatan — total ~8-24 kecamatan bergantung ketersediaan data.
func pickDistricts(ctx context.Context, q *db.Queries) ([]district, error) {
	rows, err := q.ListAllRegions(ctx)
	if err != nil {
		return nil, err
	}

	// provinsi (level 1, tanpa induk) yang namanya cocok target.
	provinces := map[string]db.ListAllRegionsRow{} // key = target lowercase
	for _, r := range rows {
		if r.Level != 1 {
			continue
		}
		low := strings.ToLower(r.Name)
		for _, t := range targetProvinces {
			if _, done := provinces[t]; done {
				continue
			}
			if strings.Contains(low, t) {
				provinces[t] = r
			}
		}
	}

	var result []district
	for _, t := range targetProvinces {
		prov, ok := provinces[t]
		if !ok {
			continue // provinsi tak ditemukan di data ini — lewati, jangan gagal seluruh run.
		}
		// kabupaten (level 2) pertama milik provinsi ini yang punya ≥1 kecamatan.
		for _, regency := range rows {
			if regency.Level != 2 || regency.ParentRegionID == nil || *regency.ParentRegionID != prov.ID {
				continue
			}
			kecs, err := q.ListDistrictsByRegency(ctx, &regency.ID)
			if err != nil {
				return nil, fmt.Errorf("kecamatan kabupaten %s: %w", regency.Name, err)
			}
			if len(kecs) == 0 {
				continue // kabupaten tanpa kecamatan — coba kabupaten berikutnya.
			}
			for i, kec := range kecs {
				if i >= maxKecPerProvince {
					break
				}
				result = append(result, district{
					ID:       kec.ID,
					Name:     kec.Name,
					Regency:  regency.Name,
					Province: prov.Name,
				})
			}
			break // cukup satu kabupaten per provinsi.
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("tak ada kecamatan ditemukan dari %d baris regions — cek data migrasi 00026", len(rows))
	}
	return result, nil
}

// village = satu Desa/Kelurahan REAL (regions level 4) yang dialokasikan ke satu
// account demo. Code = kode Kemendagri ASLI (segmen ke-4 nyata, mis.
// "32.01.01.2001") yang jadi village_code akun; DistrictID = kecamatan induk
// (FK accounts.district_id).
type village struct {
	ID         int64
	Code       string
	Name       string
	DistrictID int64
}

// villagePool memuat Desa REAL dari kecamatan terpilih (pickDistricts) sekali,
// mengacaknya, lalu membagikannya SATU-PER-SATU tanpa pengulangan (next) —
// supaya tiap account demo dapat Desa Kemendagri asli yang DISTINCT (BL-66:
// satu desa = satu akun). Menggantikan generator village_code fiktif lama.
type villagePool struct {
	items []village
	idx   int
}

// newVillagePool memuat semua Desa (level 4) di bawah kecamatan terpilih,
// menggabungkannya jadi satu kolam, lalu mengacaknya dgn rng pemanggil
// (deterministik per tag run).
func newVillagePool(ctx context.Context, q *db.Queries, rng *rand.Rand, districts []district) (*villagePool, error) {
	var items []village
	for _, d := range districts {
		id := d.ID
		desas, err := q.ListVillagesByDistrict(ctx, &id)
		if err != nil {
			return nil, fmt.Errorf("desa kecamatan %s: %w", d.Name, err)
		}
		for _, ds := range desas {
			items = append(items, village{ID: ds.ID, Code: ds.Code, Name: ds.Name, DistrictID: d.ID})
		}
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("tak ada Desa (regions level 4) di %d kecamatan terpilih — cek seed migrasi 00039", len(districts))
	}
	rng.Shuffle(len(items), func(i, j int) { items[i], items[j] = items[j], items[i] })
	return &villagePool{items: items}, nil
}

// next mengambil Desa berikutnya. Kolam habis → error (BUKAN reuse) supaya
// keunikan (tenant_id, village_code) tak dilanggar diam-diam; penyebab: kolam
// lebih kecil dari jumlah account+konversi demo (tambah maxKecPerProvince).
func (p *villagePool) next() (village, error) {
	if p.idx >= len(p.items) {
		return village{}, fmt.Errorf("kolam Desa habis (%d Desa) — kurangi jumlah account/lead demo atau naikkan maxKecPerProvince", len(p.items))
	}
	v := p.items[p.idx]
	p.idx++
	return v, nil
}
