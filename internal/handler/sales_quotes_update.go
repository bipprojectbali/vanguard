package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_quotes_update.go — AKSI sunting/status/hapus Quote + recomputeTotals.
// Dipecah dari sales_quotes.go (create) untuk file health; gerbang & konvensi sama.

// QuoteUpdate — POST /w/{workspace}/deals/{id}/quotes/{quoteID}. Menyimpan sunting
// HEADER. account_id & deal_id DIPERTAHANKAN (warisan deal, tak lewat form). Pajak
// baru dari form → grand_total dihitung ulang (recomputeTotals). Status punya jalur
// tersendiri (QuoteStatus).
func (h *Handler) QuoteUpdate(w http.ResponseWriter, r *http.Request) {
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
	form, errCode := parseQuoteForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, quoteSub(dealID, quoteID)+"/edit", errCode)
		return
	}

	uid := session.UserID(ctx)
	if _, err := h.q(ctx).UpdateQuote(ctx, db.UpdateQuoteParams{
		QuoteName:      form.QuoteName,
		DealID:         q.DealID,    // pertahankan warisan deal
		AccountID:      q.AccountID, // pertahankan jangkar account
		ExpirationDate: form.ExpirationDate,
		PaymentTerms:   form.PaymentTerms,
		NotesTerms:     form.NotesTerms,
		PreparedBy:     form.PreparedBy,
		UpdatedBy:      &uid,
		ID:             quoteID,
	}); err != nil {
		h.Log.Error("quotes: update", "err", err)
		wsRedirect(w, r, quoteSub(dealID, quoteID)+"/edit", "failed")
		return
	}
	// Pajak manual diubah lewat header → grand_total = Σ subtotal + tax baru.
	if err := h.recomputeTotals(ctx, quoteID, form.TaxAmount, uid); err != nil {
		h.Log.Error("quotes: recompute", "err", err)
		wsRedirect(w, r, quoteSub(dealID, quoteID)+"/edit", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "quote.update", session.TenantID(ctx), map[string]string{
		"quote_id": strconv.FormatInt(quoteID, 10),
	})
	wsRedirectOK(w, r, quoteSub(dealID, quoteID), "saved")
}

// QuoteStatus — POST /w/{workspace}/deals/{id}/quotes/{quoteID}/status. Transisi
// status MANUAL (approval flow ditunda). Nilai divalidasi terhadap allowlist skema
// (validQuoteStatuses) di handler agar pesan bisa diperbaiki user; CHECK jaring akhir.
func (h *Handler) QuoteStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireDealWrite(w, r) {
		return
	}
	ctx := r.Context()
	dealID, quoteID, ok := h.parseQuoteRef(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadOwnedQuote(w, r, dealID, quoteID); !ok {
		return
	}
	status := r.FormValue("quote_status")
	if _, valid := validQuoteStatuses[status]; !valid {
		wsRedirect(w, r, quoteSub(dealID, quoteID), "status")
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).UpdateQuoteStatus(ctx, db.UpdateQuoteStatusParams{
		QuoteStatus: status, UpdatedBy: &uid, ID: quoteID,
	}); err != nil {
		h.Log.Error("quotes: status", "err", err)
		wsRedirect(w, r, quoteSub(dealID, quoteID), "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "quote.status", session.TenantID(ctx), map[string]string{
		"quote_id": strconv.FormatInt(quoteID, 10), "status": status,
	})
	wsRedirectOK(w, r, quoteSub(dealID, quoteID), "status")
}

// QuoteDelete — POST /w/{workspace}/deals/{id}/quotes/{quoteID}/delete. Soft-delete
// (deleted_at). Item anaknya TAK dihapus (tetap tersimpan; hanya header disembunyikan).
func (h *Handler) QuoteDelete(w http.ResponseWriter, r *http.Request) {
	if !h.requireDealWrite(w, r) {
		return
	}
	ctx := r.Context()
	dealID, quoteID, ok := h.parseQuoteRef(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadOwnedQuote(w, r, dealID, quoteID); !ok {
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).SoftDeleteQuote(ctx, db.SoftDeleteQuoteParams{
		UpdatedBy: &uid, ID: quoteID,
	}); err != nil {
		h.Log.Error("quotes: delete", "err", err)
		wsRedirect(w, r, quoteSub(dealID, quoteID), "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "quote.delete", session.TenantID(ctx), map[string]string{
		"quote_id": strconv.FormatInt(quoteID, 10),
	})
	// Kembali ke DAFTAR quote (bukan detail deal): daftar punya slot pesan → user
	// melihat konfirmasi; detail quote yang dihapus tak lagi bisa dibuka.
	wsRedirectOK(w, r, quoteListSub(dealID), "quote_deleted")
}

// recomputeTotals: grand_total = Σ subtotal item + tax. Dipanggil tiap item berubah
// (tax = tax quote saat ini) & tiap header disunting (tax = nilai baru dari form).
func (h *Handler) recomputeTotals(ctx context.Context, quoteID int64, tax pgtype.Numeric, actorID int64) error {
	sub, err := h.q(ctx).QuoteItemsSubtotal(ctx, quoteID)
	if err != nil {
		return err
	}
	return h.q(ctx).UpdateQuoteTotals(ctx, db.UpdateQuoteTotalsParams{
		GrandTotal: addNumeric(sub, tax),
		TaxAmount:  tax,
		UpdatedBy:  &actorID,
		ID:         quoteID,
	})
}
