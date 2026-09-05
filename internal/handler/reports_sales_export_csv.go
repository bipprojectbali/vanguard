package handler

import (
	"context"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// reports_sales_export_csv.go — perakitan baris CSV per-panel Sales Report,
// dipisah dari reports_sales_export.go (ukuran file). Perilaku identik; hanya
// organisasi file yang berubah.

// reportsSalesCSV memilih agregasi sesuai panel & merakit baris CSV. Panel tak
// dikenal → pipeline (default aman, kompatibel mundur). f = filter Periode+Tim
// (BL-49) di-oper identik jalur HTML → CSV tersaring sama persis.
func (h *Handler) reportsSalesCSV(ctx context.Context, panelKey string, f salesReportFilter) (string, []string, [][]string, error) {
	deal := db.DealsListFilterFor(session.BusinessDataScope(ctx))
	lead := db.LeadsListFilterFor(session.BusinessDataScope(ctx))
	act := db.ActivitiesListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)
	q := h.q(ctx)

	switch panelKey {
	case "forecast":
		buckets, err := q.ReportSalesForecast(ctx, db.ReportSalesForecastParams{
			ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
			PeriodStart: f.Start, PeriodEnd: f.End, OwnerFilter: f.OwnerID,
		})
		if err != nil {
			return "", nil, nil, err
		}
		rows := make([][]string, 0, len(buckets))
		for _, b := range buckets {
			rows = append(rows, []string{b.Period, maskARR(numericStr(b.WeightedValue), br)})
		}
		return "sales-forecast", []string{"Periode", "Forecast"}, rows, nil

	case "winloss":
		reasons, err := q.ReportWinLossReasons(ctx, db.ReportWinLossReasonsParams{
			ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
			PeriodStart: f.Start, PeriodEnd: f.End, OwnerFilter: f.OwnerID,
		})
		if err != nil {
			return "", nil, nil, err
		}
		var total int64
		for _, rr := range reasons {
			total += rr.DealCount
		}
		rows := make([][]string, 0, len(reasons))
		for _, rr := range reasons {
			rows = append(rows, []string{rr.Reason, strconv.FormatInt(rr.DealCount, 10), pctStr(rr.DealCount, total)})
		}
		return "sales-winloss", []string{"Alasan Kalah", "Jumlah Deal", "Porsi"}, rows, nil

	case "funnel":
		funnel, err := q.ReportLeadFunnel(ctx, db.ReportLeadFunnelParams{
			ScopeAll: lead.ScopeAll, IsOwn: lead.IsOwn, Uid: &uid,
			PeriodStart: f.Start, PeriodEnd: f.End, OwnerFilter: f.OwnerID,
		})
		if err != nil {
			return "", nil, nil, err
		}
		timing, err := q.ReportWonTiming(ctx, db.ReportWonTimingParams{
			ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
			PeriodStart: f.Start, PeriodEnd: f.End, OwnerFilter: f.OwnerID,
		})
		if err != nil {
			return "", nil, nil, err
		}
		rows := make([][]string, 0, len(buildFunnelSteps(funnel, timing.WonCount)))
		for _, s := range buildFunnelSteps(funnel, timing.WonCount) {
			rows = append(rows, []string{s.Label, strconv.FormatInt(s.Count, 10), s.Pct})
		}
		return "lead-conversion", []string{"Tahap", "Jumlah", "Porsi"}, rows, nil

	case "activity":
		acts, err := q.ReportSalesActivityByOwner(ctx, db.ReportSalesActivityByOwnerParams{
			ScopeAll: act.ScopeAll, IsOwn: act.IsOwn, Uid: &uid,
			PeriodStart: f.Start, PeriodEnd: f.End, OwnerFilter: f.OwnerID,
		})
		if err != nil {
			return "", nil, nil, err
		}
		won, err := q.ReportWonDealsByOwner(ctx, db.ReportWonDealsByOwnerParams{
			ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
			PeriodStart: f.Start, PeriodEnd: f.End, OwnerFilter: f.OwnerID,
		})
		if err != nil {
			return "", nil, nil, err
		}
		built := buildActivityRows(acts, won)
		rows := make([][]string, 0, len(built))
		for _, a := range built {
			rows = append(rows, []string{
				a.Owner,
				strconv.FormatInt(a.Call, 10),
				strconv.FormatInt(a.Email, 10),
				strconv.FormatInt(a.Meeting, 10),
				strconv.FormatInt(a.Total, 10),
				strconv.FormatInt(a.Won, 10),
				a.PerDeal,
			})
		}
		return "sales-activity",
			[]string{"Sales", "Panggilan", "Email", "Meeting", "Total", "Deal Menang", "Aktivitas/Deal"},
			rows, nil

	default: // "" atau "pipeline" — kompatibel mundur M8-1.
		stages, err := q.ReportPipelineByStage(ctx, db.ReportPipelineByStageParams{
			ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
			PeriodStart: f.Start, PeriodEnd: f.End, OwnerFilter: f.OwnerID,
		})
		if err != nil {
			return "", nil, nil, err
		}
		rows := make([][]string, 0, len(stages))
		for _, s := range stages {
			rows = append(rows, []string{
				s.Stage,
				strconv.FormatInt(s.DealCount, 10),
				maskARR(numericStr(s.StageValue), br),
				strconv.FormatInt(s.AvgProbability, 10) + "%",
				maskARR(numericStr(s.WeightedValue), br),
			})
		}
		return "sales-report", []string{"Stage", "Jumlah Deal", "Nilai", "Probability", "Weighted"}, rows, nil
	}
}
