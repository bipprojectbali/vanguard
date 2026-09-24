package handler

import (
	"context"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// contacts_import_resolve.go — BL-134: resolusi baris CSV kontak ke master
// data. Dipisah dari contacts_import_parse.go (parse murni tanpa I/O) agar
// tiap file di bawah ambang tipe Route/Handler (150). Deteksi duplikat klaim
// primary dipisah lagi ke contacts_import_primary.go agar file ini tetap di
// bawah ambang yang sama.

// contactImportAccountParams merakit flag ownership F3 (SAMA sumber dgn
// writableAccountSelectParams, contacts_global.go) + kode desa yang perlu
// diresolusi — dipakai SATU kali per file impor (bukan per baris).
func contactImportAccountParams(ctx context.Context, codes []string) db.ListAccountsByVillageCodesForContactImportParams {
	uid := session.UserID(ctx)
	f := db.AccountsListFilterFor(session.BusinessDataScope(ctx))
	return db.ListAccountsByVillageCodesForContactImportParams{
		ScopeAll:     f.ScopeAll,
		IsSales:      f.IsOwn,
		IsCsm:        f.IsOwn,
		Uid:          &uid,
		VillageCodes: codes,
	}
}

// resolveContactImportRows menyelesaikan referensi lintas-tabel untuk SEMUA
// baris sekaligus — TEPAT 2 query, terlepas dari jumlah baris (Rule 13,
// hindari N+1):
//  1. ListAccountsByVillageCodesForContactImport — village_code → akun (id +
//     nama desa) SEKALIGUS flag writable F3.
//  2. ListAccountsWithPrimaryContact — account_id mana yang SUDAH punya
//     kontak utama hidup (hanya dipanggil dgn akun yang lolos tahap 1, tetap
//     satu query batch).
//
// Duplikat klaim is_primary_contact DALAM FILE (>1 baris menandai primary
// utk account_id yang sama) dideteksi di memori (map), tak butuh query
// tambahan. Mengembalikan contactResolvedRow per baris input (urutan &
// jumlah SAMA dgn rows) plus anyFailed = true bila ADA SATU SAJA baris
// gagal — kebijakan all-or-nothing BL-134 (identik BL-63): confirm menolak
// SELURUH file bila anyFailed true.
func resolveContactImportRows(ctx context.Context, h *Handler, rows []contactImportRow) ([]contactResolvedRow, bool) {
	seen := make(map[string]bool, len(rows))
	codes := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.villageCode != "" && !seen[row.villageCode] {
			seen[row.villageCode] = true
			codes = append(codes, row.villageCode)
		}
	}

	accounts, err := h.q(ctx).ListAccountsByVillageCodesForContactImport(ctx, contactImportAccountParams(ctx, codes))
	if err != nil {
		h.Log.Error("contacts import: resolve accounts", "err", err)
		return failAllContactRows(rows, "failed"), true
	}
	accByCode := make(map[string]db.ListAccountsByVillageCodesForContactImportRow, len(accounts))
	for _, a := range accounts {
		if a.VillageCode != nil {
			accByCode[*a.VillageCode] = a
		}
	}

	out := make([]contactResolvedRow, len(rows))
	anyFailed := false

	for i, row := range rows {
		rr := contactResolvedRow{rowNum: row.rowNum, villageCode: row.villageCode}

		switch {
		case row.villageCode == "":
			rr.errCode = "village_required"
		default:
			acc, ok := accByCode[row.villageCode]
			switch {
			case !ok:
				rr.errCode = "contact_village_notfound"
			case !acc.Writable:
				rr.errCode = "contact_village_forbidden"
			default:
				rr.accountID = acc.ID
				rr.villageName = acc.VillageName
			}
		}

		if rr.errCode == "" {
			fv := row.fields
			if normalized, ok := normalizeContactPrimaryFlag(fv["is_primary_contact"]); !ok {
				rr.errCode = "contact_primary_invalid"
			} else {
				fv["is_primary_contact"] = normalized
				form, code := parseContactForm(func(key string) string { return fv[key] })
				if code != "" {
					rr.errCode = code
				} else {
					rr.form = form
				}
			}
		}

		out[i] = rr
	}

	out, failed := resolveContactPrimaryDuplicates(ctx, h, rows, out)
	if failed {
		return out, true
	}

	for _, rr := range out {
		if rr.errCode != "" {
			anyFailed = true
			break
		}
	}

	return out, anyFailed
}
