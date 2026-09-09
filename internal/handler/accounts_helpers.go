package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// accounts_helpers.go — helper bersama jalur baca+tulis desa: muat-dengan-F3
// (loadOwnedAccount), prefill form (accountFormFields, int64PtrStr), dan opsi
// enum dropdown + cek sinkron compile-time. Tanpa handler HTTP sendiri.

// loadOwnedAccount memuat satu desa & menegakkan F3: di luar cakupan aktor → 404
// (kembaran AccountDetail; "tampil di daftar" & "boleh disentuh" satu jawaban).
// Mengembalikan (account, true) bila boleh, atau menulis 404/500 & (zero, false).
func (h *Handler) loadOwnedAccount(w http.ResponseWriter, r *http.Request, id int64) (db.Account, bool) {
	ctx := r.Context()
	a, err := h.q(ctx).GetAccount(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.Account{}, false
		}
		h.Log.Error("accounts: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.Account{}, false
	}
	filter := db.AccountsListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), a.AccountOwner, a.AssignedCsm, a.BackupCsm) {
		http.NotFound(w, r)
		return db.Account{}, false
	}
	return a, true
}

// villageIDForCode me-resolve village_code (Kemendagri) tersimpan → id master
// regions level 4, utk preselect dropdown Desa saat edit (BL-66). Kosong/legacy/
// tak cocok master (kode pra-BL-66 buatan sistem, atau NULL) → "" (dropdown Desa
// dibiarkan kosong; nama tersimpan tetap tampil sbg catatan di form). Soft-fail:
// resolusi hanya kenyamanan prefill, bukan syarat form bisa dibuka.
func (h *Handler) villageIDForCode(ctx context.Context, code *string) string {
	if code == nil || *code == "" {
		return ""
	}
	v, err := h.q(ctx).GetVillageByCode(ctx, *code)
	if err != nil {
		return ""
	}
	return strconv.FormatInt(v.ID, 10)
}

// int64PtrStr memformat *int64 → string ("" bila nil) untuk prefill dropdown.
func int64PtrStr(p *int64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatInt(*p, 10)
}

// accountFormFields memetakan Account → nilai prefill form. BL-106: HP Kontak
// account tak lagi ber-FLS — nomor asli selalu diisi apa adanya (tak ada lagi
// mask/kunci per-peran; itu tinggal di modul Kontak/Lead).
func accountFormFields(a db.Account) panel.AccountFormFields {
	return panel.AccountFormFields{
		VillageName:           a.VillageName,
		AccountType:           a.AccountType,
		Website:               deref(a.Website),
		Description:           deref(a.Description),
		DistrictID:            int64PtrStr(a.DistrictID),
		VillageAddress:        deref(a.VillageAddress),
		PostalCode:            deref(a.PostalCode),
		VillageStatus:         deref(a.VillageStatus),
		VillageClassification: deref(a.VillageClassification),
		Population:            int32Str(a.Population),
		HamletsCount:          int32Str(a.HamletsCount),
		// BL-60: prefill APBDes pakai moneyRupiahStr (tanpa ".00") — numgroup.js
		// membuang non-digit, jadi "750000000.00" dari numericStr akan salah
		// tampil "75000000000". Territory tak lagi punya field form (di-drop UI).
		VillageBudget: moneyRupiahStr(a.VillageBudget),
		ContactPhone:  deref(a.ContactPhone),
		OfficePhone:   deref(a.OfficePhone),
		OfficeEmail:   deref(a.OfficeEmail),
	}
}

// Opsi enum untuk dropdown — slice BERURUT (map validAccountTypes dll. tak
// berurutan). Nilai HARUS himpunan yang sama dengan map validasi & CHECK 00005;
// urutannya untuk tampilan.
var (
	accountTypeOptions    = []string{"prospect", "customer", "former_customer"}
	villageStatusOptions  = []string{"Desa", "Kelurahan", "Nagari", "Gampong"}
	classificationOptions = []string{
		"Mandiri", "Maju", "Berkembang", "Tertinggal", "Sangat Tertinggal",
	}
)

// compile-time: pastikan opsi & map validasi sepakat (panjang sama). Berbeda =
// dropdown menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(accountTypeOptions) != len(validAccountTypes) ||
		len(villageStatusOptions) != len(validVillageStatuses) ||
		len(classificationOptions) != len(validClassifications) {
		panic("accounts: opsi enum tak sinkron dengan map validasi")
	}
	return struct{}{}
}()
