package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// reports_sales.go — Sales Report (Modul 8, wireframe 8.1, BL-43): 5 panel
// (Pipeline · Forecast · Win/Loss · Lead Conversion · Sales Activity) + KPI
// ringkas. Orkestrasi di sini; transformasi baris→sub-view di
// reports_sales_panels.go; serialisasi CSV per-panel di reports_sales_export.go.
// "Report bukan objek data" (skema.md §8): nol tabel baru, agregasi murni.
//
// TIGA sumbu keamanan:
//   - F2 gate canViewReports (reports_view.go, SATU objek crm:reports read).
//   - F3 ownership per-sumber via {Deals,Leads,Activities}ListFilterFor
//     (session.BusinessDataScope) — sumber SATU dgn modul asal, tak duplikasi
//     logic scope. Support data_scope='none' atas deals → panel deal kosong
//     (F2 tetap 200).
//   - F4 maskARR pada SEMUA nilai Rp (Pipeline Value/Weighted, Forecast, KPI
//     Nilai Pipeline). Support pemegang crm:reports tapi DI LUAR allow-list
//     canSeeARR (skema.md §9) → nilai tersamar. Panel Win/Loss, Lead
//     Conversion, Sales Activity murni angka/persen/hari → tanpa masking.
//
// Filter interaktif Periode + Tim(owner) SENGAJA ditunda (BL follow-up) agar PR
// terreview; semua panel default seluruh data dalam cakupan ownership pemakai.
// Empat gap butuh-schema (Target/Batal/picklist Alasan/qualified_at) → BL-44.

// reportsSalesData menjalankan agregasi & merakit view-model 5 panel; dipakai
// ReportsSales (HTML) & ReportsSalesExport (CSV) agar keduanya konsisten (satu
// sumber angka, bukan dua jalur hitung terpisah).
func (h *Handler) reportsSalesData(ctx context.Context) (panel.ReportsSalesView, error) {
	deal := db.DealsListFilterFor(session.BusinessDataScope(ctx))
	lead := db.LeadsListFilterFor(session.BusinessDataScope(ctx))
	act := db.ActivitiesListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)
	q := h.q(ctx)

	stats, err := q.DealPipelineStats(ctx, db.DealPipelineStatsParams{
		ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}
	stages, err := q.ReportPipelineByStage(ctx, db.ReportPipelineByStageParams{
		ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}
	forecast, err := q.ReportSalesForecast(ctx, db.ReportSalesForecastParams{
		ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}
	reasons, err := q.ReportWinLossReasons(ctx, db.ReportWinLossReasonsParams{
		ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}
	funnel, err := q.ReportLeadFunnel(ctx, db.ReportLeadFunnelParams{
		ScopeAll: lead.ScopeAll, IsOwn: lead.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}
	wonTiming, err := q.ReportWonTiming(ctx, db.ReportWonTimingParams{
		ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}
	activity, err := q.ReportSalesActivityByOwner(ctx, db.ReportSalesActivityByOwnerParams{
		ScopeAll: act.ScopeAll, IsOwn: act.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}
	wonByOwner, err := q.ReportWonDealsByOwner(ctx, db.ReportWonDealsByOwnerParams{
		ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}

	total := stats.WonCount + stats.LostCount
	winRate := "—"
	if total > 0 {
		winRate = strconv.FormatFloat(float64(stats.WonCount)*100/float64(total), 'f', 1, 64) + "%"
	}

	return panel.ReportsSalesView{
		OpenCount:     stats.OpenCount,
		PipelineValue: maskARR(formatRupiah(stats.PipelineValue), br),
		WinRate:       winRate,

		Pipeline: buildPipelinePanel(stages, br),
		Forecast: buildForecastPanel(forecast, br),

		WinPct:      pctStr(stats.WonCount, total),
		LossPct:     pctStr(stats.LostCount, total),
		LossReasons: buildWinLossReasons(reasons),

		Funnel:        buildFunnelSteps(funnel, wonTiming.WonCount),
		Conversion:    pctStr(wonTiming.WonCount, funnel.TotalLeads),
		AvgLeadToDeal: daysStr(funnel.AvgDaysToDeal, funnel.DealCount),
		AvgDealToWon:  daysStr(wonTiming.AvgDaysToWon, wonTiming.WonCount),

		Activity: buildActivityRows(activity, wonByOwner),
	}, nil
}

// ReportsSales — GET /reports/sales. Bukan pemegang izin crm:reports read →
// 403 + penjelasan (pola sama dgn plans_page.go/subscriptions_page.go).
func (h *Handler) ReportsSales(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewReports(ctx) {
		h.renderReportsForbidden(w, r, "Sales Report", "/reports/sales")
		return
	}
	view, err := h.reportsSalesData(ctx)
	if err != nil {
		h.Log.Error("reports: sales data", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	view.Base = wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Sales Report", "/reports/sales", panel.ReportsSalesBody(view))
}

// renderReportsForbidden — 403 + penjelasan; dipakai Sales & Subscription
// Report (slice 3). Path dioper karena Reports punya dua halaman berbeda.
func (h *Handler) renderReportsForbidden(w http.ResponseWriter, r *http.Request, title, path string) {
	w.WriteHeader(http.StatusForbidden)
	h.renderWorkspaceShell(w, r, title, path, panel.SalesForbidden(title))
}
