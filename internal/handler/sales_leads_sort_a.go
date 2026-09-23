package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
)

// sales_leads_sort_a.go — fungsi sort per-kolom utk LeadsList (name/status/
// code; lanjutan source/rating/value di sales_leads_sort_b.go, owner/default
// di sales_leads_sort_c.go). Diekstrak dari LeadsList (sales_leads_page.go)
// krn ambang File Health (Route/Handler 150 baris) — badan tiap fungsi
// SALINAN case asli, tak ada perubahan logika.

func (h *Handler) leadsSortByName(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.LeadsListFilter, uid int64, mineOnly bool, statusFilter, query, dir string) ([]db.Lead, string, bool) {
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListLeadsSortByName(ctx, db.ListLeadsSortByNameParams{
		HasCursor:    hasCursor,
		Dir:          dir,
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
		h.Log.Error("leads: list sort name", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPageText(rows, func(l db.Lead) (string, int64) {
		return l.LeadName, l.ID
	})
	return shown, nextCursor, true
}

// leadsSortByStatus — status diurut RAW enum (New/Contacted/…, alfabetis) —
// keputusan user, tak menduplikasi urutan tingkat ke SQL.
func (h *Handler) leadsSortByStatus(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.LeadsListFilter, uid int64, mineOnly bool, statusFilter, query, dir string) ([]db.Lead, string, bool) {
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListLeadsSortByStatus(ctx, db.ListLeadsSortByStatusParams{
		HasCursor:    hasCursor,
		Dir:          dir,
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
		h.Log.Error("leads: list sort status", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPageText(rows, func(l db.Lead) (string, int64) {
		return l.LeadStatus, l.ID
	})
	return shown, nextCursor, true
}

func (h *Handler) leadsSortByCode(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.LeadsListFilter, uid int64, mineOnly bool, statusFilter, query, dir string) ([]db.Lead, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListLeadsSortByCode(ctx, db.ListLeadsSortByCodeParams{
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
		h.Log.Error("leads: list sort code", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPageTextNullable(rows, func(l db.Lead) (string, int64, bool) {
		if l.EntityCode == nil {
			return "", l.ID, true
		}
		return *l.EntityCode, l.ID, false
	})
	return shown, nextCursor, true
}
