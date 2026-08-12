package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_quotes_page.go — HALAMAN baca Quote (Quote Builder), ter-NEST di bawah
// deal. Aksi ada di sales_quotes.go / sales_quote_items.go. Dipisah baca/tulis
// (meniru sales_deals_page.go): halaman tumbuh dengan aturan LIHAT (gate baca +
// F3 warisan deal), aksi dengan aturan TULIS. Gate baca = canViewDeals (quote
// mewarisi sumbu bisnis "crm:deals"); ownership dari deal induk (loadOwnedQuote/
// loadOwnedDeal). Angka SUDAH diformat handler (view murni-data).

// QuotesList — GET /w/{workspace}/deals/{id}/quotes. Daftar quote SATU deal
// (keyset). Bukan pemegang peran CRM → 403. Deal di luar cakupan F3 → 404.
func (h *Handler) QuotesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewDeals(ctx) {
		h.renderDealsForbidden(w, r)
		return
	}
	dealID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	d, ok := h.loadOwnedDeal(w, r, dealID)
	if !ok {
		return
	}

	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListQuotesForDeal(ctx, db.ListQuotesForDealParams{
		DealID:          &dealID,
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("quotes: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	shown, nextCursor := splitPage(rows, func(q db.Quote) (pgtype.Timestamptz, int64) {
		return q.CreatedAt, q.ID
	})
	items := make([]panel.QuoteRow, 0, len(shown))
	for _, q := range shown {
		items = append(items, quoteRowView(q))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Quote — "+d.DealName, "/deals", panel.QuotesList(panel.QuotesListView{
		Base:       base,
		DealID:     dealID,
		DealName:   d.DealName,
		CanWrite:   canWriteDeals(ctx),
		Err:        wsErrMsg(r.URL.Query().Get("err")),
		Msg:        quotesMsg(r.URL.Query().Get("ok")),
		Items:      items,
		NextCursor: nextCursor,
	}))
}

// QuoteDetail — GET /w/{workspace}/deals/{id}/quotes/{quoteID}. Builder: header
// + kartu identitas + tabel line items + Subtotal/Tax/Grand + (bila boleh tulis)
// form tambah item, sunting/hapus per-item, kontrol status. F3 via loadOwnedQuote.
func (h *Handler) QuoteDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewDeals(ctx) {
		h.renderDealsForbidden(w, r)
		return
	}
	dealID, quoteID, ok := h.parseQuoteRef(w, r)
	if !ok {
		return
	}
	q, ok := h.loadOwnedQuote(w, r, dealID, quoteID)
	if !ok {
		return
	}
	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, quoteTitle(q), "/deals",
		panel.QuoteDetail(h.quoteDetailView(ctx, base, dealID, q)))
}

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
	if _, ok := h.loadOwnedDeal(w, r, dealID); !ok {
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
		Base:    base,
		DealID:  dealID,
		Action:  base + quoteListSub(dealID),
		IsEdit:  false,
		Err:     wsErrMsg(r.URL.Query().Get("err")),
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
	q, ok := h.loadOwnedQuote(w, r, dealID, quoteID)
	if !ok {
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
		Fields:  quoteFormFields(q),
		Members: members,
	}))
}

// quoteRowView memetakan satu quote → baris daftar. Angka SUDAH diformat; status
// mentah (badge diputuskan view). Dipakai daftar quote & kartu quote detail deal.
func quoteRowView(q db.Quote) panel.QuoteRow {
	return panel.QuoteRow{
		ID:         q.ID,
		EntityCode: deref(q.EntityCode),
		QuoteName:  deref(q.QuoteName),
		Status:     q.QuoteStatus,
		GrandTotal: formatRupiah(q.GrandTotal),
		Expiration: dateStr(q.ExpirationDate),
	}
}

// quoteDetailView merakit builder lengkap: header, identitas (deal/account/
// prepared_by/terms), baris item (label plan diresolusi), total, dan opsi picker
// plan (untuk form tambah item). Semua angka diformat di sini (view murni-data).
func (h *Handler) quoteDetailView(ctx context.Context, base string, dealID int64, q db.Quote) panel.QuoteDetailView {
	items, err := h.q(ctx).ListQuoteItems(ctx, q.ID)
	if err != nil {
		h.Log.Error("quotes: list items", "err", err)
		items = nil // builder tetap terbaca tanpa baris; galat sudah ter-log
	}

	plans, err := h.q(ctx).ListPlans(ctx)
	if err != nil {
		h.Log.Error("quotes: list plans", "err", err)
		plans = nil
	}
	labels := h.quotePlanLabels(ctx, items, plans)

	// Subtotal = Σ subtotal baris (dihitung app, cermin snapshot grand_total).
	var subtotal pgtype.Numeric
	itemRows := make([]panel.QuoteItemRow, 0, len(items))
	for _, it := range items {
		subtotal = addNumeric(subtotal, it.Subtotal)
		itemRows = append(itemRows, quoteItemRowView(it, labels))
	}

	planOpts := make([]panel.QuotePlanOption, 0, len(plans))
	for _, p := range plans {
		planOpts = append(planOpts, panel.QuotePlanOption{
			ID:    p.ID,
			Label: quotePlanLabel(p),
		})
	}

	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("quotes: members", "err", err)
	}

	return panel.QuoteDetailView{
		Base:         base,
		DealID:       dealID,
		ID:           q.ID,
		EntityCode:   deref(q.EntityCode),
		QuoteName:    deref(q.QuoteName),
		Status:       q.QuoteStatus,
		Statuses:     quoteStatusOptions,
		AccountLabel: h.accountLabel(ctx, q.AccountID),
		Expiration:   dateStr(q.ExpirationDate),
		PreparedBy:   ownerName(q.PreparedBy, names),
		PaymentTerms: deref(q.PaymentTerms),
		NotesTerms:   deref(q.NotesTerms),
		Subtotal:     formatRupiah(subtotal),
		Tax:          formatRupiah(q.TaxAmount),
		GrandTotal:   formatRupiah(q.GrandTotal),
		Items:        itemRows,
		Plans:        planOpts,
		CanWrite:     canWriteDeals(ctx),
	}
}

// quoteItemRowView memetakan satu quote_item → baris tabel builder. Angka
// diformat; qty/diskon mentah dipertahankan untuk prefill form sunting.
func quoteItemRowView(it db.QuoteItem, labels map[int64]string) panel.QuoteItemRow {
	label := "Plan #—"
	if it.PlanID != nil {
		if l, ok := labels[*it.PlanID]; ok {
			label = l
		} else {
			label = "Plan #" + strconv.FormatInt(*it.PlanID, 10)
		}
	}
	return panel.QuoteItemRow{
		ID:        it.ID,
		PlanLabel: label,
		Quantity:  strconv.FormatInt(int64(it.Quantity), 10),
		UnitPrice: formatRupiah(it.UnitPrice),
		Discount:  numericStr(it.DiscountPct),
		Subtotal:  formatRupiah(it.Subtotal),
	}
}

// quotePlanLabels membangun peta plan_id → label untuk baris item. Sumber utama
// ListPlans (satu query); plan yang sudah PENSIUN (is_active=false, tak muncul di
// ListPlans) diresolusi individual via GetPlan — item lama tetap ter-nama. Baris
// per-quote bounded → jumlah GetPlan susulan kecil.
func (h *Handler) quotePlanLabels(ctx context.Context, items []db.QuoteItem, plans []db.Plan) map[int64]string {
	labels := make(map[int64]string, len(plans)+len(items))
	for _, p := range plans {
		labels[p.ID] = quotePlanLabel(p)
	}
	for _, it := range items {
		if it.PlanID == nil {
			continue
		}
		if _, ok := labels[*it.PlanID]; ok {
			continue
		}
		p, err := h.q(ctx).GetPlan(ctx, *it.PlanID)
		if err != nil {
			continue // biarkan pemanggil pakai cadangan "Plan #<id>"
		}
		labels[p.ID] = quotePlanLabel(p)
	}
	return labels
}

// quotePlanLabel = label plan untuk picker/baris: "Nama — Rp harga". Harga = base
// (referensi tampilan; unit_price sebenarnya di-SNAPSHOT saat item dibuat).
func quotePlanLabel(p db.Plan) string {
	label := p.PlanName
	if price := formatRupiah(p.BasePrice); price != "" {
		label += " — " + price
	}
	return label
}

// quoteFormFields memetakan quote termuat → prefill form header (semua string).
func quoteFormFields(q db.Quote) panel.QuoteFormFields {
	preparedBy := ""
	if q.PreparedBy != nil {
		preparedBy = strconv.FormatInt(*q.PreparedBy, 10)
	}
	return panel.QuoteFormFields{
		QuoteName:      deref(q.QuoteName),
		ExpirationDate: dateStr(q.ExpirationDate),
		PaymentTerms:   deref(q.PaymentTerms),
		NotesTerms:     deref(q.NotesTerms),
		TaxAmount:      numericStr(q.TaxAmount),
		PreparedBy:     preparedBy,
	}
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
