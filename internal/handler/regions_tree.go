package handler

import (
	"encoding/json"
	"net/http"
)

// regions_tree.go — perluasan BL-163: endpoint lazy-fetch cascading
// Provinsi→Kabupaten/Kota→Kecamatan untuk tab "Wilayah" modal
// RegionSearchModal (static/regiontree.js). TIDAK meng-embed dataset (pola
// ListAllRegions ~7.817 baris di regionselect.go) karena modal ini dirender
// di SETIAP halaman via AppShell — embed penuh di sana jadi regresi payload
// global. Sama seperti AccountVillages (accounts_villages.go): same-origin
// JSON, lolos CSP default-src 'self'.

// RegionsTree — GET /w/{workspace}/regions/tree?level=1|2|3&parent=<id>.
// level=1 (Provinsi) tanpa parent; level=2 (Kabupaten/Kota)/level=3
// (Kecamatan) WAJIB parent int64 sah. Balas JSON array db.Region
// ({id,parent_region_id,level,code,name}, tanpa data sensitif) langsung,
// pola sama AccountVillages. TANPA gerbang Casbin tambahan — regions tabel
// GLOBAL tanpa tenant_id, chain dasar (RequireEnforce("user:home","read"))
// sudah cukup, sama alasan komentar r.Post("/regions/search") di routes.go.
func (h *Handler) RegionsTree(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	parentID, parentRaw := optInt64(r.URL.Query().Get("parent"))

	var rows any
	switch r.URL.Query().Get("level") {
	case "1":
		v, err := h.q(ctx).ListProvinces(ctx)
		if err != nil {
			h.Log.Error("regions: list provinces", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		rows = v
	case "2":
		if parentRaw != "" && parentID == nil {
			http.Error(w, "parameter parent tidak valid", http.StatusBadRequest)
			return
		}
		if parentID == nil {
			http.Error(w, "parameter parent wajib diisi", http.StatusBadRequest)
			return
		}
		v, err := h.q(ctx).ListRegenciesByProvince(ctx, parentID)
		if err != nil {
			h.Log.Error("regions: list regencies", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		rows = v
	case "3":
		if parentRaw != "" && parentID == nil {
			http.Error(w, "parameter parent tidak valid", http.StatusBadRequest)
			return
		}
		if parentID == nil {
			http.Error(w, "parameter parent wajib diisi", http.StatusBadRequest)
			return
		}
		v, err := h.q(ctx).ListDistrictsByRegency(ctx, parentID)
		if err != nil {
			h.Log.Error("regions: list districts", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		rows = v
	default:
		http.Error(w, "parameter level tidak valid (1, 2, atau 3)", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(rows); err != nil {
		// Header sudah terkirim; cukup catat — tak bisa lagi mengubah status.
		h.Log.Error("regions: encode tree", "err", err)
	}
}
