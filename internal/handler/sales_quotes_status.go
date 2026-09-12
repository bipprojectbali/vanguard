package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// sales_quotes_status.go — AKSI transisi status Quote, dipisah dari
// sales_quotes_update.go (QuoteUpdate/QuoteDelete) untuk file health; gerbang
// & konvensi sama. Enum status (validQuoteStatuses/quoteStatusOptions) juga di
// sini — satu tempat dengan aksi transisi yang memvalidasinya.

// quoteInitialStatus = status setiap quote baru. Transisi berikutnya = aksi manual
// tersendiri (UpdateQuoteStatus); approval flow ditunda (keputusan scope).
const quoteInitialStatus = "Draft"

// validQuoteStatuses = himpunan status legal (cermin quotes_status_chk 00010).
// Approval flow ditunda → semua status boleh diset manual dari kontrol status.
var validQuoteStatuses = map[string]struct{}{
	"Draft": {}, "Sent": {}, "Under Review": {},
	"Accepted": {}, "Rejected": {}, "Expired": {},
}

// quoteStatusOptions = status untuk dropdown, BERURUT (map validasi tak berurut).
// Harus himpunan yang sama dengan validQuoteStatuses & CHECK 00010.
var quoteStatusOptions = []string{"Draft", "Sent", "Under Review", "Accepted", "Rejected", "Expired"}

// compile-time: opsi & map validasi sepakat (panjang sama). Berbeda = dropdown
// menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(quoteStatusOptions) != len(validQuoteStatuses) {
		panic("quotes: opsi status tak sinkron dengan map validasi")
	}
	return struct{}{}
}()

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

	// BL-148: quote tanpa line item tak boleh maju dari Draft (belum ada yang
	// ditawarkan). Backend = penjaga sesungguhnya; view hanya membatasi opsi.
	if status != quoteInitialStatus {
		items, err := h.q(ctx).ListQuoteItems(ctx, quoteID)
		if err != nil {
			h.Log.Error("quotes: status item-check", "err", err)
			wsRedirect(w, r, quoteSub(dealID, quoteID), "failed")
			return
		}
		if len(items) == 0 {
			wsRedirect(w, r, quoteSub(dealID, quoteID), "quote_no_items")
			return
		}
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

	// BL-88: quote di-Accept → quote jadi SUMBER KEBENARAN komersial: salin
	// grand_total → deal.amount (nilai DIAKUI, menggantikan perkiraan manual). Fail-soft:
	// gagal di-Log, tak batalkan Accept. (Backfill plan_requested_id single-plan PR1
	// di-retire PR2b — paket kini diturunkan langsung dari quote_items saat Closed Won.)
	if status == "Accepted" {
		h.recognizeDealValueFromQuote(ctx, dealID, quoteID, q.GrandTotal, uid)
	}
	wsRedirectOK(w, r, quoteSub(dealID, quoteID), "status")
}
