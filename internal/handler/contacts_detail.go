package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// contacts_detail.go — baca SATU kontak + resolusi/gerbangnya. ContactDetail
// merender halaman; loadOwnedContact & parseContactRef adalah perkakas bersama
// (dipakai juga oleh aksi edit/update/primary/delete) yang menegakkan warisan F3
// lewat desa induk dan kejujuran URL nested. Daftar kontak di contacts_page.go.

// ContactDetail — GET /w/{workspace}/accounts/{id}/contacts/{contactID}. Satu
// kontak. F3 diwarisi desa induk lewat loadOwnedContact (404 di luar cakupan).
func (h *Handler) ContactDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewContacts(ctx) {
		h.renderContactsForbidden(w, r)
		return
	}
	accountID, contactID, ok := h.parseContactRef(w, r)
	if !ok {
		return
	}
	c, account, ok := h.loadOwnedContactAccount(w, r, accountID, contactID)
	if !ok {
		return
	}

	// Nama Owner/Dibuat/Diubah diresolusi lewat peta anggota workspace (sekali,
	// bukan N+1); id yang tak lagi anggota → "" ("—"). reports_to_id menunjuk kontak
	// lain di desa yang sama (RLS menjamin se-tenant) — namanya diambil terpisah,
	// fail-soft: gagal load → tampil sebagai id kosong, bukan menggagalkan halaman.
	names, err := h.accountMemberNames(ctx)
	if err != nil {
		h.Log.Error("contacts: member names", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	reportsToName := h.reportsToName(ctx, c.ReportsToID)

	base := wsPath(slugFromRequest(r), "")
	accountBase := base + "/accounts/" + strconv.FormatInt(accountID, 10)
	h.renderWorkspaceShell(w, r, c.FirstName, "/accounts",
		panel.ContactDetail(h.contactDetailView(ctx, base, accountBase, account.VillageName, c, names, reportsToName)))
}

// reportsToName meresolusi nama atasan (reports_to_id → nama kontak). nil → "".
// Fail-soft: atasan yang tak ditemukan / ter-soft-delete → "" (view: "—") tanpa
// menggagalkan detail — relasi hierarki bukan data kritis halaman.
func (h *Handler) reportsToName(ctx context.Context, reportsToID *int64) string {
	if reportsToID == nil {
		return ""
	}
	sup, err := h.q(ctx).GetContact(ctx, *reportsToID)
	if err != nil {
		return ""
	}
	return contactFullName(sup)
}

// loadOwnedContact memuat satu kontak & menegakkan F3 lewat DESA INDUK. Kontak
// tak menyaring ownership sendiri (GetContact murni); keputusan "boleh lihat?"
// diambil dari desa induknya — sumber yang sama dengan loadOwnedAccount, jadi
// "boleh lihat desa" & "boleh lihat kontaknya" tak pernah beda jawaban.
//
// accountID dari URL WAJIB cocok dengan c.account_id: URL nested menjanjikan
// kontak ini milik desa itu; kontak dari desa lain → 404 (menyangkal keberadaan
// di bawah alamat itu, bukan membocorkan bahwa ia ada di tempat lain).
func (h *Handler) loadOwnedContact(w http.ResponseWriter, r *http.Request, accountID, contactID int64) (db.Contact, bool) {
	c, _, ok := h.loadOwnedContactAccount(w, r, accountID, contactID)
	return c, ok
}

// loadOwnedContactAccount = loadOwnedContact yang JUGA mengembalikan desa induk
// (sudah dimuat untuk gerbang F3 — dikembalikan alih-alih dibuang agar detail bisa
// menampilkan nama desa tanpa query kedua). Pemanggil yang tak butuh desanya pakai
// loadOwnedContact (pembungkus tipis di atas ini).
func (h *Handler) loadOwnedContactAccount(w http.ResponseWriter, r *http.Request, accountID, contactID int64) (db.Contact, db.Account, bool) {
	ctx := r.Context()
	c, err := h.q(ctx).GetContact(ctx, contactID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.Contact{}, db.Account{}, false
		}
		h.Log.Error("contacts: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.Contact{}, db.Account{}, false
	}
	if c.AccountID != accountID {
		http.NotFound(w, r)
		return db.Contact{}, db.Account{}, false
	}
	// Warisan F3: keputusan diambil atas DESA INDUK (loadOwnedAccount → 404 bila
	// di luar cakupan). Memuat ulang desa memastikan kontak selalu dinilai dengan
	// aturan yang sama persis dengan halaman desanya.
	account, ok := h.loadOwnedAccount(w, r, accountID)
	if !ok {
		return db.Contact{}, db.Account{}, false
	}
	return c, account, true
}

// parseContactRef membaca {id} (desa induk) & {contactID} dari URL nested.
func (h *Handler) parseContactRef(w http.ResponseWriter, r *http.Request) (accountID, contactID int64, ok bool) {
	accountID, ok = h.parseTargetID(w, r)
	if !ok {
		return 0, 0, false
	}
	contactID, err := strconv.ParseInt(chi.URLParam(r, "contactID"), 10, 64)
	if err != nil {
		http.Error(w, "id kontak tidak valid", http.StatusBadRequest)
		return 0, 0, false
	}
	return accountID, contactID, true
}
