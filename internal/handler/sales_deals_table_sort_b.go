package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"
)

// sales_deals_table_sort_b.go — fungsi sort kolom amount/probability untuk
// dealsTable (sales_deals_table_page.go). Dipisah krn ambang File Health
// (Route/Handler 150 baris); badan tiap fungsi = isi case switch semula
// APA ADANYA, hanya dibungkus wrapper bertipe.

func (h *Handler) dealsSortByAmount(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.DealsListFilter, uid int64, mineOnly bool, stageFilter, query, dir string,
	names map[int64]string,
) ([]db.Deal, string, bool) {
	cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	// cursor Nilai kanonik = teks desimal apa adanya (optNumeric ada di
	// sales_format_number.go) — urutan SUNGGUHAN terjadi di SQL atas
	// cursor_val bertipe numeric asli, mirror Estimasi Leads.
	cursorVal, code := optNumeric(cursorRaw, "")
	if code != "" {
		hasCursor = false
	}
	rows, err := h.q(ctx).ListDealsSortByAmount(ctx, db.ListDealsSortByAmountParams{
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
		h.Log.Error("deals: list sort amount", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPageTextNullable(rows, func(d db.Deal) (string, int64, bool) {
		if !d.Amount.Valid {
			return "", d.ID, true
		}
		return numericStr(d.Amount), d.ID, false
	})
	return shown, nextCursor, true
}

func (h *Handler) dealsSortByProbability(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.DealsListFilter, uid int64, mineOnly bool, stageFilter, query, dir string,
	names map[int64]string,
) ([]db.Deal, string, bool) {
	cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	// cursor Peluang kanonik = teks integer apa adanya; urutan SUNGGUHAN
	// terjadi di SQL atas cursor_val bertipe smallint asli.
	var cursorVal int16
	if hasCursor && !isNull {
		n, perr := strconv.ParseInt(cursorRaw, 10, 16)
		if perr != nil {
			hasCursor = false
		} else {
			cursorVal = int16(n)
		}
	}
	rows, err := h.q(ctx).ListDealsSortByProbability(ctx, db.ListDealsSortByProbabilityParams{
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
		h.Log.Error("deals: list sort probability", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPageTextNullable(rows, func(d db.Deal) (string, int64, bool) {
		if d.Probability == nil {
			return "", d.ID, true
		}
		return strconv.FormatInt(int64(*d.Probability), 10), d.ID, false
	})
	return shown, nextCursor, true
}
