package handler

import (
	"context"
	"strconv"
	"strings"

	"go_starter/internal/db"
)

// accounts_import_resolve.go — BL-63: resolusi baris CSV ke master data.
// Dipisah dari accounts_import_parse.go (parse murni tanpa I/O) agar tiap
// file di bawah ambang Utility=200.

// resolveImportRows menyelesaikan referensi lintas-tabel untuk SEMUA baris
// sekaligus — TEPAT 3 query, terlepas dari jumlah baris (Rule 13, hindari
// N+1):
//  1. ListVillagesByCodes — village_code → region (id/nama/kecamatan)
//  2. ListAccountsByVillageCodes — village_code yang sudah dipakai akun lain
//  3. ListMembersByTenant — SATU kali; email_pemilik → user id dipetakan di
//     memori, bukan query per baris.
//
// Mengembalikan resolvedRow per baris input (urutan & jumlah SAMA dgn rows)
// plus anyFailed = true bila ADA SATU SAJA baris gagal — pemanggil (preview
// menampilkan status tiap baris; confirm membatalkan SELURUH insert, sesuai
// kebijakan all-or-nothing BL-63).
func resolveImportRows(ctx context.Context, h *Handler, tenantID int64, rows []importRow) ([]resolvedRow, bool) {
	codes := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.villageCode != "" {
			codes = append(codes, row.villageCode)
		}
	}

	villages, err := h.q(ctx).ListVillagesByCodes(ctx, codes)
	if err != nil {
		h.Log.Error("accounts import: resolve villages", "err", err)
		return failAllRows(rows, "failed"), true
	}
	villageByCode := make(map[string]db.ListVillagesByCodesRow, len(villages))
	for _, v := range villages {
		villageByCode[v.Code] = v
	}

	existing, err := h.q(ctx).ListAccountsByVillageCodes(ctx, db.ListAccountsByVillageCodesParams{
		TenantID: tenantID, Codes: codes,
	})
	if err != nil {
		h.Log.Error("accounts import: resolve existing accounts", "err", err)
		return failAllRows(rows, "failed"), true
	}
	dupCode := make(map[string]bool, len(existing))
	for _, a := range existing {
		if a.VillageCode != nil {
			dupCode[*a.VillageCode] = true
		}
	}

	members, err := h.q(ctx).ListMembersByTenant(ctx, tenantID)
	if err != nil {
		h.Log.Error("accounts import: resolve owners", "err", err)
		return failAllRows(rows, "failed"), true
	}
	ownerByEmail := make(map[string]int64, len(members))
	for _, m := range members {
		ownerByEmail[strings.ToLower(m.Email)] = m.UserID
	}

	seenCode := make(map[string]bool, len(rows)) // deteksi duplikat village_code DALAM file
	out := make([]resolvedRow, len(rows))
	anyFailed := false

	for i, row := range rows {
		rr := resolvedRow{rowNum: row.rowNum, villageCode: row.villageCode}

		reg, ok := db.ListVillagesByCodesRow{}, false
		switch {
		case row.villageCode == "":
			rr.errCode = "village_required"
		case dupCode[row.villageCode]:
			// Tabrakan vs akun HIDUP yang SUDAH ADA di tenant — beda kode dgn
			// tabrakan antar-baris dalam file yang sama (di bawah) walau efek
			// akhirnya sama (baris ditolak) agar pesannya bisa menuntun
			// operator ke penyebab yang benar.
			rr.errCode = "village_code_dup"
		case seenCode[row.villageCode]:
			rr.errCode = "village_code_dup_file"
		default:
			seenCode[row.villageCode] = true
			reg, ok = villageByCode[row.villageCode]
			if !ok {
				rr.errCode = "village_id"
			}
		}

		var ownerID *int64
		if rr.errCode == "" && row.ownerEmail != "" {
			uid, found := ownerByEmail[strings.ToLower(row.ownerEmail)]
			if !found {
				rr.errCode = "owner_email_notfound"
			} else {
				ownerID = &uid
			}
		}

		if rr.errCode == "" {
			fv := row.fields
			form, code := parseAccountForm(func(key string) string {
				if key == "village_id" {
					return strconv.FormatInt(reg.ID, 10)
				}
				return fv[key]
			})
			if code != "" {
				rr.errCode = code
			} else {
				rr.form = form
				rr.ownerID = ownerID
				rr.villageName = reg.Name
				rr.districtID = reg.ParentRegionID
			}
		}

		if rr.errCode != "" {
			anyFailed = true
		}
		out[i] = rr
	}

	return out, anyFailed
}

// failAllRows menandai SELURUH baris gagal dgn kode yang sama — dipakai saat
// query batch resolusi sendiri gagal (galat DB, bukan galat data baris).
func failAllRows(rows []importRow, code string) []resolvedRow {
	out := make([]resolvedRow, len(rows))
	for i, row := range rows {
		out[i] = resolvedRow{rowNum: row.rowNum, villageCode: row.villageCode, errCode: code}
	}
	return out
}
