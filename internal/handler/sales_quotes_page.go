package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_quotes_page.go — HALAMAN daftar Quote + form buat/sunting HEADER, ter-NEST
// di bawah deal. Detail (builder) ada di sales_quotes_detail.go; label plan di
// sales_quotes_planlabel.go; prefill form (quoteFormFields) di sales_quotes_form.go
// & judul (quoteTitle) di sales_quotes.go — semua dipecah lebih lanjut untuk file
// health, package sama. Aksi ada di sales_quotes.go / sales_quotes_update.go /
// sales_quote_items*.go. Dipisah baca/tulis (meniru sales_deals_page.go): halaman
// tumbuh dengan aturan LIHAT (gate baca + F3 warisan deal), aksi dengan aturan
// TULIS. Gate baca = canViewDeals (quote mewarisi sumbu bisnis "crm:deals");
// ownership dari deal induk (loadOwnedQuote/loadOwnedDeal).

// QuotesList — GET /w/{workspace}/deals/{id}/quotes. Daftar quote SATU deal
// (keyset). Bukan pemegang peran CRM → 403. Deal di luar cakupan F3 → 404.
func (h *Handler) QuotesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewDeals(ctx) {
		h.renderDealsForbidden(w, r)
		return
	}
	dealID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	d, ok := h.loadOwnedDeal(w, r, dealID)
	if !ok {
		return
	}

	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListQuotesForDeal(ctx, db.ListQuotesForDealParams{
		DealID:          &dealID,
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("quotes: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	shown, nextCursor := splitPage(rows, func(q db.Quote) (pgtype.Timestamptz, int64) {
		return q.CreatedAt, q.ID
	})
	items := make([]panel.QuoteRow, 0, len(shown))
	for _, q := range shown {
		items = append(items, quoteRowView(q))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Quote — "+d.DealName, "/deals", panel.QuotesList(panel.QuotesListView{
		Base:       base,
		DealID:     dealID,
		DealName:   d.DealName,
		CanWrite:   canWriteDeals(ctx),
		Err:        wsErrMsg(r.URL.Query().Get("err")),
		Msg:        quotesMsg(r.URL.Query().Get("ok")),
		Items:      items,
		NextCursor: nextCursor,
	}))
}

// QuoteNew — GET /w/{workspace}/deals/{id}/quotes/new. Form buat header quote.
func (h *Handler) QuoteNew(w http.ResponseWriter, r *http.Request) {
	if !h.requireDealWrite(w, r) {
		return
	}
	ctx := r.Context()
	dealID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadOwnedDeal(w, r, dealID); !ok {
		return
	}
	members, err := h.assignableMembers(ctx)
	if err != nil {
		h.Log.Error("quotes: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Buat Quote", "/deals", panel.QuoteForm(panel.QuoteFormView{
		Base:    base,
		DealID:  dealID,
		Action:  base + quoteListSub(dealID),
		IsEdit:  false,
		Err:     wsErrMsg(r.URL.Query().Get("err")),
		Members: members,
	}))
}

// QuoteEdit — GET /w/{workspace}/deals/{id}/quotes/{quoteID}/edit. Form sunting
// HEADER (item disunting di builder). Prefill dari quote termuat.
func (h *Handler) QuoteEdit(w http.ResponseWriter, r *http.Request) {
	if !h.requireDealWrite(w, r) {
		return
	}
	ctx := r.Context()
	dealID, quoteID, ok := h.parseQuoteRef(w, r)
	if !ok {
		return
	}
	q, ok := h.loadOwnedQuote(w, r, dealID, quoteID)
	if !ok {
		return
	}
	members, err := h.assignableMembers(ctx)
	if err != nil {
		h.Log.Error("quotes: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Sunting Quote", "/deals", panel.QuoteForm(panel.QuoteFormView{
		Base:    base,
		DealID:  dealID,
		Action:  base + quoteSub(dealID, quoteID),
		IsEdit:  true,
		Err:     wsErrMsg(r.URL.Query().Get("err")),
		Fields:  quoteFormFields(q),
		Members: members,
	}))
}

// quoteRowView memetakan satu quote → baris daftar. Angka SUDAH diformat; status
// mentah (badge diputuskan view). Dipakai daftar quote & kartu quote detail deal.
func quoteRowView(q db.Quote) panel.QuoteRow {
	return panel.QuoteRow{
		ID:         q.ID,
		EntityCode: deref(q.EntityCode),
		QuoteName:  deref(q.QuoteName),
		Status:     q.QuoteStatus,
		GrandTotal: formatRupiah(q.GrandTotal),
		Expiration: dateStr(q.ExpirationDate),
	}
}
