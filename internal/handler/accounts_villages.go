package handler

import (
	"encoding/json"
	"net/http"
)

// accounts_villages.go — endpoint lazy-fetch daftar Desa/Kelurahan per Kecamatan
// (BL-66). Dataset Desa (~83.762 baris master regions level 4) TERLALU besar
// untuk diembed di RegionsJSON (pola 3-level ADR 0009); dropdown level 4 di form
// desa memuatnya on-demand via fetch (static/regions.js). Same-origin JSON →
// lolos CSP default-src 'self'. READ-only: hanya SELECT master global (tanpa
// tenant_id/RLS), tak menyentuh data tenant.

// AccountVillages — GET /w/{workspace}/accounts/villages?district=<id>. Balas
// JSON [{id,code,name}] Desa di bawah Kecamatan itu, terurut nama. Master global
// READ-only → digerbang izin tulis Desa ATAU tulis Lead: dropdown Desa dipakai
// form buat/sunting desa (crm:accounts write) DAN halaman konversi Lead (BL-67,
// crm:leads write). district tak sah → 400; kosong hasil = array kosong (bukan
// error).
func (h *Handler) AccountVillages(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteAccountsPerm(ctx) && !canWriteLeadsPerm(ctx) {
		h.renderAccountsForbidden(w, r)
		return
	}
	did, code := optInt64(r.URL.Query().Get("district"))
	if code != "" || did == nil {
		http.Error(w, "parameter district tidak valid", http.StatusBadRequest)
		return
	}

	rows, err := h.q(ctx).ListVillagesByDistrict(ctx, did)
	if err != nil {
		h.Log.Error("accounts: list villages by district", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(rows); err != nil {
		// Header sudah terkirim; cukup catat — tak bisa lagi mengubah status.
		h.Log.Error("accounts: encode villages", "err", err)
	}
}
