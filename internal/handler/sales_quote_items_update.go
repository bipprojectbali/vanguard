package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// sales_quote_items_update.go — AKSI SUNTING/HAPUS baris quote (quote_items).
// Dipecah dari sales_quote_items.go (tambah + loader) semata untuk file health;
// gerbang & konvensi sama persis.

// QuoteItemUpdate — POST .../items/{itemID}. Sunting qty/diskon; subtotal dihitung
// ulang dari unit_price SNAPSHOT (tak berubah). Total quote direkalkulasi.
func (h *Handler) QuoteItemUpdate(w http.ResponseWriter, r *http.Request) {
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
	// BL-13: ubah/hapus item hanya di jendela quoting.
	if !h.requireQuotableStage(w, r, d.Stage, quoteSub(dealID, quoteID)) {
		return
	}
	item, ok := h.loadQuoteItem(w, r, quoteID)
	if !ok {
		return
	}
	form, errCode := parseQuoteItemEditForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, quoteSub(dealID, quoteID), errCode)
		return
	}

	// unit_price SNAPSHOT tetap; subtotal ikut qty/diskon baru.
	subtotal := itemSubtotal(item.UnitPrice, form.Quantity, form.DiscountPct)
	uid := session.UserID(ctx)
	if _, err := h.q(ctx).UpdateQuoteItem(ctx, db.UpdateQuoteItemParams{
		Quantity:    form.Quantity,
		DiscountPct: form.DiscountPct,
		Subtotal:    subtotal,
		LineNo:      item.LineNo, // pertahankan urutan
		ID:          item.ID,
	}); err != nil {
		h.Log.Error("quotes: update item", "err", err)
		wsRedirect(w, r, quoteSub(dealID, quoteID), "failed")
		return
	}
	if err := h.recomputeTotals(ctx, quoteID, q.TaxAmount, uid); err != nil {
		h.Log.Error("quotes: recompute", "err", err)
		wsRedirect(w, r, quoteSub(dealID, quoteID), "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "quote.item.update", session.TenantID(ctx), map[string]string{
		"quote_id": strconv.FormatInt(quoteID, 10), "item_id": strconv.FormatInt(item.ID, 10),
	})
	wsRedirectOK(w, r, quoteSub(dealID, quoteID), "item_saved")
}

// QuoteItemDelete — POST .../items/{itemID}/delete. Hard-delete satu baris (tanpa
// soft-delete), lalu rekalkulasi total quote.
func (h *Handler) QuoteItemDelete(w http.ResponseWriter, r *http.Request) {
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
	// BL-13: ubah/hapus item hanya di jendela quoting.
	if !h.requireQuotableStage(w, r, d.Stage, quoteSub(dealID, quoteID)) {
		return
	}
	item, ok := h.loadQuoteItem(w, r, quoteID)
	if !ok {
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).DeleteQuoteItem(ctx, item.ID); err != nil {
		h.Log.Error("quotes: delete item", "err", err)
		wsRedirect(w, r, quoteSub(dealID, quoteID), "failed")
		return
	}
	if err := h.recomputeTotals(ctx, quoteID, q.TaxAmount, uid); err != nil {
		h.Log.Error("quotes: recompute", "err", err)
		wsRedirect(w, r, quoteSub(dealID, quoteID), "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "quote.item.delete", session.TenantID(ctx), map[string]string{
		"quote_id": strconv.FormatInt(quoteID, 10), "item_id": strconv.FormatInt(item.ID, 10),
	})
	wsRedirectOK(w, r, quoteSub(dealID, quoteID), "item_deleted")
}
