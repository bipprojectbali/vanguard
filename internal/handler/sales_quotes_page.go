package handler

import (
	"net/http"
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"
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
	d, ok := h.loadOwnedDeal(w, r, dealID)
	if !ok {
		return
	}
	// BL-13: form buat quote tak berguna di luar jendela quoting → PRG ke daftar.
	if !h.requireQuotableStage(w, r, d.Stage, quoteListSub(dealID)) {
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
		Base:   base,
		DealID: dealID,
		Action: base + quoteListSub(dealID),
		IsEdit: false,
		Err:    wsErrMsg(r.URL.Query().Get("err")),
		// BL-15: pembuat quote hampir selalu = penyusunnya → default "Disusun oleh"
		// ke user aktif (tetap bisa diganti manual). Edit prefill dari nilai tersimpan.
		ExpMin:  todayInAppTZ().Format(dateLayout), // BL-17: min klien = hari ini
		Fields:  panel.QuoteFormFields{PreparedBy: strconv.FormatInt(session.UserID(ctx), 10)},
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
	q, d, ok := h.loadOwnedQuote(w, r, dealID, quoteID)
	if !ok {
		return
	}
	// BL-13: sunting header hanya di jendela quoting → PRG ke detail (arsip terbaca).
	if !h.requireQuotableStage(w, r, d.Stage, quoteSub(dealID, quoteID)) {
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
		ExpMin:  todayInAppTZ().Format(dateLayout), // BL-17: min klien = hari ini
		Fields:  quoteFormFields(q),
		Members: members,
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
