package handler

import (
	"context"

	"go_starter/internal/db"
	"go_starter/internal/ui"
)

// regions_search_district.go — cabang tab "Wilayah" (perluasan BL-163) dari
// RegionSearch (regions_search.go). Dipisah file sendiri karena
// regions_search.go sudah dekat batas 150 baris (CLAUDE.md §8 file health);
// tanggung jawabnya tetap sama (satu cabang pencarian modal region), hanya
// dipindah supaya tiap file tak melebihi limit.

// searchRegionsByDistrict = SATU Kecamatan terpilih (district_id dari select
// level 3 tab "Wilayah") + semua Desa anaknya. Lihat queries/regions.sql
// SearchRegionsByDistrict.
func (h *Handler) searchRegionsByDistrict(ctx context.Context, districtID int64) ([]ui.RegionSearchRow, error) {
	dbRows, err := h.q(ctx).SearchRegionsByDistrict(ctx, db.SearchRegionsByDistrictParams{
		DistrictID: districtID, PageSize: int32(pageSize),
	})
	if err != nil {
		return nil, err
	}
	return regionRowsFromDistrict(dbRows), nil
}

func regionRowsFromDistrict(dbRows []db.SearchRegionsByDistrictRow) []ui.RegionSearchRow {
	out := make([]ui.RegionSearchRow, len(dbRows))
	for i, r := range dbRows {
		out[i] = ui.RegionSearchRow{
			Code: r.Code, Desa: r.VillageName, Kecamatan: r.DistrictName,
			Kabupaten: r.RegencyName, Provinsi: r.ProvinceName,
		}
	}
	return out
}
