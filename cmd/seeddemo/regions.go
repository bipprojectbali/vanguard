package main

import (
	"context"
	"fmt"
	"strings"

	"go_starter/internal/db"
)

// regions.go — pemilihan kecamatan REAL utk desa demo. `regions` sudah
// ter-seed migrasi 00026 (~7.817 baris 3 level: provinsi/kabupaten/
// kecamatan) — file ini HANYA baca (ListAllRegions/ListDistrictsByRegency),
// tak pernah insert. Provinsi dicari via strings.Contains nama (bukan ID
// hardcode) supaya portabel antar environment/re-seed data wilayah.

// district = satu kecamatan terpilih + label kabupaten/provinsi induknya
// (dipakai accounts.go utk kolom deskriptif & district_id FK). Code = kode
// wilayah Kemendagri ASLI kecamatan ini (mis. "32.01.01", 3 segmen —
// provinsi.kabupaten.kecamatan, lihat migrations/00026_crm_regions.sql) —
// dipakai accounts.go/leads.go sbg 3 segmen depan village_code pseudo-resmi
// ("32.01.01.2001"); segmen ke-4 (desa) TETAP fiktif krn desa demo tak
// bertaut ke desa Kemendagri sungguhan.
type district struct {
	ID       int64
	Name     string
	Regency  string
	Province string
	Code     string
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
			// Kecamatan via query terpisah (bukan filter `rows`) — ListAllRegions
			// SENGAJA tak sertakan kolom `code` (payload embed dropdown form
			// produksi, lihat queries/regions.sql), sedangkan ListDistrictsByRegency
			// pakai `SELECT *` jadi punya Code (kode Kemendagri asli) yang
			// dibutuhkan utk format village_code pseudo-resmi.
			kecs, err := q.ListDistrictsByRegency(ctx, &regency.ID)
			if err != nil {
				return nil, fmt.Errorf("kecamatan kabupaten %s: %w", regency.Name, err)
			}
			if len(kecs) == 0 {
				continue
			}
			kec := kecs[0]
			result = append(result, district{
				ID:       kec.ID,
				Name:     kec.Name,
				Regency:  regency.Name,
				Province: prov.Name,
				Code:     kec.Code,
			})
			found = true
		}
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("tak ada kecamatan ditemukan dari %d baris regions — cek data migrasi 00026", len(rows))
	}
	return result, nil
}
