package handler

import "context"

// contacts_import_primary.go — resolveContactPrimaryDuplicates (deteksi
// duplikat klaim is_primary_contact, dalam-file & vs-DB). Dipisah dari
// resolveContactImportRows (contacts_import_resolve.go) agar file itu di
// bawah ambang tipe Route/Handler (150).

// resolveContactPrimaryDuplicates menandai baris yang klaim is_primary_contact-
// nya kalah lewat dua tahap, memutasi out di tempat:
//
//  1. Duplikat klaim DALAM FILE: >1 baris menandai is_primary_contact utk
//     account_id yang sama → SEMUA baris terkait ditolak (bukan hanya baris
//     ke-2 dst.) — operator tak dibiarkan menebak mana yang "menang".
//  2. Klaim primary vs primary yang SUDAH hidup di DB — hanya dicek utk akun
//     yang masih punya SATU klaim (tahap 1 sudah menyingkirkan yang >1), satu
//     query batch terlepas dari jumlahnya.
//
// Mengembalikan out (mutasi in-place, dikembalikan untuk kejelasan pemanggil)
// + true bila query batch tahap 2 gagal — pemanggil WAJIB menggantinya dgn
// failAllContactRows(rows, "failed") sendiri, sama seperti galat query lain
// di resolveContactImportRows.
func resolveContactPrimaryDuplicates(ctx context.Context, h *Handler, rows []contactImportRow, out []contactResolvedRow) ([]contactResolvedRow, bool) {
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

	return out, false
}
