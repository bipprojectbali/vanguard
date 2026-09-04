package handler

import (
	"context"
	"net/http"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// reports_subscriptions.go — Subscription Report (Modul 8, wireframe 8.4,
// BL-47): 5 panel agregasi (MRR/ARR · Renewal · Churn · Revenue by Plan ·
// Aging) + 4 KPI. Orkestrasi di sini; transformasi baris→sub-view di
// reports_subscriptions_panels.go; serialisasi CSV per-panel di
// reports_subscriptions_export.go. "Report bukan objek data" (skema.md §8):
// NOL tabel baru, agregasi murni atas subscriptions (+ plans, customer_success).
// Drill-down per-desa renewal/churn TETAP di modul Subscriptions
// (/subscriptions/renewals · /churn) — laporan ini agregasi, bukan daftar.
//
// DILEWATKAN (keputusan sadar BL-47, "data belum ada DILEWATKAN dulu"): delta
// "vs bulan lalu" kartu MRR & tren MRR bulanan historis (tak ada snapshot MRR
// lampau). Filter interaktif Periode+Paket ditunda ke BL lanjutan (pola
// BL-49/50/51).
//
// F2 gate canViewReports (SATU objek crm:reports read). F3 ownership via
// SubscriptionsListFilterFor (subscription_owner — sumber SAMA modul asal). F4
// masking Rp via maskARR di builder (Support/role tanpa akses → "•••").

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

// reportTodayDate = "hari ini" appTZ sebagai pgtype.Date (tengah malam UTC —
// jendela query date_trunc/end_date berbasis date murni). Dioper ke query
// ber-Today (hindari AT TIME ZONE di SELECT list sqlc, gotcha #14).
func reportTodayDate(now time.Time) pgtype.Date {
	return pgtype.Date{
		Time:  time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC),
		Valid: true,
	}
}

// ReportsSubscriptions — GET /reports/subscriptions. Bukan pemegang izin
// crm:reports read → 403 + penjelasan (pola sama reports_support.go).
func (h *Handler) ReportsSubscriptions(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewReports(ctx) {
		h.renderReportsForbidden(w, r, "Subscription Report", "/reports/subscriptions")
		return
	}
	f := parseSubscriptionReportFilter(r)
	view, err := h.reportsSubscriptionsData(ctx, f)
	if err != nil {
		h.Log.Error("reports: subscriptions data", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	sf := db.SubscriptionsListFilterFor(session.BusinessDataScope(ctx))
	filterView, err := h.buildSubscriptionFilterView(ctx, f, sf, session.UserID(ctx))
	if err != nil {
		h.Log.Error("reports: subscriptions filter", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	view.Filter = filterView
	view.Base = wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Subscription Report", "/reports/subscriptions",
		panel.ReportsSubscriptionsBody(view))
}
