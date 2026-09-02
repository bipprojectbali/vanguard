package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// contacts_page.go — HALAMAN daftar kontak (orang di dalam sebuah desa): daftar
// per-desa (ContactsList) & daftar global lintas-desa (ContactsAll). Detail satu
// kontak + resolusi/gerbangnya (loadOwnedContact/parseContactRef) di
// contacts_detail.go; aksi tulis di contacts.go/_update.go/_primary.go.
//
// KEPEMILIKAN DIWARISI DESA INDUK, bukan filter sendiri (docs/crm/tasks.md §M3):
//   - daftar per-desa (ContactsList) menjaga lewat desa INDUK — loadOwnedAccount
//     lebih dulu; begitu lolos, SEMUA kontak desa itu tampil.
//   - daftar global (ContactsAll) menyaring lewat kolom ownership DESA INDUK
//     (ListContacts JOIN accounts), flag identik ListAccounts.
//
// F2 (Casbin bisnis "crm:contacts") = gerbang modul. RLS tetap mengisolasi
// WORKSPACE di bawah semua ini.

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
		items = append(items, contactRowView(ctx, c))
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
			After:       r.URL.Query().Get("after"),
			Trail:       pageTrail(r),
			Err:         wsErrMsg(r.URL.Query().Get("err")),
			Msg:         contactsMsg(r.URL.Query().Get("ok")),
		}))
}

// renderContactsForbidden — 403 + penjelasan bagi anggota tanpa peran CRM. Status
// ditulis SEBELUM body agar penolakan tak terkirim sebagai 200 yang tampak sukses.
func (h *Handler) renderContactsForbidden(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, "Kontak", "/contacts", panel.ContactsForbidden())
}
