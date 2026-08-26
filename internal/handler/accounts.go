package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgconn"
)

// accounts.go — AKSI TULIS atas desa (Account): gerbang tulis, form buat, create.
// Sunting/update di accounts_update.go, penugasan CSM & soft-delete di
// accounts_assign.go, helper muat+prefill di accounts_helpers.go. Dipisah dari
// accounts_page.go (baca): aksi tumbuh dengan aturan TULIS (F2 write), halaman
// dengan aturan LIHAT (F2 read + F3).
//
// Gerbang SEMUA aksi = CanBusiness("crm:accounts","write") — support (read saja)
// DITOLAK menulis. Ownership F3 TIDAK menjaga tulis di M2-3: siapa pun pemegang
// peran tulis boleh membuat/menyunting desa mana pun di workspace-nya (batas
// per-baris untuk edit menyusul bila dibutuhkan). RLS tetap mengurung workspace.

// sqlStateUniqueViolation/sqlStateForeignKeyViolation = kode Postgres yang
// dikenali khusus. UNIQUE → tabrakan village_code (idx_accounts_code); FK →
// district_id yang dikirim klien sudah tak ada di master regions (mis. race
// dgn migrasi data, atau payload dipalsukan) — keduanya pesan spesifik, bukan
// "internal error" yang menyembunyikan sebab yang bisa diperbaiki user.
const (
	sqlStateUniqueViolation     = "23505"
	sqlStateForeignKeyViolation = "23503"
)

// requireAccountWrite = gerbang tulis bersama. Mengembalikan false & menulis
// respons penolakan bila aktor tak berhak. Read-only workspace (arsip) juga
// ditolak di sini: gerbang lifecycle memblokir POST, tapi menolak lebih awal
// dengan pesan jelas lebih baik daripada 403 telanjang dari middleware.
func (h *Handler) requireAccountWrite(w http.ResponseWriter, r *http.Request) bool {
	ctx := r.Context()
	if !canWriteAccountsPerm(ctx) {
		h.renderAccountsForbidden(w, r)
		return false
	}
	return true
}

// AccountNew — GET /w/{workspace}/accounts/new. Form kosong untuk membuat desa.
func (h *Handler) AccountNew(w http.ResponseWriter, r *http.Request) {
	if !h.requireAccountWrite(w, r) {
		return
	}
	ctx := r.Context()
	base := wsPath(slugFromRequest(r), "")
	v := panel.AccountFormView{
		Base:            base,
		Action:          base + "/accounts",
		IsEdit:          false,
		Err:             wsErrMsg(r.URL.Query().Get("err")),
		RegionsJSON:     h.regionsJSON(ctx),
		PhoneEditable:   canEditPhone(ctx),
		Types:           accountTypeOptions,
		Statuses:        villageStatusOptions,
		Classifications: classificationOptions,
	}
	h.renderWorkspaceShell(w, r, "Tambah Desa", "/accounts", panel.AccountForm(v))
}

// AccountCreate — POST /w/{workspace}/accounts. Membuat desa. entity_code
// dialokasikan DALAM tx ber-tenant yang sama (h.q), jadi create-nya atomik:
// nomor tak pernah terpakai untuk baris yang gagal disimpan.
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

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	code, err := h.q(ctx).GenerateEntityCode(ctx, tenantID, codes.EntityAccount)
	if err != nil {
		h.Log.Error("accounts: generate code", "err", err)
		wsRedirect(w, r, "/accounts/new", "failed")
		return
	}

	a, err := h.q(ctx).CreateAccount(ctx, db.CreateAccountParams{
		TenantID:              tenantID,
		EntityCode:            &code,
		VillageName:           form.VillageName,
		VillageCode:           form.VillageCode,
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
	case pgErr.Code == sqlStateForeignKeyViolation && pgErr.ConstraintName == "accounts_district_id_fkey":
		return "district_id", true
	}
	return "", false
}
