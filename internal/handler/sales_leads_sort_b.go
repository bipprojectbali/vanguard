package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
)

// sales_leads_sort_b.go — lanjutan sales_leads_sort_a.go (source/rating/value;
// owner/default di sales_leads_sort_c.go).

func (h *Handler) leadsSortBySource(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.LeadsListFilter, uid int64, mineOnly bool, statusFilter, query, dir string) ([]db.Lead, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListLeadsSortBySource(ctx, db.ListLeadsSortBySourceParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorIsNull: isNull,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     filter.ScopeAll,
		IsOwn:        filter.IsOwn,
		Uid:          &uid,
		MineOnly:     mineOnly,
		StatusFilter: statusFilter,
		Search:       query,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("leads: list sort source", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPageTextNullable(rows, func(l db.Lead) (string, int64, bool) {
		if l.LeadSource == nil {
			return "", l.ID, true
		}
		return *l.LeadSource, l.ID, false
	})
	return shown, nextCursor, true
}

func (h *Handler) leadsSortByRating(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.LeadsListFilter, uid int64, mineOnly bool, statusFilter, query, dir string) ([]db.Lead, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListLeadsSortByRating(ctx, db.ListLeadsSortByRatingParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorIsNull: isNull,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     filter.ScopeAll,
		IsOwn:        filter.IsOwn,
		Uid:          &uid,
		MineOnly:     mineOnly,
		StatusFilter: statusFilter,
		Search:       query,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("leads: list sort rating", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPageTextNullable(rows, func(l db.Lead) (string, int64, bool) {
		if l.Rating == nil {
			return "", l.ID, true
		}
		return *l.Rating, l.ID, false
	})
	return shown, nextCursor, true
}

// leadsSortByValue — cursor Estimasi kanonik = teks desimal apa adanya
// (optNumeric, sales_format_number.go) — urutan SUNGGUHAN terjadi di SQL atas
// cursor_val bertipe numeric asli, mirror MRR Subscriptions.
func (h *Handler) leadsSortByValue(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.LeadsListFilter, uid int64, mineOnly bool, statusFilter, query, dir string) ([]db.Lead, string, bool) {
	cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	cursorVal, code := optNumeric(cursorRaw, "")
	if code != "" {
		hasCursor = false
	}
	rows, err := h.q(ctx).ListLeadsSortByValue(ctx, db.ListLeadsSortByValueParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorIsNull: isNull,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     filter.ScopeAll,
		IsOwn:        filter.IsOwn,
		Uid:          &uid,
		MineOnly:     mineOnly,
		StatusFilter: statusFilter,
		Search:       query,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("leads: list sort value", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPageTextNullable(rows, func(l db.Lead) (string, int64, bool) {
		if !l.EstimatedValue.Valid {
			return "", l.ID, true
		}
		return numericStr(l.EstimatedValue), l.ID, false
	})
	return shown, nextCursor, true
}
