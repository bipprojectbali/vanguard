package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

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
		QuoteName:          form.QuoteName,
		DealID:             q.DealID,    // pertahankan warisan deal
		AccountID:          q.AccountID, // pertahankan jangkar account
		ExpirationDate:     form.ExpirationDate,
		PaymentTerms:       form.PaymentTerms,
		NotesTerms:         form.NotesTerms,
		PreparedBy:         form.PreparedBy,
		SubscriptionTerm:   form.SubscriptionTerm,
		ContractTermMonths: form.ContractTermMonths,
		UpdatedBy:          &uid,
		ID:                 quoteID,
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
	q, d, ok := h.loadOwnedQuote(w, r, dealID, quoteID)
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

	// BL-88: TEPAT 1 quote Accepted per deal (menghapus "accept terakhir menang").
	// Bila deal sudah punya quote Accepted LAIN → tolak dgn pesan ramah (index parcial
	// idx_quotes_one_accepted = jaring keras bila balapan). Guard hanya untuk transisi
	// KE Accepted; re-Accept quote yang sama lolos (id sama).
	if status == "Accepted" {
		existing, err := h.q(ctx).GetAcceptedQuoteForDeal(ctx, &dealID)
		switch {
		case err == nil && existing.ID != quoteID:
			wsRedirect(w, r, quoteSub(dealID, quoteID), "quote_already_accepted")
			return
		case err != nil && !errors.Is(err, pgx.ErrNoRows):
			h.Log.Error("quotes: accepted-guard", "err", err)
			wsRedirect(w, r, quoteSub(dealID, quoteID), "failed")
			return
		}
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

	// BL-88: quote di-Accept → quote jadi SUMBER KEBENARAN komersial. (a) salin
	// grand_total → deal.amount (nilai DIAKUI, menggantikan perkiraan manual); (b) tetap
	// backfill plan_requested_id agar jalur Won single-plan PR1 bisa membuat langganan
	// (di-retire PR2 multi-baris). Keduanya fail-soft: gagal di-Log, tak batalkan Accept.
	if status == "Accepted" {
		h.recognizeDealValueFromQuote(ctx, dealID, quoteID, q.GrandTotal, uid)
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
