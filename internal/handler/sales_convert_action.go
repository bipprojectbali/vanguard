package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// sales_convert_action.go — AKSI atomik LeadConvert (INTI Modul 4): konversi
// Lead Qualified → Desa+Kontak+Deal dalam SATU tx ber-tenant, dipisah dari
// sales_convert.go semata untuk file health (fungsi ini SENGAJA dibiarkan
// utuh, tak dipecah di tengah tx). Tx Scope-middleware SELALU commit, tapi
// Postgres membatalkan seluruh tx pada galat query APA PUN → all-or-nothing
// tanpa savepoint (mekanisme sama "nomor tak terbakar" di AccountCreate);
// galat pertama log + wsRedirect ?err= + return. Guard: hanya lead Qualified
// & belum converted (dicek sebelum tx); MarkLeadConverted juga menjaga WHERE.

// LeadConvert — POST /w/{workspace}/leads/{id}/convert. Konversi dalam satu tx atomik; sukses → /deals/{newID}?ok=converted.
func (h *Handler) LeadConvert(w http.ResponseWriter, r *http.Request) {
	if !h.requireLeadWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	l, ok := h.loadOwnedLead(w, r, id)
	if !ok {
		return
	}
	idStr := strconv.FormatInt(id, 10)
	if l.LeadStatus != "Qualified" || l.Converted {
		wsRedirect(w, r, "/leads/"+idStr, "convert_guard")
		return
	}

	form, errCode := parseConvertForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/leads/"+idStr+"/convert", errCode)
		return
	}
	// F4: aktor tanpa akses nomor penuh kirim mask (field dikunci) — salin nomor asli lead server-side, bukan mask.
	if !canEditPhone(ctx) {
		form.MobilePhone = l.MobilePhone
		form.Whatsapp = l.Whatsapp
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)
	// Pemilik entitas hasil = pemilik lead (manajer yang mengonversi tak diam-diam mengambil alih); fallback aktor bila lead tak berpemilik.
	owner := l.LeadOwner
	if owner == nil {
		owner = &uid
	}
	convertErr := "/leads/" + idStr + "/convert"

	// --- SATU tx ber-tenant (h.q ambient): gagal-sebagian = rollback penuh. ---
	accountCode, err := h.q(ctx).GenerateEntityCode(ctx, tenantID, codes.EntityAccount)
	if err != nil {
		h.Log.Error("convert: account code", "err", err)
		wsRedirect(w, r, convertErr, "failed")
		return
	}
	acc, err := h.q(ctx).CreateAccount(ctx, db.CreateAccountParams{
		TenantID:     tenantID,
		EntityCode:   &accountCode,
		VillageName:  form.VillageName,
		AccountType:  form.AccountType,
		AccountOwner: owner,
		DistrictID:   form.DistrictID,
		CreatedBy:    &uid,
	})
	if err != nil {
		if code, ok := accountWriteErr(err); ok {
			wsRedirect(w, r, convertErr, code)
			return
		}
		h.Log.Error("convert: account", "err", err)
		wsRedirect(w, r, convertErr, "failed")
		return
	}

	contact, err := h.q(ctx).CreateContact(ctx, db.CreateContactParams{
		TenantID:         tenantID,
		AccountID:        acc.ID,
		ContactOwner:     owner,
		FirstName:        form.FirstName,
		LastName:         form.LastName,
		JobTitle:         form.JobTitle,
		IsPrimaryContact: true, // kontak hasil konversi = kontak utama desa baru.
		MobilePhone:      form.MobilePhone,
		WhatsappNumber:   form.Whatsapp,
		Email:            form.Email,
		CreatedBy:        &uid,
	})
	if err != nil {
		h.Log.Error("convert: contact", "err", err)
		wsRedirect(w, r, convertErr, "failed")
		return
	}

	dealCode, err := h.q(ctx).GenerateEntityCode(ctx, tenantID, codes.EntityDeal)
	if err != nil {
		h.Log.Error("convert: deal code", "err", err)
		wsRedirect(w, r, convertErr, "failed")
		return
	}
	deal, err := h.q(ctx).CreateDeal(ctx, db.CreateDealParams{
		TenantID:         tenantID,
		EntityCode:       &dealCode,
		DealName:         form.DealName,
		AccountID:        acc.ID,
		DealOwner:        owner,
		PrimaryContactID: &contact.ID,
		Stage:            "Prospecting", // deal lahir di awal pipeline.
		Amount:           form.Amount,
		CreatedBy:        &uid,
	})
	if err != nil {
		h.Log.Error("convert: deal", "err", err)
		wsRedirect(w, r, convertErr, "failed")
		return
	}

	if err := h.q(ctx).MarkLeadConverted(ctx, db.MarkLeadConvertedParams{
		ConvertedAccountID: &acc.ID,
		ConvertedContactID: &contact.ID,
		ConvertedDealID:    &deal.ID,
		UpdatedBy:          &uid,
		ID:                 id,
	}); err != nil {
		h.Log.Error("convert: mark", "err", err)
		wsRedirect(w, r, convertErr, "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "lead.convert", tenantID, map[string]string{
		"lead_id":    idStr,
		"account_id": strconv.FormatInt(acc.ID, 10),
		"contact_id": strconv.FormatInt(contact.ID, 10),
		"deal_id":    strconv.FormatInt(deal.ID, 10),
	})
	wsRedirectOK(w, r, "/deals/"+strconv.FormatInt(deal.ID, 10), "converted")
}
