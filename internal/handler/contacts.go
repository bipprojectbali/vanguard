package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// contacts.go — AKSI atas kontak (orang di dalam sebuah desa): gerbang tulis +
// jalur BUAT (form kosong + create). Sunting/update di contacts_update.go,
// set-primary/soft-delete di contacts_primary.go, helper path/prefill/enum di
// contacts_helpers.go. Dipisah dari contacts_page.go (baca): aksi tumbuh dengan
// aturan TULIS (F2 write), halaman dengan aturan LIHAT.
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
