package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5/pgconn"
)

// accounts_create.go — aksi AccountCreate + penerjemah galat tulis
// (accountWriteErr: pelanggaran keunikan kode desa → pesan form). Dipisah dari
// accounts.go (const + guard tulis + AccountNew form) agar keduanya di bawah
// ambang tipe Route/Handler (150).
// AccountCreate — POST /w/{workspace}/accounts. Membuat desa. KEDUA kode
// (village_code Kemendagri otomatis + entity_code sistem otomatis/override)
// dialokasikan DALAM tx ber-tenant yang sama (h.q) — lihat accounts_codes.go.
func (h *Handler) AccountCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requireAccountWrite(w, r) {
		return
	}
	ctx := r.Context()
	form, errCode := parseAccountForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/accounts/new", errCode)
		return
	}

	// Kecamatan WAJIB di create (dijaga di sini, bukan di parseAccountForm yang
	// dipakai bersama update — baris lama tanpa district tetap boleh disunting):
	// village_code otomatis diturunkan darinya, tak bisa dirakit tanpa Kecamatan.
	if form.DistrictID == nil {
		wsRedirect(w, r, "/accounts/new", "district_required")
		return
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	// village_code Kemendagri OTOMATIS dari Kecamatan (validasi Kecamatan sah
	// terjadi di sini: errInvalidDistrict → "district_id"). Didahulukan sebelum
	// entity_code agar district palsu tak sempat memajukan counter kode sistem.
	vcode, err := h.generateVillageCode(ctx, tenantID, *form.DistrictID)
	if err != nil {
		if errors.Is(err, errInvalidDistrict) {
			wsRedirect(w, r, "/accounts/new", "district_id")
			return
		}
		h.Log.Error("accounts: generate village_code", "err", err)
		wsRedirect(w, r, "/accounts/new", "failed")
		return
	}

	// entity_code sistem: otomatis (form.EntityCode == nil) atau override manual.
	code, err := h.allocEntityCode(ctx, tenantID, form.EntityCode)
	if err != nil {
		h.Log.Error("accounts: alloc entity_code", "err", err)
		wsRedirect(w, r, "/accounts/new", "failed")
		return
	}

	a, err := h.q(ctx).CreateAccount(ctx, db.CreateAccountParams{
		TenantID:              tenantID,
		EntityCode:            &code,
		VillageName:           form.VillageName,
		VillageCode:           &vcode,
		AccountType:           form.AccountType,
		AccountOwner:          &uid, // pembuat = pemilik awal (dasar F3 ScopeOwn)
		Website:               form.Website,
		Description:           form.Description,
		DistrictID:            form.DistrictID,
		VillageAddress:        form.VillageAddress,
		PostalCode:            form.PostalCode,
		Territory:             form.Territory,
		VillageStatus:         form.VillageStatus,
		VillageClassification: form.VillageClassification,
		Population:            form.Population,
		HamletsCount:          form.HamletsCount,
		VillageBudget:         form.VillageBudget,
		ContactPhone:          form.ContactPhone,
		OfficePhone:           form.OfficePhone,
		OfficeEmail:           form.OfficeEmail,
		CreatedBy:             &uid,
	})
	if err != nil {
		if code, ok := accountWriteErr(err); ok {
			wsRedirect(w, r, "/accounts/new", code)
			return
		}
		h.Log.Error("accounts: create", "err", err)
		wsRedirect(w, r, "/accounts/new", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "account.create", tenantID, map[string]string{
		"account_id": strconv.FormatInt(a.ID, 10), "code": code,
	})
	wsRedirectOK(w, r, "/accounts/"+strconv.FormatInt(a.ID, 10), "created")
}

// accountWriteErr mengenali galat DB yang bisa diperbaiki user → kode pesan.
// Tabrakan village_code (UNIQUE) & district_id tak valid (FK) dikenali;
// sisanya "internal".
func accountWriteErr(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return "", false
	}
	switch {
	case pgErr.Code == sqlStateUniqueViolation && pgErr.ConstraintName == "idx_accounts_code":
		return "village_code_dup", true
	case pgErr.Code == sqlStateUniqueViolation && pgErr.ConstraintName == "idx_accounts_entity_code":
		return "entity_code_dup", true
	case pgErr.Code == sqlStateForeignKeyViolation && pgErr.ConstraintName == "accounts_district_id_fkey":
		return "district_id", true
	}
	return "", false
}
