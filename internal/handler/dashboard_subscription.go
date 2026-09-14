package handler

import (
	"context"
	"math"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// dashboard_subscription.go — BL-59b + BL-98 + BL-142: section "Langganan"
// Beranda (Modul 1). Komposisi per-kapabilitas; heading tampil hanya bila ≥1
// KPI/chart tampil.
//
// BL-98 (ramping): section kini MAKSIMUM 2 KPI — MRR (crm:subscriptions) & Churn
// Rate (crm:churn) — TANPA chart domain. ARR sudah ada di baris KPI GLOBAL;
// Langganan Aktif, Renewal Rate/Jatuh Tempo, chart MRR-movement & Revenue-by-Plan
// dipindah ke Subscription Report (ditautkan "Lihat Laporan →"). REUSE agregasi
// Report 8.4 (BL-47): ReportSubMRR (MRR) + ReportRetention (churn). F3 via
// SubscriptionsListFilterFor(dataScope). F4: MRR tersamar maskARR (nama role,
// sama dgn Report 8.4).
//
// BL-142 (membalik BL-98 KHUSUS Beranda): tambah 2 chart inline.
//   - Revenue per Paket (bar, Rp): F4 LEBIH KETAT dari KPI MRR di atas — chart
//     tak bisa memasking nilai per-bar (beda dgn string KPI tunggal), jadi
//     digate canSeeSubscriptionARR(ctx) (kapabilitas ARR, BL-58) SETELAH
//     canViewSubscriptions, persis pola pre-BL-98 (canViewPlans && canSeeARR)
//     tapi kapabilitas-based mengikuti keputusan ticket TERKINI — bukan
//     canViewPlans terpisah krn data revenue di sini milik section Langganan.
//   - Renewal per Bulan (bar, COUNT saja → aman dari F4): digate
//     canViewSubscriptions ATAU canViewChurn (union, SAMA dgn cakupan KPI MRR/
//     Churn di atas) — ticket membolehkan kedua kapabilitas melihatnya.
//     PeriodStart/End dibiarkan kosong (unbounded) — konsisten dgn KPI
//     ReportSubMRR/ReportRetention di section ini yang juga tak difilter
//     periode; filter periode hanya ada di Subscription Report (BL-52).
func (h *Handler) dashSubscriptionDomain(ctx context.Context, dataScope string, uid int64) (panel.DashDomain, bool, error) {
	d := panel.DashDomain{Title: "Langganan"}
	q := h.q(ctx)
	br := session.BusinessRole(ctx)
	filter := db.SubscriptionsListFilterFor(dataScope)
	canSubs, canChurn := canViewSubscriptions(ctx), canViewChurn(ctx)

	if canSubs {
		mrr, err := q.ReportSubMRR(ctx, db.ReportSubMRRParams{
			Today:    reportTodayDate(time.Now().In(appTZ)),
			ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		d.KPIs = append(d.KPIs, panel.DashKPI{
			Label: "MRR", Value: maskARR(formatRupiah(mrr.MrrActive), br), ValueClass: "text-primary",
		})

		// Chart: Revenue per Paket (bar, Rp) — HANYA bila berhak lihat ARR
		// (chart tak bisa memasking per-bar); skip diam-diam bila tidak (F4).
		if canSeeSubscriptionARR(ctx) {
			rev, err := q.ReportRevenueByPlan(ctx, db.ReportRevenueByPlanParams{
				ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
			})
			if err != nil {
				return panel.DashDomain{}, false, err
			}
			plans := make([]string, len(rev))
			mrrByPlan := make([]int64, len(rev))
			for i, r := range rev {
				plans[i] = r.PlanName
				mrrByPlan[i] = int64(math.Round(numericFloat(r.Mrr)))
			}
			d.Charts = append(d.Charts, panel.DashChart{
				Title: "Revenue per Paket", ChartID: "chart-sub-revenue",
				ChartJSON: h.marshalChart(barOption(plans, mrrByPlan)),
			})
		}
	}

	if canChurn {
		retention, err := q.ReportRetention(ctx, db.ReportRetentionParams{
			ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		d.KPIs = append(d.KPIs, panel.DashKPI{
			Label: "Churn Rate", Value: ratePct(retention.Churned, retention.Active+retention.Churned),
			ValueClass: "text-error",
		})
	}

	// Chart: Renewal per Bulan (bar, COUNT saja — aman F4) — union kapabilitas
	// MRR/Churn di atas (ticket membolehkan salah satu). Period kosong = semua
	// bulan (unbounded, sama dgn KPI di section ini).
	if canSubs || canChurn {
		months, err := q.ReportRenewalByMonth(ctx, db.ReportRenewalByMonthParams{
			ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		periods := make([]string, len(months))
		renewed := make([]int64, len(months))
		for i, m := range months {
			periods[i] = monthLabelFromDate(m.Period)
			renewed[i] = m.Renewed
		}
		d.Charts = append(d.Charts, panel.DashChart{
			Title: "Renewal per Bulan", ChartID: "chart-sub-renewal",
			ChartJSON: h.marshalChart(barOption(periods, renewed)),
		})
	}

	// BL-98: tautan Subscription Report — HANYA bila role ber-crm:reports.
	if canViewReports(ctx) {
		d.ReportPath = wsPathOf(ctx, "/reports/subscriptions")
	}
	return d, len(d.KPIs) > 0, nil
}
