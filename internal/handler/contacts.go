package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// contacts.go — AKSI atas kontak (orang di dalam sebuah desa): form buat/sunting,
// create, update, set-primary, soft-delete. Dipisah dari contacts_page.go (baca):
// aksi tumbuh dengan aturan TULIS (F2 write), halaman dengan aturan LIHAT.
//
// Gerbang SEMUA aksi = CanBusiness("crm:contacts","write") — support (read saja)
// DITOLAK. F3 diwarisi DESA INDUK: aktor yang tak boleh menyentuh desa (404 via
// loadOwnedAccount) tak pernah bisa mengubah kontaknya. RLS mengurung workspace.
//
// PRIMARY dua langkah dalam satu tx (h.q sudah tx dari Scope): kosongkan primary
// lama (ClearAccountPrimaryContact) SEBELUM memasang yang baru — idx_contacts_primary
// (partial UNIQUE, maks 1 utama per desa) memblokir dua utama sekaligus.

// requireContactWrite = gerbang tulis bersama. Mengembalikan false & menulis
// respons penolakan bila aktor tak berhak (F2 write). Read-only workspace (arsip)
// juga ditolak lebih awal dengan pesan jelas.
func (h *Handler) requireContactWrite(w http.ResponseWriter, r *http.Request) bool {
	if !canWriteContactsPerm(r.Context()) {
		h.renderContactsForbidden(w, r)
		return false
	}
	return true
}

// ContactNew — GET /w/{workspace}/accounts/{id}/contacts/new. Form kosong. F3
// diwarisi: hanya yang boleh melihat desa induk yang boleh menambah kontaknya.
func (h *Handler) ContactNew(w http.ResponseWriter, r *http.Request) {
	if !h.requireContactWrite(w, r) {
		return
	}
	accountID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	account, ok := h.loadOwnedAccount(w, r, accountID)
	if !ok {
		return
	}

	base := wsPath(slugFromRequest(r), "")
	accountBase := base + "/accounts/" + strconv.FormatInt(accountID, 10)
	h.renderWorkspaceShell(w, r, "Tambah Kontak", "/accounts", panel.ContactForm(panel.ContactFormView{
		AccountBase: accountBase,
		AccountName: account.VillageName,
		Action:      accountBase + "/contacts",
		IsEdit:      false,
		Err:         wsErrMsg(r.URL.Query().Get("err")),
		Positions:   contactPositionOptions,
		Roles:       contactRoleOptions,
		Channels:    contactChannelOptions,
	}))
}

// ContactCreate — POST /w/{workspace}/accounts/{id}/contacts. Membuat kontak di
// desa induk. Bila ditandai primary, primary lama dikosongkan lebih dulu (satu tx).
func (h *Handler) ContactCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requireContactWrite(w, r) {
		return
	}
	ctx := r.Context()
	accountID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadOwnedAccount(w, r, accountID); !ok {
		return
	}

	form, errCode := parseContactForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/accounts/"+strconv.FormatInt(accountID, 10)+"/contacts/new", errCode)
		return
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	// Set-primary dua langkah: kosongkan yang lama SEBELUM INSERT primary baru,
	// atau index menolak dua utama. Idempotent bila belum ada utama.
	if form.IsPrimaryContact {
		if err := h.q(ctx).ClearAccountPrimaryContact(ctx, db.ClearAccountPrimaryContactParams{
			UpdatedBy: &uid, AccountID: accountID,
		}); err != nil {
			h.Log.Error("contacts: clear primary", "err", err)
			wsRedirect(w, r, "/accounts/"+strconv.FormatInt(accountID, 10)+"/contacts/new", "failed")
			return
		}
	}

	c, err := h.q(ctx).CreateContact(ctx, db.CreateContactParams{
		TenantID:           tenantID,
		AccountID:          accountID,
		ContactOwner:       &uid, // pembuat = pemilik awal kontak
		FirstName:          form.FirstName,
		LastName:           form.LastName,
		Salutation:         form.Salutation,
		JobTitle:           form.JobTitle,
		PositionCategory:   form.PositionCategory,
		ContactRole:        form.ContactRole,
		IsPrimaryContact:   form.IsPrimaryContact,
		IsTechnicalContact: form.IsTechnicalContact,
		TermPeriod:         form.TermPeriod,
		MobilePhone:        form.MobilePhone,
		WhatsappNumber:     form.WhatsappNumber,
		OfficePhone:        form.OfficePhone,
		Email:              form.Email,
		PreferredChannel:   form.PreferredChannel,
		MailingAddress:     form.MailingAddress,
		City:               form.City,
		PostalCode:         form.PostalCode,
		EmailOptOut:        form.EmailOptOut,
		DoNotContact:       form.DoNotContact,
		CreatedBy:          &uid,
	})
	if err != nil {
		h.Log.Error("contacts: create", "err", err)
		wsRedirect(w, r, "/accounts/"+strconv.FormatInt(accountID, 10)+"/contacts/new", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "contact.create", tenantID, map[string]string{
		"account_id": strconv.FormatInt(accountID, 10),
		"contact_id": strconv.FormatInt(c.ID, 10),
	})
	wsRedirectOK(w, r, contactPath(accountID, c.ID), "created")
}

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
		AccountBase:   accountBase,
		AccountName:   account.VillageName,
		Action:        contactPath(accountID, contactID),
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

// ContactSetPrimary — POST /w/{workspace}/accounts/{id}/contacts/{contactID}/primary.
// Menjadikan satu kontak sebagai utama desa. Dua langkah: kosongkan lama → pasang
// baru (satu tx). account_id ikut di WHERE SetPrimaryContact sebagai sabuk pengaman.
func (h *Handler) ContactSetPrimary(w http.ResponseWriter, r *http.Request) {
	if !h.requireContactWrite(w, r) {
		return
	}
	ctx := r.Context()
	accountID, contactID, ok := h.parseContactRef(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadOwnedContact(w, r, accountID, contactID); !ok {
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).ClearAccountPrimaryContact(ctx, db.ClearAccountPrimaryContactParams{
		UpdatedBy: &uid, AccountID: accountID,
	}); err != nil {
		h.Log.Error("contacts: clear primary", "err", err)
		wsRedirect(w, r, "/accounts/"+strconv.FormatInt(accountID, 10)+"/contacts", "failed")
		return
	}
	if err := h.q(ctx).SetPrimaryContact(ctx, db.SetPrimaryContactParams{
		UpdatedBy: &uid, ID: contactID, AccountID: accountID,
	}); err != nil {
		h.Log.Error("contacts: set primary", "err", err)
		wsRedirect(w, r, "/accounts/"+strconv.FormatInt(accountID, 10)+"/contacts", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "contact.primary", session.TenantID(ctx), map[string]string{
		"account_id": strconv.FormatInt(accountID, 10),
		"contact_id": strconv.FormatInt(contactID, 10),
	})
	wsRedirectOK(w, r, "/accounts/"+strconv.FormatInt(accountID, 10)+"/contacts", "primary")
}

// ContactDelete — POST /w/{workspace}/accounts/{id}/contacts/{contactID}/delete.
// Soft-delete. Kontak utama yang dihapus membebaskan slot primary (index partial
// WHERE deleted_at IS NULL) — desa boleh tanpa kontak utama.
func (h *Handler) ContactDelete(w http.ResponseWriter, r *http.Request) {
	if !h.requireContactWrite(w, r) {
		return
	}
	ctx := r.Context()
	accountID, contactID, ok := h.parseContactRef(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadOwnedContact(w, r, accountID, contactID); !ok {
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).SoftDeleteContact(ctx, db.SoftDeleteContactParams{
		UpdatedBy: &uid, ID: contactID,
	}); err != nil {
		h.Log.Error("contacts: delete", "err", err)
		wsRedirect(w, r, contactPathRel(accountID, contactID), "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "contact.delete", session.TenantID(ctx), map[string]string{
		"account_id": strconv.FormatInt(accountID, 10),
		"contact_id": strconv.FormatInt(contactID, 10),
	})
	wsRedirectOK(w, r, "/accounts/"+strconv.FormatInt(accountID, 10)+"/contacts", "deleted")
}

// contactPath merakit URL absolut lengkap kontak (dipakai wsRedirectOK yang
// menerima path relatif-ke-workspace; jadi ini path relatif tanpa prefix /w/slug).
func contactPath(accountID, contactID int64) string {
	return "/accounts/" + strconv.FormatInt(accountID, 10) + "/contacts/" + strconv.FormatInt(contactID, 10)
}

// contactPathRel = alias makna-jelas untuk contactPath saat dipakai sebagai
// tujuan redirect galat (relatif-ke-workspace).
func contactPathRel(accountID, contactID int64) string {
	return contactPath(accountID, contactID)
}

// contactEditRel = path relatif form edit kontak (tujuan PRG saat validasi gagal).
func contactEditRel(accountID, contactID int64) string {
	return contactPath(accountID, contactID) + "/edit"
}

// contactFormFields memetakan Contact → nilai prefill form. F4: editor non-Sales
// menerima MASK untuk HP & WhatsApp, bukan nomor asli — nilai asli tak pernah
// mencapai browsernya (view-source pun bersih). Telepon kantor tak disamarkan.
func contactFormFields(ctx context.Context, c db.Contact) panel.ContactFormFields {
	phoneEditable := canEditPhone(ctx)
	mobile, whatsapp := deref(c.MobilePhone), deref(c.WhatsappNumber)
	if !phoneEditable {
		if mobile != "" {
			mobile = flsHidden // tersamar; field dikunci di view, tak ikut ter-submit
		}
		if whatsapp != "" {
			whatsapp = flsHidden
		}
	}
	return panel.ContactFormFields{
		FirstName:          c.FirstName,
		LastName:           deref(c.LastName),
		Salutation:         deref(c.Salutation),
		JobTitle:           deref(c.JobTitle),
		PositionCategory:   deref(c.PositionCategory),
		ContactRole:        deref(c.ContactRole),
		IsPrimaryContact:   c.IsPrimaryContact,
		IsTechnicalContact: c.IsTechnicalContact,
		TermPeriod:         deref(c.TermPeriod),
		MobilePhone:        mobile,
		WhatsappNumber:     whatsapp,
		OfficePhone:        deref(c.OfficePhone),
		Email:              deref(c.Email),
		PreferredChannel:   deref(c.PreferredChannel),
		MailingAddress:     deref(c.MailingAddress),
		City:               deref(c.City),
		PostalCode:         deref(c.PostalCode),
		EmailOptOut:        c.EmailOptOut,
		DoNotContact:       c.DoNotContact,
	}
}

// Opsi enum untuk dropdown — slice BERURUT (map validasi tak berurutan). Nilai
// HARUS himpunan yang sama dengan map validasi & CHECK 00005; urutannya tampilan.
var (
	contactPositionOptions = []string{
		"Kepala Desa", "Sekdes", "Kaur", "Kasi",
		"Operator", "Bendahara", "BPD", "Lainnya",
	}
	contactRoleOptions = []string{
		"Decision Maker", "Influencer", "User", "Finance", "Gatekeeper",
	}
	contactChannelOptions = []string{"WhatsApp", "Telepon", "Email", "Kunjungan"}
)

// compile-time: opsi & map validasi sepakat (panjang sama). Berbeda = dropdown
// menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(contactPositionOptions) != len(validContactPositions) ||
		len(contactRoleOptions) != len(validContactRoles) ||
		len(contactChannelOptions) != len(validContactChannels) {
		panic("contacts: opsi enum tak sinkron dengan map validasi")
	}
	return struct{}{}
}()
