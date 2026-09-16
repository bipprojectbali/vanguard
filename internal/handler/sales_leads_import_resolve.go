package handler

import (
	"context"
	"strconv"
)

// sales_leads_import_resolve.go — BL-133: resolusi baris CSV ke master data.
// Dipisah dari sales_leads_import_parse.go (parse murni tanpa I/O) agar tiap
// file di bawah ambang Utility=200 (pola sama BL-63).
//
// LEBIH SEDERHANA dari resolveImportRows milik Account (accounts_import_resolve.go):
// Lead tak punya constraint unik terkait wilayah (banyak lead boleh berbagi
// district_id yang sama, beda dgn village_code 1 desa = 1 akun), dan tak ada
// resolusi owner-by-email (LeadOwner/CreatedBy SELALU = importer, lihat
// sales_leads_import_confirm.go) — jadi HANYA SATU query batch diperlukan.
//
// *** KEPUTUSAN FLS BL-133 (SENGAJA, BUKAN BUG) ***
// Fungsi ini TIDAK PERNAH memanggil canEditPhone(ctx) (internal/handler/fls.go)
// — beda dgn jalur form manual LeadNew/LeadEdit yang MEMANGGILnya utk
// mengunci mobile_phone bagi role yang tak berhak (BL-84/BL-107). "Keputusan
// final FLS (14 Sep)" di docs/crm/tasks.md BL-133: importer yang punya
// crm:leads write SELALU boleh menulis HP/WhatsApp dari CSV, terlepas dari
// business_role atau konfigurasi Field Security tenant. Perilaku ini
// TERCAPAI BY OMISSION — parseLeadForm sendiri tak pernah memanggil
// canEditPhone (baca sales_leads_form.go), jadi cukup TIDAK menaruh
// pemanggilan itu di sini/di sales_leads_import_confirm.go agar HP/WhatsApp
// CSV lolos apa adanya. JANGAN tambahkan canEditPhone di jalur impor ini —
// itu akan jadi REGRESI dari keputusan yang sudah didiskusikan & dikunci
// (BL-63/133/134, 14 Sep), bukan perbaikan.
//
// Catatan kolom (penyesuaian pasca-rilis, diminta user): CSV tak lagi punya
// kolom "lead_source" (tak ada padanan di form manual) maupun "whatsapp"
// terpisah — "mobile_phone" satu-satunya kolom nomor, MENGIKUTI form manual
// Tambah/Edit Lead yang sudah menggabung HP & WhatsApp jadi satu input
// ("HP / WhatsApp", lihat sales_leads_form.go). parseLeadForm dgn sendirinya
// membiarkan Whatsapp NULL saat fv("whatsapp") tak ada di baris (map kosong
// mengembalikan string kosong) — SAMA seperti LeadCreate manual, bukan celah.
func resolveLeadImportRows(ctx context.Context, h *Handler, rows []leadImportRow) ([]resolvedLeadRow, bool) {
	codes := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.districtCode != "" {
			codes = append(codes, row.districtCode)
		}
	}

	districts, err := h.q(ctx).ListDistrictsByCodes(ctx, codes)
	if err != nil {
		h.Log.Error("leads import: resolve districts", "err", err)
		return failAllLeadRows(rows, "failed"), true
	}
	districtByCode := make(map[string]struct {
		id   int64
		name string
	}, len(districts))
	for _, d := range districts {
		districtByCode[d.Code] = struct {
			id   int64
			name string
		}{id: d.ID, name: d.Name}
	}

	out := make([]resolvedLeadRow, len(rows))
	anyFailed := false

	for i, row := range rows {
		rr := resolvedLeadRow{rowNum: row.rowNum, districtCode: row.districtCode}

		switch {
		case row.districtCode == "":
			rr.errCode = "district_required"
		default:
			d, ok := districtByCode[row.districtCode]
			if !ok {
				rr.errCode = "district_id"
			} else {
				rr.districtName = d.name
				fv := row.fields
				form, code := parseLeadForm(func(key string) string {
					if key == "district_id" {
						return strconv.FormatInt(d.id, 10)
					}
					return fv[key]
				})
				if code != "" {
					rr.errCode = code
				} else {
					rr.form = form
				}
			}
		}

		if rr.errCode != "" {
			anyFailed = true
		}
		out[i] = rr
	}

	return out, anyFailed
}

// failAllLeadRows menandai SELURUH baris gagal dgn kode yang sama — dipakai
// saat query batch resolusi sendiri gagal (galat DB, bukan galat data baris).
func failAllLeadRows(rows []leadImportRow, code string) []resolvedLeadRow {
	out := make([]resolvedLeadRow, len(rows))
	for i, row := range rows {
		out[i] = resolvedLeadRow{rowNum: row.rowNum, districtCode: row.districtCode, errCode: code}
	}
	return out
}
