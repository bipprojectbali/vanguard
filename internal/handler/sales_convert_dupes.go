package handler

import (
	"context"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// sales_convert_dupes.go — cek kandidat desa duplikat SAAT REVIEW konversi lead
// (M4-6 follow-up). Dipisah dari sales_convert.go (sudah di atas limit rule 8,
// hanya wiring 1-baris ditambah di sana) alih-alih menumpuk lagi ke file itu.
//
// KEPUTUSAN EKSPLISIT USER (2026-08-13): cakupan HANYA saat konversi (bukan
// create-account manual maupun create-lead), match EXACT case-insensitive pada
// village_name (regency opsional ikut menyaring bila terisi), perilaku
// SOFT-WARNING — banner info di halaman review, TETAP bisa lanjut konversi.
// Nama desa sama bisa valid utk beda dusun/kabupaten, jadi ini bukan hard block.

// findDuplicateVillages mencari kandidat desa dgn nama sama di tenant yang sama
// via FindDuplicateAccountsByNameRegion. Sejak ADR 0009, districtID exact match
// (bukan lagi fuzzy teks regency) — FK menghapus kebutuhan bandingan teks sama
// sekali. Galat query TAK menghalangi halaman review (soft-warning, bukan
// gerbang) — dicatat lalu dianggap "tak ada kandidat".
func (h *Handler) findDuplicateVillages(ctx context.Context, tenantID int64, villageName string, districtID *int64) []panel.DuplicateCandidate {
	rows, err := h.q(ctx).FindDuplicateAccountsByNameRegion(ctx, db.FindDuplicateAccountsByNameRegionParams{
		TenantID:    tenantID,
		VillageName: villageName,
		DistrictID:  districtID,
	})
	if err != nil {
		h.Log.Error("convert: find duplicate villages", "err", err)
		return nil
	}
	out := make([]panel.DuplicateCandidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, panel.DuplicateCandidate{
			AccountID:   row.ID,
			EntityCode:  deref(row.EntityCode),
			VillageName: row.VillageName,
			RegionLabel: h.regionLabel(ctx, row.DistrictID),
		})
	}
	return out
}
