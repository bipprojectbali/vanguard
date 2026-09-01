package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// sales_deals_detail.go — HALAMAN detail satu Deal + pembantu resolusi label.
// Dipisah dari sales_deals_page.go (daftar) untuk file health — detail tumbuh
// dengan aturannya sendiri (resolusi account/kontak/quote best-effort).

// dealQuotesPreviewLimit membatasi jumlah quote yang ditampilkan di kartu
// pratinjau pada detail deal. Daftar penuh (berkeyset) ada di
// /deals/{id}/quotes; kartu ini hanya cuplikan teratas.
const dealQuotesPreviewLimit = 5

// DealDetail — GET /w/{workspace}/deals/{id}. Satu deal. Ownership diputuskan di
// sini (filter.Allows) — kembaran per-baris dari list. Di luar cakupan → 404.
// Nama account & kontak utama diresolusi best-effort (satu query masing-masing,
// bukan N+1); gagal → tautan berlabel kode/id, bukan 500.
func (h *Handler) DealDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewDeals(ctx) {
		h.renderDealsForbidden(w, r)
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	d, err := h.q(ctx).GetDeal(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		h.Log.Error("deals: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	filter := db.DealsListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), d.DealOwner) {
		http.NotFound(w, r)
		return
	}

	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("deals: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, d.DealName, "/deals",
		panel.DealDetail(h.dealDetailView(ctx, base, d, names)))
}

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
	br := session.BusinessRole(ctx)
	accountLabel := h.accountLabel(ctx, d.AccountID)
	contactLabel := ""
	if d.PrimaryContactID != nil {
		contactLabel = h.contactLabel(ctx, *d.PrimaryContactID)
	}
	return panel.DealDetailView{
		Base:             base,
		ID:               d.ID,
		EntityCode:       deref(d.EntityCode),
		DealName:         d.DealName,
		AccountID:        d.AccountID,
		AccountLabel:     accountLabel,
		PrimaryContact:   contactLabel,
		Stage:            d.Stage,
		Stages:           dealStageOptions,
		DealType:         deref(d.DealType),
		Amount:           maskARR(formatRupiah(d.Amount), br),
		Probability:      probabilityStr(d.Probability),
		ExpectedClose:    dateStr(d.ExpectedCloseDate),
		ForecastCategory: deref(d.ForecastCategory),
		NextStep:         deref(d.NextStep),
		SubscriptionTerm: deref(d.SubscriptionTerm),
		Competitor:       deref(d.Competitor),
		WinLossReason:    deref(d.WinLossReason),
		ClosedDate:       dateStr(d.ClosedDate),
		LossNotes:        deref(d.LossNotes),
		Owner:            ownerName(d.DealOwner, names),
		CanWrite:         canWriteDeals(ctx),
		Quotes:           h.dealQuotesPreview(ctx, d.ID),
		QuotesSummary:    h.quotesSummaryForDeal(ctx, d.ID),
		Activities:       h.activitiesTimelineFor(ctx, base, "deal", d.ID, canWriteDeals(ctx)),
	}
}

// accountLabel meresolusi nama desa untuk tautan (kode — nama). Gagal → "Desa
// #<id>" sebagai cadangan (bukan 500): detail deal tetap terbaca.
func (h *Handler) accountLabel(ctx context.Context, id int64) string {
	a, err := h.q(ctx).GetAccount(ctx, id)
	if err != nil {
		return "Desa #" + strconv.FormatInt(id, 10)
	}
	if a.EntityCode != nil && *a.EntityCode != "" {
		return *a.EntityCode + " — " + a.VillageName
	}
	return a.VillageName
}

// contactLabel meresolusi nama kontak utama. Gagal → "Kontak #<id>" cadangan.
func (h *Handler) contactLabel(ctx context.Context, id int64) string {
	c, err := h.q(ctx).GetContact(ctx, id)
	if err != nil {
		return "Kontak #" + strconv.FormatInt(id, 10)
	}
	return contactFullName(c)
}
