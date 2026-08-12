package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// sales_quotes.go — AKSI atas Quote (penawaran) & barisnya, ter-NEST di bawah deal
// (/deals/{id}/quotes...). Gerbang tulis = requireDealWrite (SAMA dgn deals: quote
// mewarisi sumbu bisnis "crm:deals"). Ownership F3 DIWARISI dari deal induk lewat
// loadOwnedQuote (loadOwnedDeal → 404 di luar cakupan) — tak ada kolom quote_owner.
//
// SNAPSHOT harga (acceptance M4): saat item ditambah, plans.base_price disalin ke
// quote_items.unit_price & beku. subtotal/grand_total dihitung app (sales_money.go),
// bukan agregat live. Pajak = INPUT MANUAL; grand_total = Σ subtotal + tax_amount.
// Navigasi = native POST → 303 (gotcha #16), BUKAN sse.Redirect (diblokir CSP).

// quoteListSub / quoteSub = pembentuk sub-path relatif /w/{slug} untuk redirect.
// Satu tempat literal segmen quote agar tak tersebar (Rule 15).
func quoteListSub(dealID int64) string {
	return "/deals/" + strconv.FormatInt(dealID, 10) + "/quotes"
}
func quoteSub(dealID, quoteID int64) string {
	return quoteListSub(dealID) + "/" + strconv.FormatInt(quoteID, 10)
}

// parseQuoteRef membaca {id} (deal induk) & {quoteID} dari URL nested.
func (h *Handler) parseQuoteRef(w http.ResponseWriter, r *http.Request) (dealID, quoteID int64, ok bool) {
	dealID, ok = h.parseTargetID(w, r)
	if !ok {
		return 0, 0, false
	}
	quoteID, err := strconv.ParseInt(chi.URLParam(r, "quoteID"), 10, 64)
	if err != nil {
		http.Error(w, "id quote tidak valid", http.StatusBadRequest)
		return 0, 0, false
	}
	return dealID, quoteID, true
}

// loadOwnedQuote memuat satu quote & menegakkan F3 lewat DEAL INDUK. Quote tak
// menyaring ownership sendiri; keputusan "boleh lihat?" diambil dari deal induknya
// (loadOwnedDeal → 404 di luar cakupan) — sumber yang sama dengan halaman deal.
// dealID dari URL WAJIB cocok dengan quote.deal_id: quote dari deal lain → 404.
func (h *Handler) loadOwnedQuote(w http.ResponseWriter, r *http.Request, dealID, quoteID int64) (db.Quote, bool) {
	ctx := r.Context()
	q, err := h.q(ctx).GetQuote(ctx, quoteID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.Quote{}, false
		}
		h.Log.Error("quotes: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.Quote{}, false
	}
	if q.DealID == nil || *q.DealID != dealID {
		http.NotFound(w, r)
		return db.Quote{}, false
	}
	// Warisan F3: keputusan atas DEAL INDUK (loadOwnedDeal → 404 bila di luar cakupan).
	if _, ok := h.loadOwnedDeal(w, r, dealID); !ok {
		return db.Quote{}, false
	}
	return q, true
}

// QuoteCreate — POST /w/{workspace}/deals/{id}/quotes. Membuat quote untuk deal ini.
// account_id & deal_id DIWARISI dari deal induk (bukan dari form). entity_code
// dialokasikan DALAM tx ber-tenant (atomik). prepared_by default = pembuat bila
// tak dipilih. Belum ada item → grand_total = tax_amount.
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
	form, errCode := parseQuoteForm(r.FormValue)
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
		TenantID:       tenantID,
		EntityCode:     &code,
		DealID:         &dealID,     // warisan: quote menempel ke deal ini
		AccountID:      d.AccountID, // warisan: jangkar account deal
		QuoteName:      form.QuoteName,
		QuoteStatus:    quoteInitialStatus,
		ExpirationDate: form.ExpirationDate,
		PaymentTerms:   form.PaymentTerms,
		NotesTerms:     form.NotesTerms,
		PreparedBy:     preparedBy,
		GrandTotal:     form.TaxAmount, // belum ada item → grand = tax
		TaxAmount:      form.TaxAmount,
		CreatedBy:      &uid,
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

// recomputeTotals menulis ulang total quote (snapshot): grand_total = Σ subtotal item
// + tax. Dipanggil tiap item berubah (tax = tax quote saat ini) & tiap header disunting
// (tax = nilai baru dari form). QuoteItemsSubtotal menjumlah di DB (satu round-trip).
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
