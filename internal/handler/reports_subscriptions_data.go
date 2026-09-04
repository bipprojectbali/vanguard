package handler

import (
	"context"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// reports_subscriptions_data.go — orkestrasi agregasi & view-model Subscription
// Report, dipisah dari reports_subscriptions.go (ukuran file). Perilaku identik;
// hanya organisasi file yang berubah.

// reportsSubscriptionsData menjalankan agregasi & merakit view-model 5 panel;
// dipakai ReportsSubscriptions (HTML) & ReportsSubscriptionsExport (CSV) agar
// keduanya konsisten (satu sumber angka, bukan dua jalur hitung terpisah).
func (h *Handler) reportsSubscriptionsData(ctx context.Context, f subscriptionReportFilter) (panel.ReportsSubscriptionsView, error) {
	filter := db.SubscriptionsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)
	today := reportTodayDate(time.Now().In(appTZ))
	q := h.q(ctx)

	// BL-52: pergerakan MRR & renewal & churn dipotong Periode (kolom benar per
	// query); MRR/ARR berjalan, Revenue-by-Plan, & Aging = SNAPSHOT (Periode TAK
	// dioper). Paket (plan_filter) menyaring SEMUA query.
	mrr, err := q.ReportSubMRR(ctx, db.ReportSubMRRParams{
		Today: today, ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		PeriodStart: f.Start, PeriodEnd: f.End, PlanFilter: f.PlanID,
	})
	if err != nil {
		return panel.ReportsSubscriptionsView{}, err
	}
	renewal, err := q.ReportRenewalSummary(ctx, db.ReportRenewalSummaryParams{
		Today: today, ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		PeriodStart: f.Start, PeriodEnd: f.End, PlanFilter: f.PlanID,
	})
	if err != nil {
		return panel.ReportsSubscriptionsView{}, err
	}
	renewalMonths, err := q.ReportRenewalByMonth(ctx, db.ReportRenewalByMonthParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		PeriodStart: f.Start, PeriodEnd: f.End, PlanFilter: f.PlanID,
	})
	if err != nil {
		return panel.ReportsSubscriptionsView{}, err
	}
	retention, err := q.ReportRetention(ctx, db.ReportRetentionParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		PeriodStart: f.Start, PeriodEnd: f.End, PlanFilter: f.PlanID,
	})
	if err != nil {
		return panel.ReportsSubscriptionsView{}, err
	}
	churnAge, err := q.ReportChurnAge(ctx, db.ReportChurnAgeParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		PeriodStart: f.Start, PeriodEnd: f.End, PlanFilter: f.PlanID,
	})
	if err != nil {
		return panel.ReportsSubscriptionsView{}, err
	}
	churnReasons, err := q.ReportChurnReasons(ctx, db.ReportChurnReasonsParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		PeriodStart: f.Start, PeriodEnd: f.End, PlanFilter: f.PlanID,
	})
	if err != nil {
		return panel.ReportsSubscriptionsView{}, err
	}
	revenueByPlan, err := q.ReportRevenueByPlan(ctx, db.ReportRevenueByPlanParams{
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		PlanFilter: f.PlanID,
	})
	if err != nil {
		return panel.ReportsSubscriptionsView{}, err
	}
	aging, err := q.ReportSubscriptionAging(ctx, db.ReportSubscriptionAgingParams{
		Today: today, ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		PlanFilter: f.PlanID,
	})
	if err != nil {
		return panel.ReportsSubscriptionsView{}, err
	}

	renewalRate := ratePct(renewal.RenewedPast, renewal.DuePast)
	v := panel.ReportsSubscriptionsView{
		MRR:         maskARR(formatRupiah(mrr.MrrActive), br),
		ARR:         maskARR(formatRupiah(mrr.ArrActive), br),
		RenewalRate: renewalRate,
		ChurnRate:   ratePct(retention.Churned, retention.Active+retention.Churned),

		MRRComponents: buildMRRComponents(mrr, br),

		RenewalRateCard: renewalRate,
		RenewedValue:    maskARR(formatRupiah(renewal.RenewedValue), br),
		Due30:           renewal.Due30,
		RenewalMonths:   buildRenewalMonths(renewalMonths),

		ChurnVillages: retention.Churned,
		LostValue:     maskARR(formatRupiah(churnAge.LostValue), br),
		AvgAge:        ageDaysStr(churnAge.AvgAgeDays, churnAge.AgedCount),
		ChurnReasons:  buildSubChurnReasons(churnReasons, br),

		RevenueByPlan: buildRevenueByPlan(revenueByPlan, br),

		AgingRows: buildSubAging(aging, br),
	}
	return v, nil
}
