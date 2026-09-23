package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// sales_quotes_index_sort_a.go — fungsi sort per-kolom utk QuotesIndex (code/
// name/deal; lanjutan status/total/default di sales_quotes_index_sort_b.go).
// Diekstrak dari QuotesIndex (sales_quotes_index.go) krn ambang File Health
// (Route/Handler 150 baris) — badan tiap fungsi SALINAN case asli, tak ada
// perubahan logika.

func (h *Handler) quotesSortByCode(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.DealsListFilter, uid int64, query, dir string) ([]panel.QuoteIndexRow, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListQuotesSortByCode(ctx, db.ListQuotesSortByCodeParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorIsNull: isNull,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     filter.ScopeAll,
		IsOwn:        filter.IsOwn,
		Uid:          &uid,
		Search:       query,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("quotes: list sort code", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	page, cur := splitPageTextNullable(rows, func(q db.ListQuotesSortByCodeRow) (string, int64, bool) {
		if q.EntityCode == nil {
			return "", q.ID, true
		}
		return *q.EntityCode, q.ID, false
	})
	var shown []panel.QuoteIndexRow
	for _, q := range page {
		shown = append(shown, quoteIndexRowViewSortByCode(q))
	}
	return shown, cur, true
}

func (h *Handler) quotesSortByName(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.DealsListFilter, uid int64, query, dir string) ([]panel.QuoteIndexRow, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListQuotesSortByName(ctx, db.ListQuotesSortByNameParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorIsNull: isNull,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     filter.ScopeAll,
		IsOwn:        filter.IsOwn,
		Uid:          &uid,
		Search:       query,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("quotes: list sort name", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	page, cur := splitPageTextNullable(rows, func(q db.ListQuotesSortByNameRow) (string, int64, bool) {
		if q.QuoteName == nil {
			return "", q.ID, true
		}
		return *q.QuoteName, q.ID, false
	})
	var shown []panel.QuoteIndexRow
	for _, q := range page {
		shown = append(shown, quoteIndexRowViewSortByName(q))
	}
	return shown, cur, true
}

func (h *Handler) quotesSortByDeal(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.DealsListFilter, uid int64, query, dir string) ([]panel.QuoteIndexRow, string, bool) {
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListQuotesSortByDeal(ctx, db.ListQuotesSortByDealParams{
		HasCursor: hasCursor,
		Dir:       dir,
		CursorVal: cursorVal,
		CursorID:  cursorSortID,
		ScopeAll:  filter.ScopeAll,
		IsOwn:     filter.IsOwn,
		Uid:       &uid,
		Search:    query,
		PageSize:  pageSize + 1,
	})
	if err != nil {
		h.Log.Error("quotes: list sort deal", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	page, cur := splitPageText(rows, func(q db.ListQuotesSortByDealRow) (string, int64) {
		return q.DealName, q.ID
	})
	var shown []panel.QuoteIndexRow
	for _, q := range page {
		shown = append(shown, quoteIndexRowViewSortByDeal(q))
	}
	return shown, cur, true
}
