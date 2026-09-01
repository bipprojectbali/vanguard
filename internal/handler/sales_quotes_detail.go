package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_quotes_detail.go — HALAMAN detail satu Quote (Quote Builder): header +
// kartu identitas + tabel line items + Subtotal/Tax/Grand + (bila boleh tulis)
// form tambah item, sunting/hapus per-item, kontrol status. Dipecah dari
// sales_quotes_page.go (daftar+form header) semata untuk file health; label plan
// ada di sales_quotes_planlabel.go (dipecah lebih lanjut). F3 via loadOwnedQuote.
// Semua angka diformat di sini (view murni-data).

// QuoteDetail — GET /w/{workspace}/deals/{id}/quotes/{quoteID}.
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
	q, d, ok := h.loadOwnedQuote(w, r, dealID, quoteID)
	if !ok {
		return
	}
	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, quoteTitle(q), "/deals",
		panel.QuoteDetail(h.quoteDetailView(ctx, base, dealID, q, d.Stage)))
}

// quoteDetailView merakit builder lengkap: header, identitas (deal/account/
// prepared_by/terms), baris item (label plan diresolusi), total, dan opsi picker
// plan (untuk form tambah item). Semua angka diformat di sini (view murni-data).
// F4: Subtotal/Tax/GrandTotal/UnitPrice nilai komersial → maskARR (kebijakan
// umum kecuali Support). Diperbaiki audit FLS M9-1; laten krn Support F2-blocked
// total dari crm:deals (business_policy.csv) — defense in depth.
func (h *Handler) quoteDetailView(ctx context.Context, base string, dealID int64, q db.Quote, stage string) panel.QuoteDetailView {
	br := session.BusinessRole(ctx)
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
		itemRows = append(itemRows, quoteItemRowView(it, labels, br))
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

	taxMode, taxRateInput, taxAmountInput, taxLabel := quoteTaxView(q)

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
		Subtotal:     maskARR(formatRupiah(subtotal), br),
		Tax:          maskARR(formatRupiah(q.TaxAmount), br),
		GrandTotal:   maskARR(formatRupiah(q.GrandTotal), br),

		TaxMode:        taxMode,
		TaxRateInput:   taxRateInput,
		TaxAmountInput: taxAmountInput,
		TaxLabel:       taxLabel,

		Items:        itemRows,
		Plans:        planOpts,
		CanWrite:     canWriteDeals(ctx),
		Quotable:     quotableStage(stage), // BL-13: gate stage (mutasi vs arsip)
		StageLockMsg: stageLockMsg(stage),
		Expired:      quoteExpired(q.QuoteStatus, q.ExpirationDate, todayInAppTZ()), // BL-17
	}
}

// quoteItemRowView memetakan satu quote_item → baris tabel builder. Angka
// diformat; qty/diskon mentah dipertahankan untuk prefill form sunting. F4:
// UnitPrice/Subtotal → maskARR (lihat quoteDetailView).
func quoteItemRowView(it db.QuoteItem, labels map[int64]string, businessRole string) panel.QuoteItemRow {
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
		UnitPrice: maskARR(formatRupiah(it.UnitPrice), businessRole),
		Discount:  numericStr(it.DiscountPct),
		Subtotal:  maskARR(formatRupiah(it.Subtotal), businessRole),
	}
}
