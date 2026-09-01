package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/go-chi/chi/v5"
)

// sales_quote_items.go — AKSI TAMBAH baris quote (quote_items) + loader,
// ter-NEST di bawah quote (/deals/{id}/quotes/{quoteID}/items...). Sunting/hapus
// ada di sales_quote_items_update.go (dipecah semata untuk file health, package
// sama). Gerbang & ownership SAMA dgn quote (requireDealWrite + loadOwnedQuote).
// Tiap perubahan item memicu recomputeTotals (snapshot grand_total). unit_price =
// SNAPSHOT plans.base_price saat item DIBUAT; sunting hanya qty/diskon → unit_price
// tak berubah (harga beku, acceptance M4).

// QuoteItemAdd — POST /w/{workspace}/deals/{id}/quotes/{quoteID}/items. Menyalin
// plans.base_price → unit_price (SNAPSHOT), menghitung subtotal, lalu rekalkulasi
// total. line_no = max(existing)+1 (baris baru di bawah).
func (h *Handler) QuoteItemAdd(w http.ResponseWriter, r *http.Request) {
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
	// BL-13: tambah item hanya di jendela quoting (Qualification–Negotiation).
	if !h.requireQuotableStage(w, r, d.Stage, quoteSub(dealID, quoteID)) {
		return
	}
	form, errCode := parseQuoteItemForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, quoteSub(dealID, quoteID), errCode)
		return
	}

	// Harga di-SNAPSHOT dari plan (RLS menjamin tenant). Plan tak ditemukan → plan_req.
	plan, err := h.q(ctx).GetPlan(ctx, form.PlanID)
	if err != nil {
		wsRedirect(w, r, quoteSub(dealID, quoteID), "plan_req")
		return
	}
	unitPrice := ratToNumeric(ratFromNumeric(plan.BasePrice)) // NULL harga → 0.00; beku sesudahnya
	subtotal := itemSubtotal(unitPrice, form.Quantity, form.DiscountPct)

	items, err := h.q(ctx).ListQuoteItems(ctx, quoteID)
	if err != nil {
		h.Log.Error("quotes: list items", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	nextLine := nextLineNo(items)

	uid := session.UserID(ctx)
	if _, err := h.q(ctx).AddQuoteItem(ctx, db.AddQuoteItemParams{
		QuoteID:     quoteID,
		TenantID:    session.TenantID(ctx),
		PlanID:      &form.PlanID,
		Quantity:    form.Quantity,
		UnitPrice:   unitPrice,
		DiscountPct: form.DiscountPct,
		Subtotal:    subtotal,
		LineNo:      &nextLine,
	}); err != nil {
		h.Log.Error("quotes: add item", "err", err)
		wsRedirect(w, r, quoteSub(dealID, quoteID), "failed")
		return
	}
	if err := h.recomputeTotals(ctx, quoteID, q.TaxMode, q.TaxRate, q.TaxAmount, uid); err != nil {
		h.Log.Error("quotes: recompute", "err", err)
		wsRedirect(w, r, quoteSub(dealID, quoteID), "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "quote.item.add", session.TenantID(ctx), map[string]string{
		"quote_id": strconv.FormatInt(quoteID, 10), "plan_id": strconv.FormatInt(form.PlanID, 10),
	})
	wsRedirectOK(w, r, quoteSub(dealID, quoteID), "item_added")
}

// loadQuoteItem membaca {itemID} & memuat baris yang BENAR milik quoteID (verifikasi
// item.quote_id == quoteID → 404). Tak ada GetQuoteItem tunggal (baris per-quote
// bounded) → ambil dari ListQuoteItems. RLS mengurung tenant di bawahnya.
func (h *Handler) loadQuoteItem(w http.ResponseWriter, r *http.Request, quoteID int64) (db.QuoteItem, bool) {
	ctx := r.Context()
	itemID, err := strconv.ParseInt(chi.URLParam(r, "itemID"), 10, 64)
	if err != nil {
		http.Error(w, "id item tidak valid", http.StatusBadRequest)
		return db.QuoteItem{}, false
	}
	items, err := h.q(ctx).ListQuoteItems(ctx, quoteID)
	if err != nil {
		h.Log.Error("quotes: list items", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.QuoteItem{}, false
	}
	for _, it := range items {
		if it.ID == itemID {
			return it, true
		}
	}
	http.NotFound(w, r)
	return db.QuoteItem{}, false
}

// nextLineNo = max(line_no existing) + 1 (baris baru selalu di bawah, walau ada
// celah dari penghapusan). Item tanpa line_no dihitung 0. Bounded per-quote → aman.
func nextLineNo(items []db.QuoteItem) int16 {
	var max int16
	for _, it := range items {
		if it.LineNo != nil && *it.LineNo > max {
			max = *it.LineNo
		}
	}
	return max + 1
}
