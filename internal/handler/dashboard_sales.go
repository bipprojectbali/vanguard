package handler

import (
	"context"
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// dashboard_sales.go — BL-59a: section domain "Sales" pada Beranda (Modul 1
// redesain, tasks.md BL-59). Komposisi-per-izin: tiap butir digate kapabilitas
// modulnya (crm:deals / crm:sales_activity / crm:leads read) — role melihat UNION
// butir dari modul yang boleh diaksesnya; domain tampil hanya bila ≥1 butir
// tampil (heading di view). Selaras F2/F3 yang sudah ada → role KUSTOM otomatis
// dapat section sesuai modulnya tanpa kode baru.
//
// REUSE agregasi (JANGAN tulis ulang, tasks.md): DashboardPipelineByStage (chart
// pipeline, sama dgn dashboard lama) · DealPipelineStats (won/lost → Win Rate,
// query modul Deals) · ReportSalesActivityByOwner (dijumlah di Go → hitung
// aktivitas, query Sales Report BL-43). Dua agregasi kecil BARU (tak ada
// padanannya): DashboardDealsClosingThisMonth & DashboardLeadsBySource.
//
// F3: tiap query pakai *ListFilterFor(dataScope) modul ASAL (Deals/Activities/
// Leads) — sumber SATU, bukan duplikasi logic scope. F4: section ini murni
// COUNT/persen (tanpa Rp) → tak butuh masking; nilai Rp (pipeline value) sengaja
// dibiarkan ke Sales Report yang sudah memaskingnya (maskARR).

// dashSalesDomain merakit section Sales. bool kedua = "punya isi" → handler hanya
// menambahkan domain ke view bila true (heading tak muncul kosong).
func (h *Handler) dashSalesDomain(ctx context.Context, dataScope string, uid int64) (panel.DashDomain, bool, error) {
	d := panel.DashDomain{Title: "Sales"}
	q := h.q(ctx)

	if canViewDeals(ctx) {
		deal := db.DealsListFilterFor(dataScope)

		stages, err := q.DashboardPipelineByStage(ctx, db.DashboardPipelineByStageParams{
			ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		d.Panels = append(d.Panels, panel.DashPanel{
			Title: "Pipeline per-Stage", ChartID: "chart-pipeline",
			ChartJSON: h.marshalChart(pipelineChartOption(stages)),
		})

		stats, err := q.DealPipelineStats(ctx, db.DealPipelineStatsParams{
			ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		// Win Rate = won/(won+lost); pctStr → "0%" bila belum ada deal tertutup
		// (reuse helper reports_sales_panels.go, satu definisi persen).
		d.KPIs = append(d.KPIs, panel.DashKPI{
			Label: "Win Rate", Value: pctStr(stats.WonCount, stats.WonCount+stats.LostCount),
			ValueClass: "text-success",
		})

		ms, me := monthRangeDates(time.Now().In(appTZ))
		closing, err := q.DashboardDealsClosingThisMonth(ctx, db.DashboardDealsClosingThisMonthParams{
			MonthStart: ms, MonthEnd: me, ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		d.KPIs = append(d.KPIs, panel.DashKPI{
			Label: "Deal Tutup Bulan Ini", Value: strconv.FormatInt(closing, 10),
			ValueClass: "text-primary",
		})
	}

	if canViewSalesActivity(ctx) {
		act := db.ActivitiesListFilterFor(dataScope)
		rows, err := q.ReportSalesActivityByOwner(ctx, db.ReportSalesActivityByOwnerParams{
			ScopeAll: act.ScopeAll, IsOwn: act.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		var total int64
		for _, r := range rows {
			total += r.TotalCount
		}
		d.KPIs = append(d.KPIs, panel.DashKPI{
			Label: "Aktivitas Sales", Value: strconv.FormatInt(total, 10),
			ValueClass: "text-base-content",
		})
	}

	if canViewLeads(ctx) {
		lead := db.LeadsListFilterFor(dataScope)
		src, err := q.DashboardLeadsBySource(ctx, db.DashboardLeadsBySourceParams{
			ScopeAll: lead.ScopeAll, IsOwn: lead.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		d.Panels = append(d.Panels, panel.DashPanel{
			Title: "Lead per Sumber", ChartID: "chart-leads",
			ChartJSON: h.marshalChart(leadsBySourceChartOption(src)),
		})
	}

	return d, len(d.KPIs) > 0 || len(d.Panels) > 0, nil
}

// monthRangeDates mengembalikan tanggal awal & akhir bulan kalender dari now
// (UTC date, bukan timestamp) — untuk filter expected_close_date "bulan ini".
func monthRangeDates(now time.Time) (start, end pgtype.Date) {
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1)
	return pgtype.Date{Time: first, Valid: true}, pgtype.Date{Time: last, Valid: true}
}
