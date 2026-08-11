package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/authz"
	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// accounts.go — AKSI atas desa (Account): form buat/sunting, create, update,
// penugasan CSM, soft-delete. Dipisah dari accounts_page.go (baca): aksi tumbuh
// dengan aturan TULIS (F2 write), halaman dengan aturan LIHAT (F2 read + F3).
//
// Gerbang SEMUA aksi = CanBusiness("crm:accounts","write") — support (read saja)
// DITOLAK menulis. Ownership F3 TIDAK menjaga tulis di M2-3: siapa pun pemegang
// peran tulis boleh membuat/menyunting desa mana pun di workspace-nya (batas
// per-baris untuk edit menyusul bila dibutuhkan). RLS tetap mengurung workspace.

// sqlStateUniqueViolation = kode Postgres untuk pelanggaran UNIQUE. Dipakai
// mengenali tabrakan village_code (idx_accounts_code) → pesan spesifik, bukan
// "internal error" yang menyembunyikan sebab yang bisa diperbaiki user.
const sqlStateUniqueViolation = "23505"

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
	base := wsPath(slugFromRequest(r), "")
	v := panel.AccountFormView{
		Base:            base,
		Action:          base + "/accounts",
		IsEdit:          false,
		Err:             wsErrMsg(r.URL.Query().Get("err")),
		PhoneEditable:   canEditPhone(r.Context()),
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
		Province:              form.Province,
		Regency:               form.Regency,
		District:              form.District,
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

// AccountEdit — GET /w/{workspace}/accounts/{id}/edit. Form terisi. Mensyaratkan
// F3: hanya yang boleh MELIHAT baris yang boleh membuka form suntingnya (404 bila
// di luar cakupan — kembaran AccountDetail).
func (h *Handler) AccountEdit(w http.ResponseWriter, r *http.Request) {
	if !h.requireAccountWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	a, ok := h.loadOwnedAccount(w, r, id)
	if !ok {
		return
	}

	base := wsPath(slugFromRequest(r), "")
	idStr := strconv.FormatInt(a.ID, 10)
	members, err := h.assignableMembers(ctx)
	if err != nil {
		h.Log.Error("accounts: members", "err", err)
		wsRedirect(w, r, "/accounts/"+idStr, "failed")
		return
	}

	v := panel.AccountFormView{
		Base:            base,
		Action:          base + "/accounts/" + idStr,
		IsEdit:          true,
		Err:             wsErrMsg(r.URL.Query().Get("err")),
		PhoneEditable:   canEditPhone(ctx),
		Fields:          accountFormFields(a, canEditPhone(ctx)),
		Types:           accountTypeOptions,
		Statuses:        villageStatusOptions,
		Classifications: classificationOptions,
		AssignAction:    base + "/accounts/" + idStr + "/assign",
		Members:         members,
		AssignedCSM:     int64PtrStr(a.AssignedCsm),
		BackupCSM:       int64PtrStr(a.BackupCsm),
	}
	h.renderWorkspaceShell(w, r, "Sunting Desa", "/accounts", panel.AccountForm(v))
}

// AccountUpdate — POST /w/{workspace}/accounts/{id}. Menyimpan sunting profil.
// entity_code/village_code/owner/CSM TIDAK disentuh di sini (jalur terpisah).
func (h *Handler) AccountUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.requireAccountWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	a, ok := h.loadOwnedAccount(w, r, id)
	if !ok {
		return
	}

	form, errCode := parseAccountForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/accounts/"+strconv.FormatInt(id, 10)+"/edit", errCode)
		return
	}

	// F4: editor bukan-Sales tak mengirim contact_phone (field terkunci) — nilai
	// tersamar TAK boleh menimpa nomor asli. Pertahankan yang tersimpan. Sales
	// mengirim nilai apa adanya (termasuk pengosongan yang disengaja).
	contactPhone := form.ContactPhone
	if !canEditPhone(ctx) {
		contactPhone = a.ContactPhone
	}

	uid := session.UserID(ctx)
	if _, err := h.q(ctx).UpdateAccount(ctx, db.UpdateAccountParams{
		VillageName:           form.VillageName,
		AccountType:           form.AccountType,
		Website:               form.Website,
		Description:           form.Description,
		Province:              form.Province,
		Regency:               form.Regency,
		District:              form.District,
		VillageAddress:        form.VillageAddress,
		PostalCode:            form.PostalCode,
		Territory:             form.Territory,
		VillageStatus:         form.VillageStatus,
		VillageClassification: form.VillageClassification,
		Population:            form.Population,
		HamletsCount:          form.HamletsCount,
		VillageBudget:         form.VillageBudget,
		ContactPhone:          contactPhone,
		OfficePhone:           form.OfficePhone,
		OfficeEmail:           form.OfficeEmail,
		UpdatedBy:             &uid,
		ID:                    id,
	}); err != nil {
		if code, ok := accountWriteErr(err); ok {
			wsRedirect(w, r, "/accounts/"+strconv.FormatInt(id, 10)+"/edit", code)
			return
		}
		h.Log.Error("accounts: update", "err", err)
		wsRedirect(w, r, "/accounts/"+strconv.FormatInt(id, 10)+"/edit", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "account.update", session.TenantID(ctx), map[string]string{
		"account_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/accounts/"+strconv.FormatInt(id, 10), "saved")
}

// AccountAssign — POST /w/{workspace}/accounts/{id}/assign. Menetapkan CSM utama
// & cadangan. Kandidat WAJIB anggota workspace (dicek GetMembership) — memasang
// non-anggota membuat baris yang tak bisa dilihat siapa pun lewat F3.
func (h *Handler) AccountAssign(w http.ResponseWriter, r *http.Request) {
	if !h.requireAccountWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	// F3 gate: hanya yang boleh menyentuh baris ini yang boleh menugaskan CSM-nya.
	if _, ok := h.loadOwnedAccount(w, r, id); !ok {
		return
	}

	assigned, code := h.parseMemberRef(ctx, r.FormValue("assigned_csm"))
	if code != "" {
		wsRedirect(w, r, "/accounts/"+strconv.FormatInt(id, 10)+"/edit", code)
		return
	}
	backup, code := h.parseMemberRef(ctx, r.FormValue("backup_csm"))
	if code != "" {
		wsRedirect(w, r, "/accounts/"+strconv.FormatInt(id, 10)+"/edit", code)
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).AssignAccountCSM(ctx, db.AssignAccountCSMParams{
		AssignedCsm: assigned,
		BackupCsm:   backup,
		UpdatedBy:   &uid,
		ID:          id,
	}); err != nil {
		h.Log.Error("accounts: assign", "err", err)
		wsRedirect(w, r, "/accounts/"+strconv.FormatInt(id, 10)+"/edit", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "account.assign", session.TenantID(ctx), map[string]string{
		"account_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/accounts/"+strconv.FormatInt(id, 10), "assigned")
}

// AccountDelete — POST /w/{workspace}/accounts/{id}/delete. Soft-delete.
// Reversibel di DB (deleted_at), tapi tak ada UI restore di M2-3 — jejak audit
// mencatat siapa & kapan.
func (h *Handler) AccountDelete(w http.ResponseWriter, r *http.Request) {
	if !h.requireAccountWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadOwnedAccount(w, r, id); !ok {
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).SoftDeleteAccount(ctx, db.SoftDeleteAccountParams{
		UpdatedBy: &uid, ID: id,
	}); err != nil {
		h.Log.Error("accounts: delete", "err", err)
		wsRedirect(w, r, "/accounts/"+strconv.FormatInt(id, 10), "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "account.delete", session.TenantID(ctx), map[string]string{
		"account_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/accounts", "deleted")
}

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

// parseMemberRef menerjemahkan nilai form dropdown CSM: "" → (nil, "") (lepas
// tugas); angka → validasi keanggotaan → (&id, "") atau (nil, "csm") bila bukan
// anggota; tak terurai → (nil, "csm"). Non-anggota ditolak agar tak ada baris
// yatim yang tak terlihat siapa pun.
func (h *Handler) parseMemberRef(ctx context.Context, raw string) (*int64, string) {
	if raw == "" {
		return nil, ""
	}
	uid, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, "csm"
	}
	if _, err := h.q(ctx).GetMembership(ctx, db.GetMembershipParams{
		UserID: uid, TenantID: session.TenantID(ctx),
	}); err != nil {
		return nil, "csm"
	}
	return &uid, ""
}

// assignableMembers = kandidat penerima tugas CSM (semua anggota workspace).
// Label = nama bila ada, jatuh ke email — penanda orang, bukan id telanjang.
func (h *Handler) assignableMembers(ctx context.Context) ([]panel.AccountMemberOption, error) {
	rows, err := h.q(ctx).ListMembersByTenant(ctx, session.TenantID(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]panel.AccountMemberOption, 0, len(rows))
	for _, m := range rows {
		label := m.Email
		if m.Name != nil && *m.Name != "" {
			label = *m.Name
		}
		out = append(out, panel.AccountMemberOption{ID: m.UserID, Label: label})
	}
	return out, nil
}

// accountWriteErr mengenali galat DB yang bisa diperbaiki user → kode pesan.
// Hanya tabrakan village_code (UNIQUE) yang dikenali; sisanya "internal".
func accountWriteErr(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == sqlStateUniqueViolation {
		if pgErr.ConstraintName == "idx_accounts_code" {
			return "village_code_dup", true
		}
	}
	return "", false
}

// canEditPhone = boleh menyunting nomor HP kontak (F4: Sales saja lihat penuh,
// jadi hanya Sales boleh menyuntingnya — selain itu formnya mengirim mask).
func canEditPhone(ctx context.Context) bool {
	return session.BusinessRole(ctx) == authz.BusinessRoleSales
}

// int64PtrStr memformat *int64 → string ("" bila nil) untuk prefill dropdown.
func int64PtrStr(p *int64) string {
	if p == nil {
		return ""
	}
	return strconv.FormatInt(*p, 10)
}

// accountFormFields memetakan Account → nilai prefill form. phoneEditable
// menentukan apakah nomor asli atau mask yang ditaruh di field (F4): editor
// non-Sales menerima mask, bukan nomor asli — nilai asli tak pernah mencapai
// browsernya (view-source pun bersih).
func accountFormFields(a db.Account, phoneEditable bool) panel.AccountFormFields {
	phone := deref(a.ContactPhone)
	if !phoneEditable {
		phone = flsHidden // tersamar; field dikunci di view, tak ikut ter-submit
	}
	return panel.AccountFormFields{
		VillageName:           a.VillageName,
		VillageCode:           deref(a.VillageCode),
		AccountType:           a.AccountType,
		Website:               deref(a.Website),
		Description:           deref(a.Description),
		Province:              deref(a.Province),
		Regency:               deref(a.Regency),
		District:              deref(a.District),
		VillageAddress:        deref(a.VillageAddress),
		PostalCode:            deref(a.PostalCode),
		Territory:             deref(a.Territory),
		VillageStatus:         deref(a.VillageStatus),
		VillageClassification: deref(a.VillageClassification),
		Population:            int32Str(a.Population),
		HamletsCount:          int32Str(a.HamletsCount),
		VillageBudget:         numericStr(a.VillageBudget),
		ContactPhone:          phone,
		OfficePhone:           deref(a.OfficePhone),
		OfficeEmail:           deref(a.OfficeEmail),
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
