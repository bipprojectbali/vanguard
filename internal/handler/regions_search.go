package handler

import (
	"context"
	"net/http"
	"regexp"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/ui"

	"github.com/starfederation/datastar-go/datastar"
)

// regions_search.go — BL-163: modal global "Cari Kode Desa/Kecamatan". SSE
// fragment-reload (pola sama sales_activities_contact_options.go, BL-164):
// "q" dibaca dari QUERY STRING (bukan body form) agar tak terikat <form>
// terdekat, dipicu input ter-debounce di ui.RegionSearchModal. Akses = anggota
// workspace mana pun (chain RequireEnforce("user:home","read") di routes.go
// sudah cukup; regions GLOBAL tanpa tenant_id, tak perlu gerbang Casbin
// tambahan — keputusan desain BL-163).

// regionCodePattern mendeteksi ketikan berbentuk Kode Kemendagri (digit +
// titik, mis. "32.01.01" Kecamatan atau "32.01.01.2001" Desa) vs nama bebas —
// menentukan cabang query (SearchRegionsByCode vs SearchRegionsByName) sesuai
// keputusan desain BL-163.
var regionCodePattern = regexp.MustCompile(`^\d{1,2}(\.\d{1,4}){1,3}$`)

// RegionSearch — POST /w/{slug}/regions/search?q=.... Ketikan < RegionSearchMinChars
// → fragmen "belum cukup ketikan" TANPA query DB (guard server-side; client
// sudah skip @post di bawah panjang ini, ini pertahanan kedua bila dilewati).
//
// Perluasan BL-163: ?district=<id> (dari select Kecamatan tab "Wilayah",
// internal/ui/region_search.go regionSearchTreePanel) mem-BYPASS jalur "q" —
// cabang district DICEK LEBIH DULU sebab keduanya bisa muncul di request
// yang sama secara teori (query string lama tersisa) tapi hanya satu tab yang
// bisa aktif di client; district = sinyal eksplisit "user memilih Kecamatan",
// diprioritaskan drpd sisa "q" basi.
func (h *Handler) RegionSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	districtID, districtRaw := optInt64(r.URL.Query().Get("district"))

	var rows []ui.RegionSearchRow
	emptyHint := ""
	switch {
	case districtRaw != "" && districtID == nil:
		// district terisi tapi bukan int64 sah → 400 (bukan fallback diam-diam
		// ke jalur q, supaya kesalahan client/manipulasi kelihatan jelas).
		http.Error(w, "parameter district tidak valid", http.StatusBadRequest)
		return
	case districtID != nil:
		var err error
		rows, err = h.searchRegionsByDistrict(ctx, *districtID)
		if err != nil {
			h.Log.Error("regions: search by district", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if len(rows) == 0 {
			emptyHint = ui.RegionSearchHintNoResults
		}
	default:
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if len([]rune(q)) >= ui.RegionSearchMinChars {
			var err error
			if regionCodePattern.MatchString(q) {
				rows, err = h.searchRegionsByCode(ctx, q)
			} else {
				rows, err = h.searchRegionsByName(ctx, q)
			}
			if err != nil {
				h.Log.Error("regions: search", "err", err)
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			if len(rows) == 0 {
				emptyHint = ui.RegionSearchHintNoResults
			}
		}
	}

	var buf strings.Builder
	if err := ui.RegionSearchResults(rows, emptyHint).Render(&buf); err != nil {
		h.Log.Error("regions: render results fragment", "err", err)
		return
	}
	sse := datastar.NewSSE(w, r)
	_ = sse.PatchElements(buf.String())
}

// searchRegionsByCode = cabang kode PERSIS (level 3 Kecamatan ATAU level 4
// Desa) — lihat queries/regions.sql SearchRegionsByCode.
func (h *Handler) searchRegionsByCode(ctx context.Context, q string) ([]ui.RegionSearchRow, error) {
	dbRows, err := h.q(ctx).SearchRegionsByCode(ctx, db.SearchRegionsByCodeParams{
		Code: q, PageSize: int32(pageSize),
	})
	if err != nil {
		return nil, err
	}
	return regionRowsFromCode(dbRows), nil
}

// searchRegionsByName = cabang ILIKE nama, level 3+4 dicampur satu daftar
// (keputusan desain BL-163). % dan _ di ketikan user DI-ESCAPE (bukan
// disaring/ditolak) agar wildcard ILIKE bawaan Postgres tak bocor dari
// ketikan bebas (mis. user cari "50%" secara harfiah, bukan pola).
func (h *Handler) searchRegionsByName(ctx context.Context, q string) ([]ui.RegionSearchRow, error) {
	pattern := "%" + escapeLikePattern(q) + "%"
	dbRows, err := h.q(ctx).SearchRegionsByName(ctx, db.SearchRegionsByNameParams{
		Pattern: pattern, PageSize: int32(pageSize),
	})
	if err != nil {
		return nil, err
	}
	return regionRowsFromName(dbRows), nil
}

func escapeLikePattern(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

func regionRowsFromCode(dbRows []db.SearchRegionsByCodeRow) []ui.RegionSearchRow {
	out := make([]ui.RegionSearchRow, len(dbRows))
	for i, r := range dbRows {
		out[i] = ui.RegionSearchRow{
			Code: r.Code, Desa: r.VillageName, Kecamatan: r.DistrictName,
			Kabupaten: r.RegencyName, Provinsi: r.ProvinceName,
		}
	}
	return out
}

func regionRowsFromName(dbRows []db.SearchRegionsByNameRow) []ui.RegionSearchRow {
	out := make([]ui.RegionSearchRow, len(dbRows))
	for i, r := range dbRows {
		out[i] = ui.RegionSearchRow{
			Code: r.Code, Desa: r.VillageName, Kecamatan: r.DistrictName,
			Kabupaten: r.RegencyName, Provinsi: r.ProvinceName,
		}
	}
	return out
}
