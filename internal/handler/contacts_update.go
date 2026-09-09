package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// contacts_update.go — jalur SUNTING kontak: form terisi (ContactEdit) + simpan
// (ContactUpdate). Dipisah dari jalur buat (contacts.go) karena sunting membawa
// dua invarian tambahan: F3 diwarisi desa induk (loadOwnedContact → 404) dan F4
// (nomor pribadi non-Sales tak boleh ditimpa mask).

// ContactEdit — GET /w/{workspace}/accounts/{id}/contacts/{contactID}/edit. Form
// terisi. F3 diwarisi (loadOwnedContact → 404 di luar cakupan).
func (h *Handler) ContactEdit(w http.ResponseWriter, r *http.Request) {
	if !h.requireContactWrite(w, r) {
		return
	}
	ctx := r.Context()
	accountID, contactID, ok := h.parseContactRef(w, r)
	if !ok {
		return
	}
	c, ok := h.loadOwnedContact(w, r, accountID, contactID)
	if !ok {
		return
	}
	account, ok := h.loadOwnedAccount(w, r, accountID)
	if !ok {
		return
	}

	base := wsPath(slugFromRequest(r), "")
	accountBase := base + "/accounts/" + strconv.FormatInt(accountID, 10)
	h.renderWorkspaceShell(w, r, "Sunting Kontak", "/accounts", panel.ContactForm(panel.ContactFormView{
		AccountBase: accountBase,
		AccountName: account.VillageName,
		// BL-117: Action WAJIB ter-prefiks /w/{slug} (pakai accountBase) — form
		// dirender langsung, jadi butuh path lengkap. contactPath() workspace-RELATIF
		// (dipakai wsRedirectOK yang menambah prefiks); dipasang mentah di sini → di
		// luar grup route /w/{slug} → submit 404. Sejajar form CREATE (contacts.go).
		Action:        accountBase + "/contacts/" + strconv.FormatInt(contactID, 10),
		IsEdit:        true,
		Err:           wsErrMsg(r.URL.Query().Get("err")),
		PhoneEditable: canEditPhone(ctx),
		Fields:        contactFormFields(ctx, c),
		Positions:     contactPositionOptions,
		Roles:         contactRoleOptions,
		Channels:      contactChannelOptions,
	}))
}

// ContactUpdate — POST /w/{workspace}/accounts/{id}/contacts/{contactID}. Menyimpan
// sunting. account_id TIDAK diubah (memindahkan kontak antar-desa aksi lain).
func (h *Handler) ContactUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.requireContactWrite(w, r) {
		return
	}
	ctx := r.Context()
	accountID, contactID, ok := h.parseContactRef(w, r)
	if !ok {
		return
	}
	c, ok := h.loadOwnedContact(w, r, accountID, contactID)
	if !ok {
		return
	}

	form, errCode := parseContactForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, contactEditRel(accountID, contactID), errCode)
		return
	}

	uid := session.UserID(ctx)

	// F4: editor bukan-Sales tak mengirim nomor HP/WhatsApp (field terkunci) —
	// nilai tersamar TAK boleh menimpa nomor asli. Pertahankan yang tersimpan.
	mobile, whatsapp := form.MobilePhone, form.WhatsappNumber
	if !canEditPhone(ctx) {
		mobile, whatsapp = c.MobilePhone, c.WhatsappNumber
	}

	// Set-primary dua langkah: bila dinaikkan jadi utama, kosongkan yang lama dulu
	// (satu tx). Bila kontak INI sudah utama, ClearAccount... juga mengosongkannya
	// lalu UPDATE memasangnya kembali — tetap benar (nol perubahan bersih).
	if form.IsPrimaryContact {
		if err := h.q(ctx).ClearAccountPrimaryContact(ctx, db.ClearAccountPrimaryContactParams{
			UpdatedBy: &uid, AccountID: accountID,
		}); err != nil {
			h.Log.Error("contacts: clear primary", "err", err)
			wsRedirect(w, r, contactEditRel(accountID, contactID), "failed")
			return
		}
	}

	if _, err := h.q(ctx).UpdateContact(ctx, db.UpdateContactParams{
		ContactOwner:       c.ContactOwner,
		ReportsToID:        c.ReportsToID,
		FirstName:          form.FirstName,
		LastName:           form.LastName,
		Salutation:         form.Salutation,
		JobTitle:           form.JobTitle,
		PositionCategory:   form.PositionCategory,
		ContactRole:        form.ContactRole,
		IsPrimaryContact:   form.IsPrimaryContact,
		IsTechnicalContact: form.IsTechnicalContact,
		TermPeriod:         form.TermPeriod,
		MobilePhone:        mobile,
		WhatsappNumber:     whatsapp,
		OfficePhone:        form.OfficePhone,
		Email:              form.Email,
		PreferredChannel:   form.PreferredChannel,
		MailingAddress:     form.MailingAddress,
		City:               form.City,
		PostalCode:         form.PostalCode,
		EmailOptOut:        form.EmailOptOut,
		DoNotContact:       form.DoNotContact,
		UpdatedBy:          &uid,
		ID:                 contactID,
	}); err != nil {
		h.Log.Error("contacts: update", "err", err)
		wsRedirect(w, r, contactEditRel(accountID, contactID), "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "contact.update", session.TenantID(ctx), map[string]string{
		"account_id": strconv.FormatInt(accountID, 10),
		"contact_id": strconv.FormatInt(contactID, 10),
	})
	wsRedirectOK(w, r, contactPath(accountID, contactID), "saved")
}
