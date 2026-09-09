package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
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
	}
	// HP & WhatsApp digabung jadi satu field UI (mobile_phone) — form konversi tak lagi
	// menyertakan WhatsApp; salin nomor WA lead apa adanya ke kontak baru (data tak hilang).
	form.Whatsapp = l.Whatsapp

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)
	// Pemilik entitas hasil = pemilik lead (manajer yang mengonversi tak diam-diam mengambil alih); fallback aktor bila lead tak berpemilik.
	owner := l.LeadOwner
	if owner == nil {
		owner = &uid
	}
	convertErr := "/leads/" + idStr + "/convert"

	// BL-67: desa hasil konversi diambil dari master Kemendagri (regions level 4)
	// — SAMA seperti AccountCreate langsung — agar akun hasil convert punya
	// village_code Kemendagri asli & tunduk aturan "satu desa hidup = satu akun".
	// WAJIB: konversi hanya saat lead Qualified (convert_guard) → desa sudah pasti;
	// nil → village_required. Desa tak dikenal (id palsu / bukan level 4) →
	// "village_id".
	if form.VillageID == nil {
		wsRedirect(w, r, convertErr, "village_required")
		return
	}
	reg, err := h.q(ctx).GetVillageRegion(ctx, *form.VillageID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			wsRedirect(w, r, convertErr, "village_id")
			return
		}
		h.Log.Error("convert: get village region", "err", err)
		wsRedirect(w, r, convertErr, "failed")
		return
	}
	vcode := reg.Code

	// Blokir (bukan soft-warning nama): desa ini sudah punya akun HIDUP di
	// workspace → tolak konversi & tautkan operator ke akun eksisting (dup=<id>,
	// dirender jadi <a> di halaman review). Pre-check lewat SELECT SEBELUM INSERT
	// karena konversi = satu tx atomik — pelanggaran UNIQUE (idx_accounts_code)
	// akan meracuni seluruh tx (pola sama VillageCodeExists). Unique index tetap
	// jaring balapan (accountWriteErr, di bawah).
	if existing, derr := h.q(ctx).GetAccountByVillageCode(ctx, db.GetAccountByVillageCodeParams{
		TenantID:    tenantID,
		VillageCode: &vcode,
	}); derr == nil {
		wsRedirect(w, r, convertErr, "village_code_dup&dup="+strconv.FormatInt(existing.ID, 10))
		return
	} else if !errors.Is(derr, pgx.ErrNoRows) {
		h.Log.Error("convert: check village dup", "err", derr)
		wsRedirect(w, r, convertErr, "failed")
		return
	}

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
		VillageName:  reg.Name, // BL-67: nama dari master Desa, bukan input teks
		VillageCode:  &vcode,   // BL-67: kode Kemendagri asli (kini tak lagi NULL)
		AccountType:  form.AccountType,
		AccountOwner: owner,
		DistrictID:   reg.ParentRegionID, // BL-67: Kecamatan induk Desa dari master
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
