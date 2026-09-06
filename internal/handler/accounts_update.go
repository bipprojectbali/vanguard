package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// accounts_update.go — sunting profil desa: form terisi (AccountEdit) + simpan
// (AccountUpdate). entity_code/village_code/owner/CSM TIDAK disentuh di jalur ini
// (penugasan CSM di accounts_assign.go). canEditPhone menegakkan F4: hanya Sales
// (yang melihat nomor penuh) boleh menyuntingnya.

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

	fields := accountFormFields(a, canEditPhone(ctx))
	// BL-66: preselect dropdown Desa dari village_code tersimpan (resolve → id
	// master). Legacy/kode tak cocok → "" (dropdown kosong; nama tetap tampil
	// sbg catatan di view).
	fields.VillageID = h.villageIDForCode(ctx, a.VillageCode)

	v := panel.AccountFormView{
		Base:            base,
		Action:          base + "/accounts/" + idStr,
		IsEdit:          true,
		Err:             wsErrMsg(r.URL.Query().Get("err")),
		RegionsJSON:     h.regionsJSON(ctx),
		VillagesURL:     base + "/accounts/villages",
		PhoneEditable:   canEditPhone(ctx),
		Fields:          fields,
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

	// BL-66: Desa/Kelurahan (village_id) opsional saat edit. Terpilih → turunkan
	// nama, village_code (Kemendagri), & district_id dari master (satu sumber).
	// Kosong → pertahankan nilai tersimpan (desa legacy yang dropdown-nya tak bisa
	// preselect tak boleh kehilangan datanya) — pola sama dgn Teritori/contactPhone.
	villageName := a.VillageName
	villageCode := a.VillageCode
	districtID := a.DistrictID
	if form.VillageID != nil {
		reg, err := h.q(ctx).GetVillageRegion(ctx, *form.VillageID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				wsRedirect(w, r, "/accounts/"+strconv.FormatInt(id, 10)+"/edit", "village_id")
				return
			}
			h.Log.Error("accounts: get village region", "err", err)
			wsRedirect(w, r, "/accounts/"+strconv.FormatInt(id, 10)+"/edit", "failed")
			return
		}
		c := reg.Code
		villageName = reg.Name
		villageCode = &c
		districtID = reg.ParentRegionID
	}

	// BL-60: field Teritori dilepas dari UI → form tak lagi mengirimnya
	// (form.Territory selalu nil). Pertahankan nilai tersimpan agar edit profil
	// tak menghapus data teritori lama (pola sama dgn contactPhone di atas).
	uid := session.UserID(ctx)
	if _, err := h.q(ctx).UpdateAccount(ctx, db.UpdateAccountParams{
		VillageName:           villageName,
		VillageCode:           villageCode,
		AccountType:           form.AccountType,
		Website:               form.Website,
		Description:           form.Description,
		DistrictID:            districtID,
		VillageAddress:        form.VillageAddress,
		PostalCode:            form.PostalCode,
		Territory:             a.Territory,
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

// canEditPhone = boleh menyunting nomor HP kontak (F4: Sales saja lihat penuh,
// jadi hanya Sales boleh menyuntingnya — selain itu formnya mengirim mask).
func canEditPhone(ctx context.Context) bool {
	return session.BusinessRole(ctx) == authz.BusinessRoleSales
}
