package handler

import (
	"context"
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// dashboard_subscription.go — builder section "Langganan" Beranda (BL-59b).
// Komposisi per-kapabilitas: tiap butir digate crm:subscriptions/renewals/
// plans/churn read; heading tampil hanya bila ≥1 butir tampil (bool balik).
// REUSE agregasi Subscription Report 8.4 (BL-47) — TAK menulis ulang query.
// F3 kepemilikan via SubscriptionsListFilterFor(dataScope) (subscription_owner,
// PERSIS ListSubscriptions). F4: MRR pakai maskARR (nama role), ARR pakai
// kapabilitas canSeeSubscriptionARR (BL-58); chart nilai Rp (Revenue-by-Plan,
// MRR baru vs churn) HANYA dirakit bila canSeeARR — chart tak bisa memasking
// nilai per-bar, jadi disembunyikan penuh sesuai posture FLS.
func (h *Handler) dashSubscriptionDomain(ctx context.Context, dataScope string, uid int64) (panel.DashDomain, bool, error) {
	d := panel.DashDomain{Title: "Langganan"}
	q := h.q(ctx)
	br := session.BusinessRole(ctx)
	canARR := canSeeSubscriptionARR(ctx)
	filter := db.SubscriptionsListFilterFor(dataScope)
	today := reportTodayDate(time.Now().In(appTZ))

	// ReportSubMRR memberi MRR/ARR/jumlah aktif (crm:subscriptions) SEKALIGUS
	// komponen pergerakan MRR baru & churn (crm:churn) — dihitung sekali bila
	// salah satu section membutuhkannya.
	var mrr db.ReportSubMRRRow
	if canViewSubscriptions(ctx) || canViewChurn(ctx) {
		m, err := q.ReportSubMRR(ctx, db.ReportSubMRRParams{
			Today: today, ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		mrr = m
	}

	if canViewSubscriptions(ctx) {
		d.KPIs = append(d.KPIs,
			panel.DashKPI{Label: "MRR", Value: maskARR(formatRupiah(mrr.MrrActive), br), ValueClass: "text-primary"},
			panel.DashKPI{Label: "ARR", Value: maskSubscriptionARR(formatRupiah(mrr.ArrActive), canARR), ValueClass: "text-primary"},
			panel.DashKPI{Label: "Langganan Aktif", Value: strconv.FormatInt(mrr.ActiveCount, 10), ValueClass: "text-base-content"},
		)
	}

	if canViewRenewals(ctx) {
		renewal, err := q.ReportRenewalSummary(ctx, db.ReportRenewalSummaryParams{
			Today: today, ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		d.KPIs = append(d.KPIs,
			panel.DashKPI{Label: "Renewal Rate", Value: ratePct(renewal.RenewedPast, renewal.DuePast), ValueClass: "text-success"},
			panel.DashKPI{Label: "Jatuh Tempo 30 Hari", Value: strconv.FormatInt(renewal.Due30, 10), ValueClass: "text-warning"},
		)
	}

	if canViewChurn(ctx) {
		retention, err := q.ReportRetention(ctx, db.ReportRetentionParams{
			ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		d.KPIs = append(d.KPIs,
			panel.DashKPI{Label: "Churn Rate", Value: ratePct(retention.Churned, retention.Active+retention.Churned), ValueClass: "text-error"},
		)
		// Panel bernilai Rp → hanya bila berhak lihat nilai (chart tak memasking).
		if canSeeARR(br) {
			d.Panels = append(d.Panels, panel.DashPanel{
				Title: "MRR Baru vs Churn", ChartID: "chart-mrr-movement",
				ChartJSON: h.marshalChart(mrrMovementChartOption(mrr)),
			})
		}
	}

	if canViewPlans(ctx) && canSeeARR(br) {
		rev, err := q.ReportRevenueByPlan(ctx, db.ReportRevenueByPlanParams{
			ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		d.Panels = append(d.Panels, panel.DashPanel{
			Title: "Revenue per Paket", ChartID: "chart-revenue-plan",
			ChartJSON: h.marshalChart(revenueByPlanChartOption(rev)),
		})
	}

	return d, len(d.KPIs) > 0 || len(d.Panels) > 0, nil
}
