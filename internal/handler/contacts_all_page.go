package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// contacts_all_page.go — daftar kontak LINTAS-desa (ContactsAll), dipisah dari
// contacts_page.go (daftar per-desa + renderContactsForbidden) demi ambang tipe
// Route/Handler (150). Package sama; kepemilikan diwarisi desa induk identik.

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
	switch sortCol {
	case "code":
		cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		rows, err := h.q(ctx).ListContactsSortByCode(ctx, db.ListContactsSortByCodeParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     params.ScopeAll,
			IsSales:      params.IsSales,
			IsCsm:        params.IsCsm,
			Uid:          params.Uid,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("contacts: list sort code", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageTextNullable(rows, func(c db.ListContactsSortByCodeRow) (string, int64, bool) {
			if c.EntityCode == nil {
				return "", c.ID, true
			}
			return *c.EntityCode, c.ID, false
		})
		nextCursor = nc
		items = make([]panel.ContactRow, 0, len(shown))
		for _, c := range shown {
			items = append(items, contactRowViewGlobal(ctx, contactListRowFromCodeSort(c)))
		}
	case "name":
		// Nama diurut ekspresi PERSIS yang tampil (first_name + last_name) —
		// SELALU NOT NULL (first_name NOT NULL, coalesce menutup last_name).
		cursorVal, cursorSortID, hasCursor := pageCursorText(r)
		rows, err := h.q(ctx).ListContactsSortByName(ctx, db.ListContactsSortByNameParams{
			HasCursor: hasCursor,
			Dir:       dir,
			CursorVal: cursorVal,
			CursorID:  cursorSortID,
			ScopeAll:  params.ScopeAll,
			IsSales:   params.IsSales,
			IsCsm:     params.IsCsm,
			Uid:       params.Uid,
			Search:    query,
			PageSize:  pageSize + 1,
		})
		if err != nil {
			h.Log.Error("contacts: list sort name", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageText(rows, func(c db.ListContactsSortByNameRow) (string, int64) {
			return fullName(c.FirstName, c.LastName), c.ID
		})
		nextCursor = nc
		items = make([]panel.ContactRow, 0, len(shown))
		for _, c := range shown {
			items = append(items, contactRowViewGlobal(ctx, contactListRowFromNameSort(c)))
		}
	case "role":
		// Peran diurut RAW nilai kolom (alfabetis) — sama keputusan "Status"
		// Leads/"Tipe" Accounts: tak menduplikasi urutan tampil ke SQL.
		cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		rows, err := h.q(ctx).ListContactsSortByRole(ctx, db.ListContactsSortByRoleParams{
			HasCursor:    hasCursor,
			Dir:          dir,
			CursorIsNull: isNull,
			CursorVal:    cursorVal,
			CursorID:     cursorSortID,
			ScopeAll:     params.ScopeAll,
			IsSales:      params.IsSales,
			IsCsm:        params.IsCsm,
			Uid:          params.Uid,
			Search:       query,
			PageSize:     pageSize + 1,
		})
		if err != nil {
			h.Log.Error("contacts: list sort role", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageTextNullable(rows, func(c db.ListContactsSortByRoleRow) (string, int64, bool) {
			if c.ContactRole == nil {
				return "", c.ID, true
			}
			return *c.ContactRole, c.ID, false
		})
		nextCursor = nc
		items = make([]panel.ContactRow, 0, len(shown))
		for _, c := range shown {
			items = append(items, contactRowViewGlobal(ctx, contactListRowFromRoleSort(c)))
		}
	case "village":
		// village_name TIDAK NULLABLE (00005_crm_foundation.sql) → pola sederhana
		// (tanpa kerumitan NULL), sama dgn "name".
		cursorVal, cursorSortID, hasCursor := pageCursorText(r)
		rows, err := h.q(ctx).ListContactsSortByVillage(ctx, db.ListContactsSortByVillageParams{
			HasCursor: hasCursor,
			Dir:       dir,
			CursorVal: cursorVal,
			CursorID:  cursorSortID,
			ScopeAll:  params.ScopeAll,
			IsSales:   params.IsSales,
			IsCsm:     params.IsCsm,
			Uid:       params.Uid,
			Search:    query,
			PageSize:  pageSize + 1,
		})
		if err != nil {
			h.Log.Error("contacts: list sort village", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPageText(rows, func(c db.ListContactsSortByVillageRow) (string, int64) {
			return c.VillageName, c.ID
		})
		nextCursor = nc
		items = make([]panel.ContactRow, 0, len(shown))
		for _, c := range shown {
			items = append(items, contactRowViewGlobal(ctx, contactListRowFromVillageSort(c)))
		}
	default:
		params.CursorCreatedAt, params.CursorID = pageCursor(r)
		params.Search = query
		params.PageSize = pageSize + 1
		rows, err := h.q(ctx).ListContacts(ctx, params)
		if err != nil {
			h.Log.Error("contacts: list all", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		shown, nc := splitPage(rows, func(c db.ListContactsRow) (pgtype.Timestamptz, int64) {
			return c.CreatedAt, c.ID
		})
		nextCursor = nc
		items = make([]panel.ContactRow, 0, len(shown))
		for _, c := range shown {
			items = append(items, contactRowViewGlobal(ctx, contactListRowFromDefault(c)))
		}
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
