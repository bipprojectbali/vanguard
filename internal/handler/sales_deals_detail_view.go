package handler

import (
	"context"
	"errors"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// sales_deals_detail_view.go — rakitan view detail Deal (cuplikan quote &
// pemetaan lengkap), dipisah dari sales_deals_detail.go (handler halaman)
// untuk file health.

// dealQuotesPreview memuat cuplikan quote deal ini untuk kartu di detail deal.
// Best-effort (mirror accountLabel): gagal query → nil + log, detail deal tetap
// terbaca; daftar penuh berkeyset ada di /deals/{id}/quotes.
func (h *Handler) dealQuotesPreview(ctx context.Context, dealID int64) []panel.QuoteRow {
	at, id := firstPageCursor()
	rows, err := h.q(ctx).ListQuotesForDeal(ctx, db.ListQuotesForDealParams{
		DealID:          &dealID,
		CursorCreatedAt: at,
		CursorID:        id,
		PageSize:        dealQuotesPreviewLimit,
	})
	if err != nil {
		h.Log.Error("deals: quotes preview", "err", err)
		return nil
	}
	today := todayInAppTZ() // BL-17: konsisten dengan daftar quote penuh
	out := make([]panel.QuoteRow, 0, len(rows))
	for _, q := range rows {
		out = append(out, quoteRowView(q, today))
	}
	return out
}

// dealDetailView merakit detail lengkap + F4 (amount tersamar). Nama account &
// kontak utama diresolusi best-effort (baris di luar tenant/terhapus → label
// cadangan, tak menggagalkan halaman).
func (h *Handler) dealDetailView(ctx context.Context, base string, d db.Deal, names map[int64]string) panel.DealDetailView {
	accountLabel := h.accountLabel(ctx, d.AccountID)
	contactLabel := ""
	if d.PrimaryContactID != nil {
		contactLabel = h.contactLabel(ctx, *d.PrimaryContactID)
	}
	// BL-88: label nilai deal bergantung sumbernya, dan termin langganan kini
	// milik quote (bukan deal.SubscriptionTerm yang tak lagi diisi form). Bila deal
	// sudah punya quote Accepted, deal.Amount = grand_total quote (nilai DIAKUI,
	// otoritatif) & termin diambil dari quote itu. Tanpa quote Accepted, Amount
	// masih perkiraan manual & termin kosong (belum ditetapkan di quote).
	// Best-effort (mirror accountLabel): gagal query → anggap belum diakui.
	amountLabel := "Nilai perkiraan"
	subscriptionTerm := ""
	if aq, err := h.q(ctx).GetAcceptedQuoteForDeal(ctx, &d.ID); err == nil {
		amountLabel = "Nilai diakui (dari quote)"
		subscriptionTerm = deref(aq.SubscriptionTerm)
	} else if !errors.Is(err, pgx.ErrNoRows) {
		h.Log.Error("deals: accepted-quote label", "err", err)
	}
	return panel.DealDetailView{
		Base:           base,
		ID:             d.ID,
		EntityCode:     deref(d.EntityCode),
		DealName:       d.DealName,
		AccountID:      d.AccountID,
		AccountLabel:   accountLabel,
		PrimaryContact: contactLabel,
		Stage:          d.Stage,
		Stages:         dealStageOptions,
		NextStages:     nextDealStages(d.Stage), // BL-159: sequential-only

		WonSubStatuses:   wonSubStatusOptions,
		DealType:         deref(d.DealType),
		Amount:           maskARR(formatRupiah(d.Amount), canSeeARR(ctx)),
		AmountLabel:      amountLabel,
		Probability:      probabilityStr(d.Probability),
		ExpectedClose:    dateStr(d.ExpectedCloseDate),
		ForecastCategory: deref(d.ForecastCategory),
		NextStep:         deref(d.NextStep),
		SubscriptionTerm: subscriptionTerm,
		Competitor:       deref(d.Competitor),
		WinLossReason:    deref(d.WinLossReason),
		LossReasonCode:   deref(d.LossReasonCode),
		LossReasonCodes:  lossReasonCodeOptions,
		ClosedDate:       dateStr(d.ClosedDate),
		LossNotes:        deref(d.LossNotes),
		LastActiveStage:  deref(d.LastActiveStage),
		Owner:            ownerName(d.DealOwner, names),
		CanWrite:         canWriteDeals(ctx),
		CreatedByName:    ownerName(d.CreatedBy, names),
		CreatedAt:        fmtDateTime(d.CreatedAt),
		UpdatedByName:    ownerName(d.UpdatedBy, names),
		UpdatedAt:        fmtDateTime(d.UpdatedAt),
		// BL-86: tombol "Buat Quote" hanya saat boleh tulis DAN stage quotable
		// (reuse quotableStage/stageLockMsg BL-13, jangan literal stage di view).
		CanCreateQuote:    canWriteDeals(ctx) && quotableStage(d.Stage),
		QuoteStageLockMsg: stageLockMsg(d.Stage),
		Quotes:            h.dealQuotesPreview(ctx, d.ID),
		QuotesSummary:     h.quotesSummaryForDeal(ctx, d.ID),
		Activities:        h.activitiesTimelineFor(ctx, base, "deal", d.ID, canWriteDeals(ctx)),
	}
}
