package handler

import (
	"context"
	"strconv"
	"strings"

	"go_starter/internal/db"
)

// sales_leads_import_warn.go — BL-133 follow-up: peringatan (non-blocking)
// utk baris pratinjau impor CSV Lead yang KOMBINASI lead_name + kecamatannya
// (via district_id hasil resolusi) SUDAH DIPAKAI BERSAMA oleh lead lain
// HIDUP di tenant ini — SATU peringatan gabungan, BUKAN dua peringatan
// independen (revisi 16 Sep: nama-sama-saja atau kecamatan-sama-saja saja
// TIDAK memicu apa pun; harus keduanya cocok pada lead lain yang SAMA).
// Mirror semangat findDuplicateVillages/FindDuplicateAccountsByNameRegion
// (M4-6, sales_convert_dupes.go) — SOFT-WARNING, BUKAN hard block: baik
// lead_name maupun district_id BUKAN kolom unik untuk Lead (banyak lead
// boleh berbagi nama kontak yang sama ATAU kecamatan yang sama, sendiri-
// sendiri — cuma KOMBINASI keduanya yang dianggap layak diberi tahu ke
// operator).
//
// SENGAJA dipanggil HANYA dari LeadImportPreview, TIDAK dari
// LeadImportConfirm: peringatan ini tak pernah menggagalkan commit (kebijakan
// all-or-nothing BL-133 hanya bereaksi ke errCode/anyFailed dari
// resolveLeadImportRows), jadi jalur TOCTOU re-validasi confirm tak perlu
// menghitung ulang query batch tambahan ini di setiap konfirmasi — kerja
// terbuang tanpa dampak (hasilnya tak pernah dibaca).
const leadImportWarnDup = "Nama lead ini sudah dipakai lead lain"

// leadImportRowWarnings mengembalikan pesan peringatan per baris (index-
// aligned dgn rows/resolved; "" = tak ada peringatan). HANYA baris yang lolos
// resolusi (rr.errCode == "") diberi peringatan — baris gagal sudah punya
// status error sendiri, ditambah peringatan duplikat hanya membingungkan.
// Galat query TAK menghalangi pratinjau tampil (soft-warning gagal dianggap
// "tak ada peringatan"), pola sama findDuplicateVillages.
func leadImportRowWarnings(ctx context.Context, h *Handler, tenantID int64, rows []leadImportRow, resolved []resolvedLeadRow) []string {
	out := make([]string, len(rows))

	// Kumpulkan kandidat nama & district_id (dedup) HANYA dari baris yang
	// sudah lolos resolusi (rr.errCode == "") — baris gagal tak punya
	// district_id yang valid utk dicocokkan.
	names := make([]string, 0, len(rows))
	seenName := map[string]bool{}
	districtIDs := make([]int64, 0, len(rows))
	seenDistrict := map[int64]bool{}
	for i, rr := range resolved {
		if rr.errCode != "" || rr.form.DistrictID == nil {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(rows[i].fields["lead_name"]))
		if name != "" && !seenName[name] {
			seenName[name] = true
			names = append(names, name)
		}
		id := *rr.form.DistrictID
		if !seenDistrict[id] {
			seenDistrict[id] = true
			districtIDs = append(districtIDs, id)
		}
	}
	if len(names) == 0 || len(districtIDs) == 0 {
		return out
	}

	// existingPairs = kombinasi (nama_ci, district_id) ASLI yang benar-benar
	// tersimpan bersama di satu lead lain hidup di tenant ini — bukan
	// cross-product hasil filter ANY() ganda (lihat komentar query).
	existing, err := h.q(ctx).ListLeadsByNameDistrictCI(ctx, db.ListLeadsByNameDistrictCIParams{
		TenantID: tenantID, Names: names, DistrictIds: districtIDs,
	})
	if err != nil {
		h.Log.Error("leads import: warn name+district dup", "err", err)
		return out
	}
	existingPairs := make(map[string]bool, len(existing))
	for _, e := range existing {
		if e.DistrictID != nil {
			existingPairs[pairKey(e.NameCi, *e.DistrictID)] = true
		}
	}

	for i, rr := range resolved {
		if rr.errCode != "" || rr.form.DistrictID == nil {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(rows[i].fields["lead_name"]))
		if name == "" {
			continue
		}
		if existingPairs[pairKey(name, *rr.form.DistrictID)] {
			out[i] = leadImportWarnDup
		}
	}

	return out
}

// pairKey membentuk kunci gabungan nama_ci+district_id — "\n" sbg pemisah
// (tak mungkin muncul dlm lead_name yg sudah di-trim single-line CSV) agar
// tak ambigu dgn nama yg kebetulan mengandung digit.
func pairKey(nameCI string, districtID int64) string {
	return nameCI + "\n" + strconv.FormatInt(districtID, 10)
}
