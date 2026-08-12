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

// subscriptions_detail.go — HALAMAN baca satu langganan + riwayat rantai renewal
// (M5-3b, GET-only). Ownership (F3) diputuskan di sini atas baris (filter.Allows)
// — kembaran per-baris dari daftar; di luar cakupan → 404 (bukan 403: keberadaan
// baris pun tak diungkap). Meniru DealDetail. Renew/churn menyusul slice lain.

// SubscriptionDetail — GET /w/{workspace}/subscriptions/{id}. Satu langganan.
// Nama plan & desa diresolusi best-effort (satu query masing-masing, bukan N+1);
// gagal → label cadangan, bukan 500.
func (h *Handler) SubscriptionDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewSubscriptions(ctx) {
		h.renderSubscriptionsForbidden(w, r)
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	s, err := h.q(ctx).GetSubscription(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		h.Log.Error("subscriptions: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	filter := db.SubscriptionsListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), s.SubscriptionOwner) {
		http.NotFound(w, r)
		return
	}

	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("subscriptions: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	base := wsPath(slugFromRequest(r), "")
	title := deref(s.EntityCode)
	if title == "" {
		title = "Langganan #" + strconv.FormatInt(s.ID, 10)
	}
	h.renderWorkspaceShell(w, r, title, "/subscriptions",
		panel.SubDetail(h.subDetailView(ctx, base, s, names)))
}

// subDetailView merakit detail lengkap + F4 (ARR disamarkan) + riwayat rantai
// renewal. Nama plan & desa diresolusi best-effort (di luar tenant/terhapus →
// label cadangan, tak menggagalkan halaman).
func (h *Handler) subDetailView(ctx context.Context, base string, s db.Subscription, names map[int64]string) panel.SubDetailView {
	br := session.BusinessRole(ctx)
	return panel.SubDetailView{
		Base:         base,
		ID:           s.ID,
		EntityCode:   deref(s.EntityCode),
		Village:      h.accountLabel(ctx, s.AccountID),
		AccountID:    s.AccountID,
		Plan:         h.planLabel(ctx, s.PlanID),
		Status:       s.Status,
		MRR:          formatRupiah(s.Mrr),
		ARR:          maskSubscriptionARR(formatRupiah(s.Arr), br),
		BillingCycle: deref(s.BillingCycle),
		AutoRenew:    s.AutoRenew,
		Start:        dateStr(s.StartDate),
		End:          dateStr(s.EndDate),
		Seats:        int32Str(s.QuantitySeats),
		PaymentState: deref(s.PaymentStatus),
		Owner:        ownerName(s.SubscriptionOwner, names),
		Chain:        h.renewalChainView(ctx, s.ID, br),
	}
}

// renewalChainView memuat riwayat rantai renewal (lama→baru) untuk kartu di
// detail. Best-effort (mirror dealQuotesPreview): gagal query → nil + log, detail
// tetap terbaca. ARR tiap periode disamarkan mengikuti kebijakan yang sama.
func (h *Handler) renewalChainView(ctx context.Context, id int64, businessRole string) []panel.SubChainRow {
	rows, err := h.q(ctx).ListRenewalChain(ctx, id)
	if err != nil {
		h.Log.Error("subscriptions: renewal chain", "err", err)
		return nil
	}
	out := make([]panel.SubChainRow, 0, len(rows))
	for _, c := range rows {
		out = append(out, panel.SubChainRow{
			ID:     c.ID,
			IsThis: c.ID == id,
			Status: c.Status,
			MRR:    formatRupiah(c.Mrr),
			ARR:    maskSubscriptionARR(formatRupiah(c.Arr), businessRole),
			Start:  dateStr(c.StartDate),
			End:    dateStr(c.EndDate),
		})
	}
	return out
}

// planLabel meresolusi nama plan untuk detail (nama saja). Gagal → "Plan #<id>"
// cadangan (bukan 500): detail langganan tetap terbaca.
func (h *Handler) planLabel(ctx context.Context, id int64) string {
	p, err := h.q(ctx).GetPlan(ctx, id)
	if err != nil {
		return "Plan #" + strconv.FormatInt(id, 10)
	}
	return p.PlanName
}
