package handler

import (
	"context"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// reports_sales_data.go — orkestrasi agregasi & view-model Sales Report,
// dipisah dari reports_sales.go (ukuran file). Perilaku identik; hanya
// organisasi file yang berubah.

// reportsSalesData menjalankan agregasi & merakit view-model 5 panel; dipakai
// ReportsSales (HTML) & ReportsSalesExport (CSV) agar keduanya konsisten (satu
// sumber angka, bukan dua jalur hitung terpisah). f = filter Periode+Tim (BL-49)
// yang di-AND ke tiap query (guard NULL = tak menyaring); owner_filter menyempit
// DI ATAS scope F3, tak melebarkan.
func (h *Handler) reportsSalesData(ctx context.Context, f salesReportFilter) (panel.ReportsSalesView, error) {
	deal := db.DealsListFilterFor(session.BusinessDataScope(ctx))
	lead := db.LeadsListFilterFor(session.BusinessDataScope(ctx))
	act := db.ActivitiesListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)
	q := h.q(ctx)

	stats, err := q.DealPipelineStats(ctx, db.DealPipelineStatsParams{
		ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
		PeriodStart: f.Start, PeriodEnd: f.End, OwnerFilter: f.OwnerID,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}
	stages, err := q.ReportPipelineByStage(ctx, db.ReportPipelineByStageParams{
		ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
		PeriodStart: f.Start, PeriodEnd: f.End, OwnerFilter: f.OwnerID,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}
	forecast, err := q.ReportSalesForecast(ctx, db.ReportSalesForecastParams{
		ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
		PeriodStart: f.Start, PeriodEnd: f.End, OwnerFilter: f.OwnerID,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}
	reasons, err := q.ReportWinLossReasons(ctx, db.ReportWinLossReasonsParams{
		ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
		PeriodStart: f.Start, PeriodEnd: f.End, OwnerFilter: f.OwnerID,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}
	funnel, err := q.ReportLeadFunnel(ctx, db.ReportLeadFunnelParams{
		ScopeAll: lead.ScopeAll, IsOwn: lead.IsOwn, Uid: &uid,
		PeriodStart: f.Start, PeriodEnd: f.End, OwnerFilter: f.OwnerID,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}
	wonTiming, err := q.ReportWonTiming(ctx, db.ReportWonTimingParams{
		ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
		PeriodStart: f.Start, PeriodEnd: f.End, OwnerFilter: f.OwnerID,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}
	activity, err := q.ReportSalesActivityByOwner(ctx, db.ReportSalesActivityByOwnerParams{
		ScopeAll: act.ScopeAll, IsOwn: act.IsOwn, Uid: &uid,
		PeriodStart: f.Start, PeriodEnd: f.End, OwnerFilter: f.OwnerID,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}
	wonByOwner, err := q.ReportWonDealsByOwner(ctx, db.ReportWonDealsByOwnerParams{
		ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
		PeriodStart: f.Start, PeriodEnd: f.End, OwnerFilter: f.OwnerID,
	})
	if err != nil {
		return panel.ReportsSalesView{}, err
	}

	filterView, err := h.buildSalesFilterView(ctx, f, deal, uid)
	if err != nil {
		return panel.ReportsSalesView{}, err
	}

	total := stats.WonCount + stats.LostCount
	winRate := "—"
	if total > 0 {
		winRate = strconv.FormatFloat(float64(stats.WonCount)*100/float64(total), 'f', 1, 64) + "%"
	}

	return panel.ReportsSalesView{
		Filter: filterView,

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
