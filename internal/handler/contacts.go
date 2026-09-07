package handler

import (
	"net/http"
	"strconv"

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
	ctx := r.Context()
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
		AccountName: accountPickerLabel(account.VillageCode, account.VillageName),
		Action:      accountBase + "/contacts",
		IsEdit:      false,
		Err:         wsErrMsg(r.URL.Query().Get("err")),
		// F4: hanya Sales (canEditPhone) boleh mengisi HP/WhatsApp — tanpa flag ini
		// form BUAT mengunci nomor bagi SEMUA role (termasuk Sales sendiri). Sejajar
		// form EDIT (contactFormFields juga pakai canEditPhone).
		PhoneEditable: canEditPhone(ctx),
		Positions:     contactPositionOptions,
		Roles:         contactRoleOptions,
		Channels:      contactChannelOptions,
	}))
}
