package handler

import (
	"context"
	"encoding/json"
)

// regions_helpers.go — helper bersama master wilayah administratif (ADR 0009):
// embed JSON utk cascading dropdown client-side (regionsJSON), dan resolusi
// nama district_id → Kecamatan/Kabupaten-Kota/Provinsi TANPA N+1 di daftar
// (regionAncestryMap + regionNames, rule #13 — pola sama accountMemberNames:
// muat SEKALI per render daftar, bukan per baris). Dipakai accounts & leads
// (list/detail/form) + banner dupe konversi.

// regionsJSON mengambil SELURUH master wilayah (~7.817 baris, tabel GLOBAL
// tanpa tenant_id/RLS — lihat regions.sql) & marshal ke JSON siap-embed utk
// cascading dropdown (static/regions.js). Dipanggil SEKALI per render form
// (Account/Lead New&Edit, LeadConvertPage). Galat (jaring; mustahil utk tipe
// primitif) → "[]" (dropdown kosong, bukan 500 — form tetap bisa disimpan
// tanpa wilayah).
func (h *Handler) regionsJSON(ctx context.Context) string {
	rows, err := h.q(ctx).ListAllRegions(ctx)
	if err != nil {
		h.Log.Error("regions: list all", "err", err)
		return "[]"
	}
	b, err := json.Marshal(rows)
	if err != nil {
		h.Log.Error("regions: marshal", "err", err)
		return "[]"
	}
	return string(b)
}

// regionNode = simpul minimal (nama + parent) utk jalan mundur parent_region_id
// di memori — bagian dari regionAncestryMap, bukan dipakai sendiri.
type regionNode struct {
	name   string
	parent *int64
}

// regionAncestryMap memuat SELURUH master wilayah SEKALI (peta id→node), dipakai
// resolve nama BANYAK district_id sekaligus tanpa query tambahan per baris
// (rule #13). Cocok utk halaman DAFTAR (accounts_page.go: sampai ~20 baris/
// halaman); detail satu baris pakai GetRegionAncestry (1 query, bukan scan
// ~7.817 baris demi 1 hasil).
func (h *Handler) regionAncestryMap(ctx context.Context) (map[int64]regionNode, error) {
	rows, err := h.q(ctx).ListAllRegions(ctx)
	if err != nil {
		return nil, err
	}
	m := make(map[int64]regionNode, len(rows))
	for _, r := range rows {
		m[r.ID] = regionNode{name: r.Name, parent: r.ParentRegionID}
	}
	return m, nil
}

// regionNames resolve district_id opsional → (kecamatan, kabupaten/kota,
// provinsi) lewat peta ancestry yang sudah dimuat (regionAncestryMap) — TANPA
// query tambahan. nil, atau id tak ketemu di peta (data lama pra-migrasi/
// dihapus), → semua "".
func regionNames(m map[int64]regionNode, districtID *int64) (district, regency, province string) {
	if districtID == nil {
		return "", "", ""
	}
	d, ok := m[*districtID]
	if !ok {
		return "", "", ""
	}
	district = d.name
	if d.parent == nil {
		return district, "", ""
	}
	rgc, ok := m[*d.parent]
	if !ok {
		return district, "", ""
	}
	regency = rgc.name
	if rgc.parent == nil {
		return district, regency, ""
	}
	prov, ok := m[*rgc.parent]
	if !ok {
		return district, regency, ""
	}
	province = prov.name
	return district, regency, province
}

// regionLabel memformat district_id opsional → "Kecamatan, Kabupaten/Kota"
// singkat lewat SATU query (GetRegionAncestry) — cocok utk resolusi satuan
// (mis. banner dupe konversi, yang hanya butuh 1 label). nil / district_id
// sudah tak ada di master → "" (soft-fail, bukan gagal seluruh banner).
func (h *Handler) regionLabel(ctx context.Context, districtID *int64) string {
	if districtID == nil {
		return ""
	}
	a, err := h.q(ctx).GetRegionAncestry(ctx, *districtID)
	if err != nil {
		return ""
	}
	return a.DistrictName + ", " + a.RegencyName
}
