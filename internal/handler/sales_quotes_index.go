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
	shown, nextCursor := splitPage(rows, func(q db.ListQuotesRow) (pgtype.Timestamptz, int64) {
		return q.CreatedAt, q.ID
	})

	items := make([]panel.QuoteIndexRow, 0, len(shown))
	for _, q := range shown {
		items = append(items, quoteIndexRowView(q))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Quotes", "/quotes", panel.QuotesIndex(panel.QuotesIndexView{
		Base:       base,
		Err:        wsErrMsg(r.URL.Query().Get("err")),
		Query:      query,
		Items:      items,
		NextCursor: nextCursor,
	}))
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
