package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// sales_quotes_create.go — AKSI buat Quote (QuoteCreate), dipisah dari
// sales_quotes.go (path helper, F3 loader, quoteTitle) demi ambang tipe
// Route/Handler (150). Package sama; gerbang tulis & warisan F3 identik.

// QuoteCreate — POST /w/{workspace}/deals/{id}/quotes. Membuat quote untuk deal ini.
// account_id & deal_id DIWARISI dari deal induk (bukan dari form). entity_code
// dialokasikan DALAM tx ber-tenant (atomik). prepared_by default = pembuat bila
// tak dipilih. Quote baru lahir TANPA pajak (grand_total/tax_amount NULL, tax_mode
// default 'amount') — pajak diset di builder lewat QuoteTax setelah ada subtotal.
func (h *Handler) QuoteCreate(w http.ResponseWriter, r *http.Request) {
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
	// BL-13: quote hanya boleh dibuat saat deal di jendela quoting (Qualification–Negotiation).
	if !h.requireQuotableStage(w, r, d.Stage, quoteListSub(dealID)) {
		return
	}
	form, errCode := parseQuoteForm(r.FormValue, todayInAppTZ())
	if errCode != "" {
		wsRedirect(w, r, quoteListSub(dealID)+"/new", errCode)
		return
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)
	preparedBy := form.PreparedBy
	if preparedBy == nil {
		preparedBy = &uid // pembuat = penyusun default
	}

	code, err := h.q(ctx).GenerateEntityCode(ctx, tenantID, codes.EntityQuote)
	if err != nil {
		h.Log.Error("quotes: generate code", "err", err)
		wsRedirect(w, r, quoteListSub(dealID)+"/new", "failed")
		return
	}

	q, err := h.q(ctx).CreateQuote(ctx, db.CreateQuoteParams{
		TenantID:           tenantID,
		EntityCode:         &code,
		DealID:             &dealID,     // warisan: quote menempel ke deal ini
		AccountID:          d.AccountID, // warisan: jangkar account deal
		QuoteName:          form.QuoteName,
		QuoteStatus:        quoteInitialStatus,
		ExpirationDate:     form.ExpirationDate,
		PaymentTerms:       form.PaymentTerms,
		NotesTerms:         form.NotesTerms,
		PreparedBy:         preparedBy,
		SubscriptionTerm:   form.SubscriptionTerm,
		ContractTermMonths: form.ContractTermMonths,
		CreatedBy:          &uid,
		// GrandTotal/TaxAmount sengaja tak diisi (NULL): quote baru tanpa item &
		// tanpa pajak. tax_mode default DB 'amount'. Pajak diset lewat QuoteTax.
	})
	if err != nil {
		h.Log.Error("quotes: create", "err", err)
		wsRedirect(w, r, quoteListSub(dealID)+"/new", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "quote.create", tenantID, map[string]string{
		"quote_id": strconv.FormatInt(q.ID, 10), "deal_id": strconv.FormatInt(dealID, 10), "code": code,
	})
	wsRedirectOK(w, r, quoteSub(dealID, q.ID), "created")
}
