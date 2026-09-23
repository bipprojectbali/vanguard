package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// contacts_all_sort_b.go — fungsi sort kolom role/village + jalur default
// (created_at DESC) untuk ContactsAll (contacts_all_page.go). Dipisah krn
// ambang File Health (Route/Handler 150 baris); badan tiap fungsi = isi case
// switch semula APA ADANYA, hanya dibungkus wrapper bertipe.

func (h *Handler) contactsAllSortByRole(w http.ResponseWriter, r *http.Request, ctx context.Context,
	params db.ListContactsParams, query, dir string,
) ([]panel.ContactRow, string, bool) {
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
		return nil, "", false
	}
	shown, nc := splitPageTextNullable(rows, func(c db.ListContactsSortByRoleRow) (string, int64, bool) {
		if c.ContactRole == nil {
			return "", c.ID, true
		}
		return *c.ContactRole, c.ID, false
	})
	items := make([]panel.ContactRow, 0, len(shown))
	for _, c := range shown {
		items = append(items, contactRowViewGlobal(ctx, contactListRowFromRoleSort(c)))
	}
	return items, nc, true
}

func (h *Handler) contactsAllSortByVillage(w http.ResponseWriter, r *http.Request, ctx context.Context,
	params db.ListContactsParams, query, dir string,
) ([]panel.ContactRow, string, bool) {
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
		return nil, "", false
	}
	shown, nc := splitPageText(rows, func(c db.ListContactsSortByVillageRow) (string, int64) {
		return c.VillageName, c.ID
	})
	items := make([]panel.ContactRow, 0, len(shown))
	for _, c := range shown {
		items = append(items, contactRowViewGlobal(ctx, contactListRowFromVillageSort(c)))
	}
	return items, nc, true
}

func (h *Handler) contactsAllSortDefault(w http.ResponseWriter, r *http.Request, ctx context.Context,
	params db.ListContactsParams, query string,
) ([]panel.ContactRow, string, bool) {
	params.CursorCreatedAt, params.CursorID = pageCursor(r)
	params.Search = query
	params.PageSize = pageSize + 1
	rows, err := h.q(ctx).ListContacts(ctx, params)
	if err != nil {
		h.Log.Error("contacts: list all", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPage(rows, func(c db.ListContactsRow) (pgtype.Timestamptz, int64) {
		return c.CreatedAt, c.ID
	})
	items := make([]panel.ContactRow, 0, len(shown))
	for _, c := range shown {
		items = append(items, contactRowViewGlobal(ctx, contactListRowFromDefault(c)))
	}
	return items, nc, true
}
