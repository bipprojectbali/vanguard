package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// contacts_all_sort_a.go — fungsi sort kolom code/name untuk ContactsAll
// (contacts_all_page.go). Dipisah krn ambang File Health (Route/Handler 150
// baris); badan tiap fungsi = isi case switch semula APA ADANYA, hanya
// dibungkus wrapper bertipe.

func (h *Handler) contactsAllSortByCode(w http.ResponseWriter, r *http.Request, ctx context.Context,
	params db.ListContactsParams, query, dir string,
) ([]panel.ContactRow, string, bool) {
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
		return nil, "", false
	}
	shown, nc := splitPageTextNullable(rows, func(c db.ListContactsSortByCodeRow) (string, int64, bool) {
		if c.EntityCode == nil {
			return "", c.ID, true
		}
		return *c.EntityCode, c.ID, false
	})
	items := make([]panel.ContactRow, 0, len(shown))
	for _, c := range shown {
		items = append(items, contactRowViewGlobal(ctx, contactListRowFromCodeSort(c)))
	}
	return items, nc, true
}

func (h *Handler) contactsAllSortByName(w http.ResponseWriter, r *http.Request, ctx context.Context,
	params db.ListContactsParams, query, dir string,
) ([]panel.ContactRow, string, bool) {
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
		return nil, "", false
	}
	shown, nc := splitPageText(rows, func(c db.ListContactsSortByNameRow) (string, int64) {
		return fullName(c.FirstName, c.LastName), c.ID
	})
	items := make([]panel.ContactRow, 0, len(shown))
	for _, c := range shown {
		items = append(items, contactRowViewGlobal(ctx, contactListRowFromNameSort(c)))
	}
	return items, nc, true
}
