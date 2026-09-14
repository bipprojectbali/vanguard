package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_quotes_index.go — GET /w/{workspace}/quotes: daftar quote LINTAS-deal
// (menu sidebar "Quotes"). Beda dari QuotesList (nested /deals/{id}/quotes) yang
// hanya menampilkan quote satu deal. Ownership F3 DIWARISI dari deal induk: query
// ListQuotes JOIN deals lalu menyaring deal_owner (sama seperti ListDeals). Read-
// only — pembuatan quote tetap dari detail deal (mewarisi account/deal).

// QuotesIndex merender daftar quote seluruh workspace dalam cakupan F3 user,
// berkeyset. Gate = canViewDeals (quote mewarisi izin baca dari deal).
func (h *Handler) QuotesIndex(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewDeals(ctx) {
		h.renderDealsForbidden(w, r)
		return
	}

	filter := db.DealsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)

	// q = pencarian bebas (BL-6): MEMPERSEMPIT di atas ownership warisan deal.
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	// sort/dir (BL-157f): whitelist 5 kolom sortable (quoteSortableColumns).
	// Kombinasi tak dikenal → jatuh ke jalur default (created_at DESC), TAK
	// error — URL lama/disunting tetap render halaman yang benar.
	sortCol := r.URL.Query().Get("sort")
	if !quoteSortableColumns[sortCol] {
		sortCol = ""
	}
	dir := r.URL.Query().Get("dir")
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}

	var shown []panel.QuoteIndexRow
	var nextCursor string
	switch sortCol {
	case "code":
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
			return
		}
		page, cur := splitPageTextNullable(rows, func(q db.ListQuotesSortByCodeRow) (string, int64, bool) {
			if q.EntityCode == nil {
				return "", q.ID, true
			}
			return *q.EntityCode, q.ID, false
		})
		nextCursor = cur
		for _, q := range page {
			shown = append(shown, quoteIndexRowViewSortByCode(q))
		}
	case "name":
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
			return
		}
		page, cur := splitPageTextNullable(rows, func(q db.ListQuotesSortByNameRow) (string, int64, bool) {
			if q.QuoteName == nil {
				return "", q.ID, true
			}
			return *q.QuoteName, q.ID, false
		})
		nextCursor = cur
		for _, q := range page {
			shown = append(shown, quoteIndexRowViewSortByName(q))
		}
	case "deal":
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
			return
		}
		page, cur := splitPageText(rows, func(q db.ListQuotesSortByDealRow) (string, int64) {
			return q.DealName, q.ID
		})
		nextCursor = cur
		for _, q := range page {
			shown = append(shown, quoteIndexRowViewSortByDeal(q))
		}
	case "status":
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
			return
		}
		page, cur := splitPageText(rows, func(q db.ListQuotesSortByStatusRow) (string, int64) {
			return q.QuoteStatus, q.ID
		})
		nextCursor = cur
		for _, q := range page {
			shown = append(shown, quoteIndexRowViewSortByStatus(q))
		}
	case "total":
		cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
		// cursor Grand Total kanonik = teks desimal apa adanya (optNumeric ada
		// di sales_format_number.go) — urutan SUNGGUHAN terjadi di SQL atas
		// cursor_val bertipe numeric asli, mirror Nilai Deals.
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
			return
		}
		page, cur := splitPageTextNullable(rows, func(q db.ListQuotesSortByTotalRow) (string, int64, bool) {
			if !q.GrandTotal.Valid {
				return "", q.ID, true
			}
			return numericStr(q.GrandTotal), q.ID, false
		})
		nextCursor = cur
		for _, q := range page {
			shown = append(shown, quoteIndexRowViewSortByTotal(q))
		}
	default:
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
			return
		}
		page, cur := splitPage(rows, func(q db.ListQuotesRow) (pgtype.Timestamptz, int64) {
			return q.CreatedAt, q.ID
		})
		nextCursor = cur
		for _, q := range page {
			shown = append(shown, quoteIndexRowView(q))
		}
	}
	if shown == nil {
		shown = []panel.QuoteIndexRow{}
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Quotes", "/quotes", panel.QuotesIndex(panel.QuotesIndexView{
		Base:       base,
		Err:        wsErrMsg(r.URL.Query().Get("err")),
		Query:      query,
		Items:      shown,
		NextCursor: nextCursor,
		After:      r.URL.Query().Get("after"),
		Trail:      pageTrail(r),
		Sort:       sortCol,
		Dir:        dir,
	}))
}

// quoteSortableColumns = whitelist kolom yang boleh diminta lewat ?sort=
// (BL-157f: 5 kolom tabel Quotes global). ?sort= di luar daftar ini
// diperlakukan seolah absen (jatuh ke default created_at DESC), TAK error.
var quoteSortableColumns = map[string]bool{
	"code":   true,
	"name":   true,
	"deal":   true,
	"status": true,
	"total":  true,
}

// quoteIndexRowView memetakan satu baris ListQuotes (quote + deal_name) → baris
// daftar global. Angka SUDAH diformat; status mentah (badge diputuskan view).
// DealID dibawa agar view merakit tautan ke quote di bawah deal induknya.
func quoteIndexRowView(q db.ListQuotesRow) panel.QuoteIndexRow {
	// deal_id selalu terisi (ListQuotes INNER JOIN deals ON d.id = q.deal_id) —
	// guard nil hanya untuk ketahanan tipe (skema mengizinkan NULL).
	var dealID int64
	if q.DealID != nil {
		dealID = *q.DealID
	}
	return panel.QuoteIndexRow{
		QuoteID:    q.ID,
		DealID:     dealID,
		EntityCode: deref(q.EntityCode),
		QuoteName:  deref(q.QuoteName),
		DealName:   q.DealName,
		Status:     q.QuoteStatus,
		GrandTotal: formatRupiah(q.GrandTotal),
	}
}

// quoteIndexRowViewSortByCode/…SortByName/…SortByDeal/…SortByStatus/…SortByTotal
// memetakan baris dari tiap query ListQuotesSortByX (kolom Row-nya identik dgn
// ListQuotesRow, generated per-query oleh sqlc) → panel.QuoteIndexRow yang sama.
func quoteIndexRowViewSortByCode(q db.ListQuotesSortByCodeRow) panel.QuoteIndexRow {
	var dealID int64
	if q.DealID != nil {
		dealID = *q.DealID
	}
	return panel.QuoteIndexRow{
		QuoteID:    q.ID,
		DealID:     dealID,
		EntityCode: deref(q.EntityCode),
		QuoteName:  deref(q.QuoteName),
		DealName:   q.DealName,
		Status:     q.QuoteStatus,
		GrandTotal: formatRupiah(q.GrandTotal),
	}
}

func quoteIndexRowViewSortByName(q db.ListQuotesSortByNameRow) panel.QuoteIndexRow {
	var dealID int64
	if q.DealID != nil {
		dealID = *q.DealID
	}
	return panel.QuoteIndexRow{
		QuoteID:    q.ID,
		DealID:     dealID,
		EntityCode: deref(q.EntityCode),
		QuoteName:  deref(q.QuoteName),
		DealName:   q.DealName,
		Status:     q.QuoteStatus,
		GrandTotal: formatRupiah(q.GrandTotal),
	}
}

func quoteIndexRowViewSortByDeal(q db.ListQuotesSortByDealRow) panel.QuoteIndexRow {
	var dealID int64
	if q.DealID != nil {
		dealID = *q.DealID
	}
	return panel.QuoteIndexRow{
		QuoteID:    q.ID,
		DealID:     dealID,
		EntityCode: deref(q.EntityCode),
		QuoteName:  deref(q.QuoteName),
		DealName:   q.DealName,
		Status:     q.QuoteStatus,
		GrandTotal: formatRupiah(q.GrandTotal),
	}
}

func quoteIndexRowViewSortByStatus(q db.ListQuotesSortByStatusRow) panel.QuoteIndexRow {
	var dealID int64
	if q.DealID != nil {
		dealID = *q.DealID
	}
	return panel.QuoteIndexRow{
		QuoteID:    q.ID,
		DealID:     dealID,
		EntityCode: deref(q.EntityCode),
		QuoteName:  deref(q.QuoteName),
		DealName:   q.DealName,
		Status:     q.QuoteStatus,
		GrandTotal: formatRupiah(q.GrandTotal),
	}
}

func quoteIndexRowViewSortByTotal(q db.ListQuotesSortByTotalRow) panel.QuoteIndexRow {
	var dealID int64
	if q.DealID != nil {
		dealID = *q.DealID
	}
	return panel.QuoteIndexRow{
		QuoteID:    q.ID,
		DealID:     dealID,
		EntityCode: deref(q.EntityCode),
		QuoteName:  deref(q.QuoteName),
		DealName:   q.DealName,
		Status:     q.QuoteStatus,
		GrandTotal: formatRupiah(q.GrandTotal),
	}
}
