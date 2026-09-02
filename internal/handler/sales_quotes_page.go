package handler

import (
	"net/http"
	"time"

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
	today := todayInAppTZ() // BL-17: satu "hari ini" untuk semua baris halaman
	items := make([]panel.QuoteRow, 0, len(shown))
	for _, q := range shown {
		items = append(items, quoteRowView(q, today))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Quote — "+d.DealName, "/deals", panel.QuotesList(panel.QuotesListView{
		Base:         base,
		DealID:       dealID,
		DealName:     d.DealName,
		CanWrite:     canWriteDeals(ctx),
		Quotable:     quotableStage(d.Stage), // BL-13: gate stage
		StageLockMsg: stageLockMsg(d.Stage),
		Err:          wsErrMsg(r.URL.Query().Get("err")),
		Msg:          quotesMsg(r.URL.Query().Get("ok")),
		Items:        items,
		NextCursor:   nextCursor,
		After:        r.URL.Query().Get("after"),
		Trail:        pageTrail(r),
		Summary:      h.quotesSummaryForDeal(ctx, dealID),
	}))
}

// quoteRowView memetakan satu quote → baris daftar. Angka SUDAH diformat; status
// mentah (badge diputuskan view). Dipakai daftar quote & kartu quote detail deal.
func quoteRowView(q db.Quote, today time.Time) panel.QuoteRow {
	return panel.QuoteRow{
		ID:         q.ID,
		EntityCode: deref(q.EntityCode),
		QuoteName:  deref(q.QuoteName),
		Status:     q.QuoteStatus,
		GrandTotal: formatRupiah(q.GrandTotal),
		Expiration: dateStr(q.ExpirationDate),
		Expired:    quoteExpired(q.QuoteStatus, q.ExpirationDate, today), // BL-17
	}
}
