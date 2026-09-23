package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
)

// sales_activities_sort_a.go — fungsi sort kolom kind/subject/owner (mode
// normal) untuk ActivitiesList (sales_activities_page.go). Dipisah krn
// ambang File Health (Route/Handler 150 baris); badan tiap fungsi = isi
// case switch semula APA ADANYA, hanya dibungkus wrapper bertipe.

func (h *Handler) activitiesSortByKind(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.ActivitiesListFilter, uid int64, query, dir string,
) ([]db.Activity, string, bool) {
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListActivitiesSortByKind(ctx, db.ListActivitiesSortByKindParams{
		ContextFilter: activityContextSales,
		HasCursor:     hasCursor,
		Dir:           dir,
		CursorVal:     cursorVal,
		CursorID:      cursorSortID,
		ScopeAll:      filter.ScopeAll,
		IsOwn:         filter.IsOwn,
		Uid:           &uid,
		Search:        query,
		PageSize:      pageSize + 1,
	})
	if err != nil {
		h.Log.Error("activities: list sort kind", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageText(rows, func(a db.Activity) (string, int64) {
		return a.Kind, a.ID
	})
	return shown, nc, true
}

func (h *Handler) activitiesSortBySubject(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.ActivitiesListFilter, uid int64, query, dir string,
) ([]db.Activity, string, bool) {
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListActivitiesSortBySubject(ctx, db.ListActivitiesSortBySubjectParams{
		ContextFilter: activityContextSales,
		HasCursor:     hasCursor,
		Dir:           dir,
		CursorVal:     cursorVal,
		CursorID:      cursorSortID,
		ScopeAll:      filter.ScopeAll,
		IsOwn:         filter.IsOwn,
		Uid:           &uid,
		Search:        query,
		PageSize:      pageSize + 1,
	})
	if err != nil {
		h.Log.Error("activities: list sort subject", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageText(rows, func(a db.Activity) (string, int64) {
		return a.Subject, a.ID
	})
	return shown, nc, true
}

func (h *Handler) activitiesSortByOwner(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.ActivitiesListFilter, uid int64, query, dir string, names map[int64]string,
) ([]db.Activity, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListActivitiesSortByOwner(ctx, db.ListActivitiesSortByOwnerParams{
		ContextFilter: activityContextSales,
		HasCursor:     hasCursor,
		Dir:           dir,
		CursorIsNull:  isNull,
		CursorVal:     cursorVal,
		CursorID:      cursorSortID,
		ScopeAll:      filter.ScopeAll,
		IsOwn:         filter.IsOwn,
		Uid:           &uid,
		Search:        query,
		PageSize:      pageSize + 1,
	})
	if err != nil {
		h.Log.Error("activities: list sort owner", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	// Kunci cursor Pemilik = nama/email resolusi peta anggota (ownerName),
	// PERSIS yang ditampilkan — bukan owner_id mentah. NULL mengikuti
	// owner_id asli (bukan string kosong hasil ownerName), sama dengan
	// kondisi NULL di kunci sort SQL (LEFT JOIN users).
	shown, nc := splitPageTextNullable(rows, func(a db.Activity) (string, int64, bool) {
		if a.OwnerID == nil {
			return "", a.ID, true
		}
		return ownerName(a.OwnerID, names), a.ID, false
	})
	return shown, nc, true
}
