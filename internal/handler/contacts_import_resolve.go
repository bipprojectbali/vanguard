package handler

import (
	"context"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// contacts_import_resolve.go — BL-134: resolusi baris CSV kontak ke master
// data. Dipisah dari contacts_import_parse.go (parse murni tanpa I/O) agar
// tiap file di bawah ambang Utility=200.

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

	// Duplikat klaim primary DALAM FILE: >1 baris menandai is_primary_contact
	// utk account_id yang sama → SEMUA baris terkait ditolak (bukan hanya
	// baris ke-2 dst.) — operator tak dibiarkan menebak mana yang "menang".
	primaryRowsByAccount := make(map[int64][]int)
	for i, rr := range out {
		if rr.errCode == "" && rr.form.IsPrimaryContact {
			primaryRowsByAccount[rr.accountID] = append(primaryRowsByAccount[rr.accountID], i)
		}
	}
	for accountID, idxs := range primaryRowsByAccount {
		if len(idxs) > 1 {
			for _, i := range idxs {
				out[i].errCode = "contact_primary_dup_file"
			}
			delete(primaryRowsByAccount, accountID)
		}
	}

	// Klaim primary vs primary yang SUDAH hidup di DB — hanya dicek utk akun
	// yang masih punya SATU klaim (dup-dalam-file di atas sudah menyingkirkan
	// yang >1), satu query batch terlepas dari jumlahnya.
	if len(primaryRowsByAccount) > 0 {
		accountIDs := make([]int64, 0, len(primaryRowsByAccount))
		for accountID := range primaryRowsByAccount {
			accountIDs = append(accountIDs, accountID)
		}
		existing, err := h.q(ctx).ListAccountsWithPrimaryContact(ctx, accountIDs)
		if err != nil {
			h.Log.Error("contacts import: resolve existing primary", "err", err)
			return failAllContactRows(rows, "failed"), true
		}
		for _, accountID := range existing {
			for _, i := range primaryRowsByAccount[accountID] {
				out[i].errCode = "contact_primary_exists"
			}
		}
	}

	for _, rr := range out {
		if rr.errCode != "" {
			anyFailed = true
			break
		}
	}

	return out, anyFailed
}
