package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// accounts_import_confirm.go — BL-63: AccountImportConfirm (POST
// /accounts/import/confirm). raw_csv datang dari FORM BIASA (bukan multipart
// lagi — file sudah jadi teks di langkah pratinjau), dan di sini
// parseImportCSV+resolveImportRows dipanggil ULANG dari nol — TIDAK percaya
// hasil pratinjau sebelumnya, menutup celah TOCTOU (mis. village_code dipakai
// akun lain di antara kedua request) by construction.
//
// Atomisitas all-or-nothing TANPA SAVEPOINT (pola sama LeadConvert,
// sales_convert_action.go): (1) SEMUA validasi selesai SEBELUM insert
// pertama — satu baris gagal berarti loop insert tak pernah mulai; (2) insert
// berurutan berhenti+redirect di galat DB PERTAMA — Postgres membatalkan
// SELURUH tx `Scope` pada galat query apa pun, jadi baris yang sudah ter-INSERT
// di request ini ikut batal saat commit.
func (h *Handler) AccountImportConfirm(w http.ResponseWriter, r *http.Request) {
	if !h.requireAccountWrite(w, r) {
		return
	}
	ctx := r.Context()

	rawCSV := r.FormValue("raw_csv")
	if strings.TrimSpace(rawCSV) == "" {
		wsRedirect(w, r, "/accounts/import", "import_empty")
		return
	}

	rows, code := parseImportCSV(strings.NewReader(rawCSV))
	if code != "" {
		wsRedirect(w, r, "/accounts/import", code)
		return
	}

	tenantID := session.TenantID(ctx)
	uid := session.UserID(ctx)

	resolved, anyFailed := resolveImportRows(ctx, h, tenantID, rows)
	if anyFailed {
		// Re-validasi confirm menemukan baris tak valid yang lolos di pratinjau
		// (TOCTOU — mis. village_code direbut import lain di antara kedua
		// request) — tolak SELURUH file, nol baris ditulis, minta ulang dari awal.
		wsRedirect(w, r, "/accounts/import", "import_row_invalid")
		return
	}

	created := 0
	for _, rr := range resolved {
		ownerID := uid
		if rr.ownerID != nil {
			ownerID = *rr.ownerID
		}

		entityCode, err := h.allocEntityCode(ctx, tenantID, rr.form.EntityCode)
		if err != nil {
			h.Log.Error("accounts import: alloc entity_code", "err", err)
			wsRedirect(w, r, "/accounts/import", "failed")
			return
		}

		vcode := rr.villageCode
		_, err = h.q(ctx).CreateAccount(ctx, db.CreateAccountParams{
			TenantID:              tenantID,
			EntityCode:            &entityCode,
			VillageName:           rr.villageName,
			VillageCode:           &vcode,
			AccountType:           rr.form.AccountType,
			AccountOwner:          &ownerID,
			Website:               rr.form.Website,
			Description:           rr.form.Description,
			DistrictID:            rr.districtID,
			VillageAddress:        rr.form.VillageAddress,
			PostalCode:            rr.form.PostalCode,
			Territory:             rr.form.Territory,
			VillageStatus:         rr.form.VillageStatus,
			VillageClassification: rr.form.VillageClassification,
			Population:            rr.form.Population,
			HamletsCount:          rr.form.HamletsCount,
			VillageBudget:         rr.form.VillageBudget,
			ContactPhone:          rr.form.ContactPhone,
			OfficePhone:           rr.form.OfficePhone,
			OfficeEmail:           rr.form.OfficeEmail,
			CreatedBy:             &uid,
		})
		if err != nil {
			if wcode, ok := accountWriteErr(err); ok {
				wsRedirect(w, r, "/accounts/import", wcode)
				return
			}
			h.Log.Error("accounts import: create", "err", err, "row", rr.rowNum)
			wsRedirect(w, r, "/accounts/import", "failed")
			return
		}
		created++
	}

	h.auditWorkspace(ctx, uid, "account.import", tenantID, map[string]string{
		"count": strconv.Itoa(created),
	})
	wsRedirectOK(w, r, "/accounts", "imported")
}
