package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_quotes_index_sort_b.go — lanjutan sales_quotes_index_sort_a.go
// (status/total; jalur default created_at DESC).

func (h *Handler) quotesSortByStatus(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.DealsListFilter, uid int64, query, dir string) ([]panel.QuoteIndexRow, string, bool) {
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListQuotesSortByStatus(ctx, db.ListQuotesSortByStatusParams{
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
		h.Log.Error("quotes: list sort status", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	page, cur := splitPageText(rows, func(q db.ListQuotesSortByStatusRow) (string, int64) {
		return q.QuoteStatus, q.ID
	})
	var shown []panel.QuoteIndexRow
	for _, q := range page {
		shown = append(shown, quoteIndexRowViewSortByStatus(q))
	}
	return shown, cur, true
}

// quotesSortByTotal — cursor Grand Total kanonik = teks desimal apa adanya
// (optNumeric, sales_format_number.go) — urutan SUNGGUHAN terjadi di SQL atas
// cursor_val bertipe numeric asli, mirror Nilai Deals.
func (h *Handler) quotesSortByTotal(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.DealsListFilter, uid int64, query, dir string) ([]panel.QuoteIndexRow, string, bool) {
	cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	cursorVal, code := optNumeric(cursorRaw, "")
	if code != "" {
		hasCursor = false
	}
	rows, err := h.q(ctx).ListQuotesSortByTotal(ctx, db.ListQuotesSortByTotalParams{
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
		h.Log.Error("quotes: list sort total", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	page, cur := splitPageTextNullable(rows, func(q db.ListQuotesSortByTotalRow) (string, int64, bool) {
		if !q.GrandTotal.Valid {
			return "", q.ID, true
		}
		return numericStr(q.GrandTotal), q.ID, false
	})
	var shown []panel.QuoteIndexRow
	for _, q := range page {
		shown = append(shown, quoteIndexRowViewSortByTotal(q))
	}
	return shown, cur, true
}

func (h *Handler) quotesSortDefault(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.DealsListFilter, uid int64, query string) ([]panel.QuoteIndexRow, string, bool) {
	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListQuotes(ctx, db.ListQuotesParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		Search:          query,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("quotes: index", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	page, cur := splitPage(rows, func(q db.ListQuotesRow) (pgtype.Timestamptz, int64) {
		return q.CreatedAt, q.ID
	})
	var shown []panel.QuoteIndexRow
	for _, q := range page {
		shown = append(shown, quoteIndexRowView(q))
	}
	return shown, cur, true
}
