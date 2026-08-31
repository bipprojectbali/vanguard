package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// contacts_create.go — ContactCreate (submit form) + insertContact (INSERT
// ber-tenant). Dipisah dari contacts.go (requireContactWrite + ContactNew form
// kosong) agar tiap file di bawah ambang Route/Handler (150). Gerbang tulis &
// isolasi RLS sama; satu paket dengan pemanggilnya.
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

	c, err := h.insertContact(ctx, accountID, form)
	if err != nil {
		h.Log.Error("contacts: create", "err", err)
		wsRedirect(w, r, "/accounts/"+strconv.FormatInt(accountID, 10)+"/contacts/new", "failed")
		return
	}
	wsRedirectOK(w, r, contactPath(accountID, c.ID), "created")
}

// insertContact = jalur BUAT kontak bersama (nested & global). Set-primary dua
// langkah (kosongkan yang lama SEBELUM INSERT primary baru, atau idx_contacts_primary
// menolak dua utama) + INSERT + audit dalam SATU tx (h.q sudah tx dari Scope).
// Pemanggil menentukan redirect sukses/gagalnya sendiri (alamat form berbeda).
// F3 sudah DIVERIFIKASI pemanggil lewat loadOwnedAccount(accountID) — helper ini
// menganggap accountID sah & dalam cakupan.
func (h *Handler) insertContact(ctx context.Context, accountID int64, form contactForm) (db.Contact, error) {
	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	if form.IsPrimaryContact {
		if err := h.q(ctx).ClearAccountPrimaryContact(ctx, db.ClearAccountPrimaryContactParams{
			UpdatedBy: &uid, AccountID: accountID,
		}); err != nil {
			return db.Contact{}, err
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
		return db.Contact{}, err
	}

	h.auditWorkspace(ctx, uid, "contact.create", tenantID, map[string]string{
		"account_id": strconv.FormatInt(accountID, 10),
		"contact_id": strconv.FormatInt(c.ID, 10),
	})
	return c, nil
}
