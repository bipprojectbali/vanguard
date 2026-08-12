package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// contacts_primary.go — aksi yang mengubah STATUS kontak: menaikkan satu kontak
// jadi utama (ContactSetPrimary) & soft-delete (ContactDelete). Keduanya berbagi
// invarian idx_contacts_primary (maks 1 utama hidup per desa) yang dipisah dari
// jalur create/update agar aturan set-primary terbaca di satu tempat.

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
