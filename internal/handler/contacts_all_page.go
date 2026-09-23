package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// contacts_all_page.go — daftar kontak LINTAS-desa (ContactsAll), dipisah dari
// contacts_page.go (daftar per-desa + renderContactsForbidden) demi ambang tipe
// Route/Handler (150). Package sama; kepemilikan diwarisi desa induk identik.
//
// Fungsi sort per-kolom (code/name di sort_a.go, role/village/default di
// sort_b.go) dipisah krn ambang File Health yang sama.

// ContactsAll — GET /w/{workspace}/contacts. Daftar kontak LINTAS-desa, keyset +
// filter kepemilikan DESA INDUK (bukan filter kontak sendiri). Ditolak (bukan
// pemegang peran CRM) → 403 + penjelasan (kembaran AccountsList).
func (h *Handler) ContactsAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewContacts(ctx) {
		h.renderContactsForbidden(w, r)
		return
	}

	dataScope := session.BusinessDataScope(ctx)
	uid := session.UserID(ctx)
	// Tab (Semua/Kontak Saya) HANYA untuk peran ScopeAll — Sales/CSM (ScopeOwn) cuma
	// lihat kontak di desanya (Semua≡Saya), jadi tab redundan & disembunyikan.
	showTabs := db.AccountsScopeFor(dataScope) == db.ScopeAll
	view := normalizeContactsView(r.URL.Query().Get("view"), showTabs)

	// q = pencarian bebas (BL-6): MEMPERSEMPIT di atas ownership desa induk.
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	params := contactsListParams(dataScope, view, uid)

	// sort/dir (BL-157d): whitelist 4 kolom sortable (contactSortableColumns).
	// Kombinasi tak dikenal → jatuh ke jalur default (created_at DESC), TAK
	// error — URL lama/disunting tetap render halaman yang benar.
	sortCol := r.URL.Query().Get("sort")
	if !contactSortableColumns[sortCol] {
		sortCol = ""
	}
	dir := r.URL.Query().Get("dir")
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}

	var items []panel.ContactRow
	var nextCursor string
	var ok bool
	switch sortCol {
	case "code":
		items, nextCursor, ok = h.contactsAllSortByCode(w, r, ctx, params, query, dir)
	case "name":
		items, nextCursor, ok = h.contactsAllSortByName(w, r, ctx, params, query, dir)
	case "role":
		items, nextCursor, ok = h.contactsAllSortByRole(w, r, ctx, params, query, dir)
	case "village":
		items, nextCursor, ok = h.contactsAllSortByVillage(w, r, ctx, params, query, dir)
	default:
		items, nextCursor, ok = h.contactsAllSortDefault(w, r, ctx, params, query)
	}
	if !ok {
		return
	}

	// Tombol "Tambah Kontak" tampil hanya bila aktor boleh menulis DAN punya ≥1
	// desa yang bisa dipilih sebagai induk. Count gagal → sembunyikan tombol (log),
	// bukan 500-kan daftar: tombol sekunder, daftar tetap berguna.
	canWrite := false
	if canWriteContacts(ctx) {
		has, err := h.hasWritableAccount(ctx)
		if err != nil {
			h.Log.Error("contacts: count writable accounts", "err", err)
		}
		canWrite = has
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Kontak", "/contacts",
		panel.ContactsAll(panel.ContactsAllView{
			Base:       base,
			Items:      items,
			ShowTabs:   showTabs,
			ActiveView: view,
			Query:      query,
			NextCursor: nextCursor,
			After:      r.URL.Query().Get("after"),
			Trail:      pageTrail(r),
			CanWrite:   canWrite,
			Err:        wsErrMsg(r.URL.Query().Get("err")),
			Msg:        contactsMsg(r.URL.Query().Get("ok")),
			Sort:       sortCol,
			Dir:        dir,
		}))
}

// contactSortableColumns = whitelist kolom yang boleh diminta lewat ?sort=
// (BL-157d: 4 kolom tabel Kontak global). ?sort= di luar daftar ini
// diperlakukan seolah absen (jatuh ke default created_at DESC), TAK error.
// HP/WhatsApp SENGAJA tak sortable (F4 — mempermudah pencarian nomor tanpa
// hak lihat via urutan); Penanda (4 boolean lepas, tak ada kunci sort tunggal
// wajar) & Terakhir (placeholder DITUNDA, selalu "—") juga dikecualikan.
var contactSortableColumns = map[string]bool{
	"code":    true,
	"name":    true,
	"role":    true,
	"village": true,
}
