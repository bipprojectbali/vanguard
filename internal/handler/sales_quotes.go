package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// sales_quotes.go — AKSI atas Quote (penawaran), ter-NEST di bawah deal
// (/deals/{id}/quotes...): path helper, F3 loader, create, & quoteTitle. Sunting/
// status/hapus ada di sales_quotes_update.go (dipecah untuk file health, package
// sama). Gerbang tulis = requireDealWrite (SAMA dgn deals: quote mewarisi sumbu
// bisnis "crm:deals"). Ownership F3 DIWARISI dari deal induk lewat loadOwnedQuote
// (loadOwnedDeal → 404 di luar cakupan) — tak ada kolom quote_owner.
// SNAPSHOT harga (acceptance M4): plans.base_price disalin ke quote_items.unit_price
// & beku saat item ditambah; grand_total = Σ subtotal + tax_amount, tax_amount kini
// snapshot hasil konfigurasi pajak builder (BL-14), bukan input header.
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

// loadOwnedQuote memuat satu quote & DEAL INDUKNYA, menegakkan F3 lewat deal. Quote
// tak menyaring ownership sendiri; keputusan "boleh lihat?" diambil dari deal induk
// (loadOwnedDeal → 404 di luar cakupan) — sumber yang sama dengan halaman deal.
// dealID dari URL WAJIB cocok dengan quote.deal_id: quote dari deal lain → 404. Deal
// dikembalikan agar pemanggil punya deal.Stage (gerbang quotable BL-13) tanpa query ulang.
func (h *Handler) loadOwnedQuote(w http.ResponseWriter, r *http.Request, dealID, quoteID int64) (db.Quote, db.Deal, bool) {
	ctx := r.Context()
	q, err := h.q(ctx).GetQuote(ctx, quoteID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.Quote{}, db.Deal{}, false
		}
		h.Log.Error("quotes: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.Quote{}, db.Deal{}, false
	}
	if q.DealID == nil || *q.DealID != dealID {
		http.NotFound(w, r)
		return db.Quote{}, db.Deal{}, false
	}
	// Warisan F3: keputusan atas DEAL INDUK (loadOwnedDeal → 404 bila di luar cakupan).
	d, ok := h.loadOwnedDeal(w, r, dealID)
	if !ok {
		return db.Quote{}, db.Deal{}, false
	}
	return q, d, true
}

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
		CreatedBy:      &uid,
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

// quoteTitle = judul halaman builder: nama quote bila ada, jatuh ke kode, lalu
// label generik. Tak pernah kosong (judul shell butuh teks).
func quoteTitle(q db.Quote) string {
	if q.QuoteName != nil && *q.QuoteName != "" {
		return *q.QuoteName
	}
	if q.EntityCode != nil && *q.EntityCode != "" {
		return *q.EntityCode
	}
	return "Quote"
}
