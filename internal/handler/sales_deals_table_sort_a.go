package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
)

// sales_deals_table_sort_a.go — fungsi sort kolom name/stage/code untuk
// dealsTable (sales_deals_table_page.go). Dipisah krn ambang File Health
// (Route/Handler 150 baris); badan tiap fungsi = isi case switch semula
// APA ADANYA, hanya dibungkus wrapper bertipe.

func (h *Handler) dealsSortByName(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.DealsListFilter, uid int64, mineOnly bool, stageFilter, query, dir string,
	names map[int64]string,
) ([]db.Deal, string, bool) {
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListDealsSortByName(ctx, db.ListDealsSortByNameParams{
		HasCursor:   hasCursor,
		Dir:         dir,
		CursorVal:   cursorVal,
		CursorID:    cursorSortID,
		ScopeAll:    filter.ScopeAll,
		IsOwn:       filter.IsOwn,
		Uid:         &uid,
		MineOnly:    mineOnly,
		StageFilter: stageFilter,
		Search:      query,
		PageSize:    pageSize + 1,
	})
	if err != nil {
		h.Log.Error("deals: list sort name", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPageText(rows, func(d db.Deal) (string, int64) {
		return d.DealName, d.ID
	})
	return shown, nextCursor, true
}

func (h *Handler) dealsSortByStage(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.DealsListFilter, uid int64, mineOnly bool, stageFilter, query, dir string,
	names map[int64]string,
) ([]db.Deal, string, bool) {
	// Tahap diurut RAW enum (Prospecting/Qualification/…, alfabetis) —
	// keputusan user, tak menduplikasi urutan pipeline ke SQL.
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListDealsSortByStage(ctx, db.ListDealsSortByStageParams{
		HasCursor:   hasCursor,
		Dir:         dir,
		CursorVal:   cursorVal,
		CursorID:    cursorSortID,
		ScopeAll:    filter.ScopeAll,
		IsOwn:       filter.IsOwn,
		Uid:         &uid,
		MineOnly:    mineOnly,
		StageFilter: stageFilter,
		Search:      query,
		PageSize:    pageSize + 1,
	})
	if err != nil {
		h.Log.Error("deals: list sort stage", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPageText(rows, func(d db.Deal) (string, int64) {
		return d.Stage, d.ID
	})
	return shown, nextCursor, true
}

func (h *Handler) dealsSortByCode(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.DealsListFilter, uid int64, mineOnly bool, stageFilter, query, dir string,
	names map[int64]string,
) ([]db.Deal, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListDealsSortByCode(ctx, db.ListDealsSortByCodeParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorIsNull: isNull,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     filter.ScopeAll,
		IsOwn:        filter.IsOwn,
		Uid:          &uid,
		MineOnly:     mineOnly,
		StageFilter:  stageFilter,
		Search:       query,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("deals: list sort code", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPageTextNullable(rows, func(d db.Deal) (string, int64, bool) {
		if d.EntityCode == nil {
			return "", d.ID, true
		}
		return *d.EntityCode, d.ID, false
	})
	return shown, nextCursor, true
}
