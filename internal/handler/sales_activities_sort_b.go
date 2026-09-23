package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_activities_sort_b.go — fungsi sort kolom status/date + jalur default
// (created_at DESC) untuk mode normal ActivitiesList
// (sales_activities_page.go). Dipisah krn ambang File Health (Route/Handler
// 150 baris); badan tiap fungsi = isi case switch semula APA ADANYA, hanya
// dibungkus wrapper bertipe.

func (h *Handler) activitiesSortByStatus(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.ActivitiesListFilter, uid int64, query, dir string,
) ([]db.Activity, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListActivitiesSortByStatus(ctx, db.ListActivitiesSortByStatusParams{
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
		h.Log.Error("activities: list sort status", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageTextNullable(rows, func(a db.Activity) (string, int64, bool) {
		if a.Status == nil {
			return "", a.ID, true
		}
		return *a.Status, a.ID, false
	})
	return shown, nc, true
}

func (h *Handler) activitiesSortByDate(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.ActivitiesListFilter, uid int64, query, dir string,
) ([]db.Activity, string, bool) {
	cursorAt, cursorSortID, hasCursor := pageCursorTimestamp(r)
	rows, err := h.q(ctx).ListActivitiesSortByDate(ctx, db.ListActivitiesSortByDateParams{
		ContextFilter: activityContextSales,
		HasCursor:     hasCursor,
		Dir:           dir,
		CursorVal:     cursorAt,
		CursorID:      cursorSortID,
		ScopeAll:      filter.ScopeAll,
		IsOwn:         filter.IsOwn,
		Uid:           &uid,
		Search:        query,
		PageSize:      pageSize + 1,
	})
	if err != nil {
		h.Log.Error("activities: list sort date", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageTimestamp(rows, func(a db.Activity) (pgtype.Timestamptz, int64) {
		return a.CreatedAt, a.ID
	})
	return shown, nc, true
}

func (h *Handler) activitiesSortDefault(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.ActivitiesListFilter, uid int64, query string,
) ([]db.Activity, string, bool) {
	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListActivities(ctx, db.ListActivitiesParams{
		ContextFilter:   activityContextSales,
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		Search:          query,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("activities: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPage(rows, func(a db.Activity) (pgtype.Timestamptz, int64) {
		return a.CreatedAt, a.ID
	})
	return shown, nc, true
}
