package handler

import (
	"context"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// dashboard_subscription.go — BL-59b + BL-98: section "Langganan" Beranda
// (Modul 1). Komposisi per-kapabilitas; heading tampil hanya bila ≥1 KPI tampil.
//
// BL-98 (ramping): section kini MAKSIMUM 2 KPI — MRR (crm:subscriptions) & Churn
// Rate (crm:churn) — TANPA chart domain. ARR sudah ada di baris KPI GLOBAL;
// Langganan Aktif, Renewal Rate/Jatuh Tempo, chart MRR-movement & Revenue-by-Plan
// dipindah ke Subscription Report (ditautkan "Lihat Laporan →"). REUSE agregasi
// Report 8.4 (BL-47): ReportSubMRR (MRR) + ReportRetention (churn). F3 via
// SubscriptionsListFilterFor(dataScope). F4: MRR tersamar maskARR (nama role,
// sama dgn Report 8.4).
func (h *Handler) dashSubscriptionDomain(ctx context.Context, dataScope string, uid int64) (panel.DashDomain, bool, error) {
	d := panel.DashDomain{Title: "Langganan"}
	q := h.q(ctx)
	br := session.BusinessRole(ctx)
	filter := db.SubscriptionsListFilterFor(dataScope)

	if canViewSubscriptions(ctx) {
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
	}

	if canViewChurn(ctx) {
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

	// BL-98: tautan Subscription Report — HANYA bila role ber-crm:reports.
	if canViewReports(ctx) {
		d.ReportPath = wsPathOf(ctx, "/reports/subscriptions")
	}
	return d, len(d.KPIs) > 0, nil
}
