package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// sales_quotes_update.go — AKSI sunting/status/hapus Quote + recomputeTotals.
// Dipecah dari sales_quotes.go (create) untuk file health; gerbang & konvensi sama.

// QuoteUpdate — POST /w/{workspace}/deals/{id}/quotes/{quoteID}. Menyimpan sunting
// HEADER. account_id & deal_id DIPERTAHANKAN (warisan deal, tak lewat form). Pajak
// TIDAK lagi di header (BL-14) — dikelola di builder via QuoteTax, jadi update header
// tak menyentuh total. Status punya jalur tersendiri (QuoteStatus).
func (h *Handler) QuoteUpdate(w http.ResponseWriter, r *http.Request) {
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
	// BL-13: simpan sunting header hanya di jendela quoting.
	if !h.requireQuotableStage(w, r, d.Stage, quoteSub(dealID, quoteID)) {
		return
	}
	form, errCode := parseQuoteForm(r.FormValue, todayInAppTZ())
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
	_, d, ok := h.loadOwnedQuote(w, r, dealID, quoteID)
	if !ok {
		return
	}
	// BL-13: transisi status quote ikut di-gate agar terminal benar-benar beku.
	if !h.requireQuotableStage(w, r, d.Stage, quoteSub(dealID, quoteID)) {
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

	// BL-100 (Fix B): quote di-Accept → salin paket ke deal induk agar Closed Won bisa
	// membuat langganan (subscriptionFromWonDeal baca deals.plan_requested_id yang tak
	// pernah diisi jalur UI). Fail-soft: kegagalan di-Log, tak menggagalkan Accept.
	if status == "Accepted" {
		h.backfillDealPlanFromQuote(ctx, dealID, quoteID, uid)
	}
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
	_, d, ok := h.loadOwnedQuote(w, r, dealID, quoteID)
	if !ok {
		return
	}
	// BL-13: hapus quote hanya di jendela quoting (terminal = arsip, read-only).
	if !h.requireQuotableStage(w, r, d.Stage, quoteSub(dealID, quoteID)) {
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
