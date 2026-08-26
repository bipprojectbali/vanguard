package main

import (
	"context"
	"fmt"
	"strings"

	"go_starter/internal/db"
)

// regions.go — pemilihan kecamatan REAL utk desa demo. `regions` sudah
// ter-seed migrasi 00026 (~7.817 baris 3 level: provinsi/kabupaten/
// kecamatan) — file ini HANYA baca (ListAllRegions), tak pernah insert.
// Provinsi dicari via strings.Contains nama (bukan ID hardcode) supaya
// portabel antar environment/re-seed data wilayah.

// district = satu kecamatan terpilih + label kabupaten/provinsi induknya
// (dipakai accounts.go utk kolom deskriptif & district_id FK).
type district struct {
	ID       int64
	Name     string
	Regency  string
	Province string
}

// targetProvinces = ~8 provinsi tersebar (Jawa, Sumatera, Sulawesi, Bali,
// Nusa Tenggara) supaya data desa demo tak menumpuk di satu pulau.
var targetProvinces = []string{
	"jawa barat", "jawa tengah", "jawa timur",
	"sumatera barat", "sumatera utara",
	"sulawesi selatan", "bali", "nusa tenggara barat",
}

// pickDistricts memuat SELURUH regions sekali, lalu memilih tepat satu
// kecamatan per provinsi target (total ~8-10) yang benar-benar ada relasi
// kabupaten→provinsi-nya di data.
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
		found := false
		// kabupaten (level 2) pertama milik provinsi ini yang punya ≥1 kecamatan.
		for _, regency := range rows {
			if found || regency.Level != 2 || regency.ParentRegionID == nil || *regency.ParentRegionID != prov.ID {
				continue
			}
			for _, kec := range rows {
				if kec.Level == 3 && kec.ParentRegionID != nil && *kec.ParentRegionID == regency.ID {
					result = append(result, district{
						ID:       kec.ID,
						Name:     kec.Name,
						Regency:  regency.Name,
						Province: prov.Name,
					})
					found = true
					break
				}
			}
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("tak ada kecamatan ditemukan dari %d baris regions — cek data migrasi 00026", len(rows))
	}
	return result, nil
}
