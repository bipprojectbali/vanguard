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
//
// Pemetaan baris (renewalChainView, subItemRows) & resolusi label plan
// (planLabel) di subscriptions_detail_rows.go.

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
	view := h.subDetailView(ctx, base, s, names)
	view.Msg = subscriptionsMsg(r.URL.Query().Get("ok"))
	view.Err = wsErrMsg(r.URL.Query().Get("err"))
	h.renderWorkspaceShell(w, r, title, "/subscriptions", panel.SubDetail(view))
}

// subDetailView merakit detail lengkap + F4 (ARR disamarkan) + riwayat rantai
// renewal. Nama plan & desa diresolusi best-effort (di luar tenant/terhapus →
// label cadangan, tak menggagalkan halaman).
func (h *Handler) subDetailView(ctx context.Context, base string, s db.Subscription, names map[int64]string) panel.SubDetailView {
	br := session.BusinessRole(ctx)
	canARR := canSeeSubscriptionARR(ctx) // BL-58: kapabilitas ter-matriks, bukan nama role

	// Item paket (BL-88 PR2b): best-effort (gagal → nil, detail tetap terbaca).
	// >1 item → label "N paket" (parent plan_id NULL); 1 item → nama paket item
	// itu; 0 item (langganan lama tanpa item) → planLabel(parent) cadangan.
	items, err := h.q(ctx).ListSubscriptionItemsWithPlan(ctx, s.ID)
	if err != nil {
		h.Log.Error("subscriptions: items", "err", err)
	}
	planDisplay := h.planLabel(ctx, s.PlanID)
	switch {
	case len(items) > 1:
		planDisplay = strconv.Itoa(len(items)) + " paket"
	case len(items) == 1 && items[0].PlanName != nil && *items[0].PlanName != "":
		planDisplay = *items[0].PlanName
	}

	return panel.SubDetailView{
		Base:         base,
		ID:           s.ID,
		EntityCode:   deref(s.EntityCode),
		Village:      h.accountLabel(ctx, s.AccountID),
		AccountID:    s.AccountID,
		Plan:         planDisplay,
		Items:        subItemRows(items, br, canARR),
		Status:       s.Status,
		MRR:          maskARR(formatRupiah(s.Mrr), br),
		ARR:          maskSubscriptionARR(formatRupiah(s.Arr), canARR),
		BillingCycle: deref(s.BillingCycle),
		AutoRenew:    s.AutoRenew,
		Start:        dateStr(s.StartDate),
		End:          dateStr(s.EndDate),
		Seats:        int32Str(s.QuantitySeats),
		PaymentState: deref(s.PaymentStatus),
		Owner:        ownerName(s.SubscriptionOwner, names),
		Chain:        h.renewalChainView(ctx, s.ID, br, canARR),

		// Flag aksi (M5-3c) diprecompute di sini — view murni-data (tak panggil authz).
		CanRenew:     canRenewSubscriptions(ctx),
		CanChurn:     canChurnSubscriptions(ctx),
		CanApprove:   canApproveRenewal(ctx),
		CanActivate:  canActivateSubscriptions(ctx),
		ChurnReasons: churnReasonOptions,
		ChurnTypes:   churnTypeOptions,
	}
}
