package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// sales_quotes_index.go — GET /w/{workspace}/quotes: daftar quote LINTAS-deal
// (menu sidebar "Quotes"). Beda dari QuotesList (nested /deals/{id}/quotes) yang
// hanya menampilkan quote satu deal. Ownership F3 DIWARISI dari deal induk: query
// ListQuotes JOIN deals lalu menyaring deal_owner (sama seperti ListDeals). Read-
// only — pembuatan quote tetap dari detail deal (mewarisi account/deal).
//
// quoteSortableColumns/quoteIndexRowView* di sales_quotes_index_row.go; fungsi
// sort per-kolom di sales_quotes_index_sort_a.go & sales_quotes_index_sort_b.go
// (dipisah krn ambang File Health).

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
	var ok bool
	switch sortCol {
	case "code":
		shown, nextCursor, ok = h.quotesSortByCode(w, r, ctx, filter, uid, query, dir)
		if !ok {
			return
		}
	case "name":
		shown, nextCursor, ok = h.quotesSortByName(w, r, ctx, filter, uid, query, dir)
		if !ok {
			return
		}
	case "deal":
		shown, nextCursor, ok = h.quotesSortByDeal(w, r, ctx, filter, uid, query, dir)
		if !ok {
			return
		}
	case "status":
		shown, nextCursor, ok = h.quotesSortByStatus(w, r, ctx, filter, uid, query, dir)
		if !ok {
			return
		}
	case "total":
		shown, nextCursor, ok = h.quotesSortByTotal(w, r, ctx, filter, uid, query, dir)
		if !ok {
			return
		}
	default:
		shown, nextCursor, ok = h.quotesSortDefault(w, r, ctx, filter, uid, query)
		if !ok {
			return
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
