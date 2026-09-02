package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// sales_quotes_form_page.go — form GET buat/sunting HEADER quote (QuoteNew,
// QuoteEdit), dipisah dari sales_quotes_page.go (daftar + quoteRowView) demi
// ambang tipe Route/Handler (150). Package sama; gate/ownership identik.

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
