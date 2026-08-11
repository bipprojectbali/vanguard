package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// contacts_page.go — HALAMAN baca kontak (orang di dalam sebuah desa): daftar
// per-desa, daftar global lintas-desa, dan detail. Aksi (buat/sunting/hapus/
// set-primary) ada di contacts.go.
//
// KEPEMILIKAN DIWARISI DESA INDUK, bukan filter sendiri (docs/crm/tasks.md §M3):
//   - daftar per-desa (ContactsList) menjaga lewat desa INDUK — loadOwnedAccount
//     lebih dulu; begitu lolos, SEMUA kontak desa itu tampil.
//   - daftar global (ContactsAll) menyaring lewat kolom ownership DESA INDUK
//     (ListContacts JOIN accounts), flag identik ListAccounts.
//   - detail (ContactDetail) memuat kontak lalu memuat desa induknya &
//     memeriksa filter.Allows atas desa itu (loadOwnedContact).
//
// F2 (Casbin bisnis "crm:contacts") = gerbang modul; F4 (masking nomor) saat
// merender detail. RLS tetap mengisolasi WORKSPACE di bawah semua ini.

// ContactsList — GET /w/{workspace}/accounts/{id}/contacts. Kontak SATU desa,
// keyset. Gerbangnya desa induk: aktor yang tak boleh melihat desa (F3) tak
// pernah sampai ke kontaknya (loadOwnedAccount → 404).
func (h *Handler) ContactsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewContacts(ctx) {
		h.renderContactsForbidden(w, r)
		return
	}
	accountID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	// F3 diwarisi: hanya yang boleh melihat desa induk yang boleh melihat kontaknya.
	account, ok := h.loadOwnedAccount(w, r, accountID)
	if !ok {
		return
	}

	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListContactsByAccount(ctx, db.ListContactsByAccountParams{
		AccountID:       accountID,
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("contacts: list by account", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	shown, nextCursor := splitPage(rows, func(c db.Contact) (pgtype.Timestamptz, int64) {
		return c.CreatedAt, c.ID
	})
	items := make([]panel.ContactRow, 0, len(shown))
	for _, c := range shown {
		items = append(items, contactRowView(c))
	}

	base := wsPath(slugFromRequest(r), "")
	accountBase := base + "/accounts/" + strconv.FormatInt(accountID, 10)
	h.renderWorkspaceShell(w, r, account.VillageName+" — Kontak", "/accounts",
		panel.ContactsList(panel.ContactsListView{
			Base:        base,
			AccountBase: accountBase,
			AccountName: account.VillageName,
			Items:       items,
			CanWrite:    canWriteContacts(ctx),
			NextCursor:  nextCursor,
			Err:         wsErrMsg(r.URL.Query().Get("err")),
			Msg:         contactsMsg(r.URL.Query().Get("ok")),
		}))
}

// ContactsAll — GET /w/{workspace}/contacts. Daftar kontak LINTAS-desa, keyset +
// filter kepemilikan DESA INDUK (bukan filter kontak sendiri). Ditolak (bukan
// pemegang peran CRM) → 403 + penjelasan (kembaran AccountsList).
func (h *Handler) ContactsAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewContacts(ctx) {
		h.renderContactsForbidden(w, r)
		return
	}

	filter := db.AccountsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)

	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListContacts(ctx, db.ListContactsParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		// IsOwn = union kepemilikan desa induk → KEDUA flag SQL true (identik
		// ListAccounts): "kontak siapa yang tampil" = "desa siapa yang tampil".
		IsSales:  filter.IsOwn,
		Uid:      &uid,
		IsCsm:    filter.IsOwn,
		PageSize: pageSize + 1,
	})
	if err != nil {
		h.Log.Error("contacts: list all", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	shown, nextCursor := splitPage(rows, func(c db.Contact) (pgtype.Timestamptz, int64) {
		return c.CreatedAt, c.ID
	})
	items := make([]panel.ContactRow, 0, len(shown))
	for _, c := range shown {
		items = append(items, contactRowView(c))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Kontak", "/contacts",
		panel.ContactsAll(panel.ContactsAllView{
			Base:       base,
			Items:      items,
			NextCursor: nextCursor,
			Err:        wsErrMsg(r.URL.Query().Get("err")),
			Msg:        contactsMsg(r.URL.Query().Get("ok")),
		}))
}

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
	c, ok := h.loadOwnedContact(w, r, accountID, contactID)
	if !ok {
		return
	}

	base := wsPath(slugFromRequest(r), "")
	accountBase := base + "/accounts/" + strconv.FormatInt(accountID, 10)
	h.renderWorkspaceShell(w, r, c.FirstName, "/accounts",
		panel.ContactDetail(h.contactDetailView(ctx, base, accountBase, c)))
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
	ctx := r.Context()
	c, err := h.q(ctx).GetContact(ctx, contactID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.Contact{}, false
		}
		h.Log.Error("contacts: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.Contact{}, false
	}
	if c.AccountID != accountID {
		http.NotFound(w, r)
		return db.Contact{}, false
	}
	// Warisan F3: keputusan diambil atas DESA INDUK (loadOwnedAccount → 404 bila
	// di luar cakupan). Memuat ulang desa memastikan kontak selalu dinilai dengan
	// aturan yang sama persis dengan halaman desanya.
	if _, ok := h.loadOwnedAccount(w, r, accountID); !ok {
		return db.Contact{}, false
	}
	return c, true
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

// renderContactsForbidden — 403 + penjelasan bagi anggota tanpa peran CRM. Status
// ditulis SEBELUM body agar penolakan tak terkirim sebagai 200 yang tampak sukses.
func (h *Handler) renderContactsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Kontak", "/contacts", panel.ContactsForbidden())
}
