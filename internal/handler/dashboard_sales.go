package handler

import (
	"context"
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// dashboard_sales.go — BL-59a + BL-98 + BL-141: section domain "Sales" pada
// Beranda (Modul 1). Komposisi-per-izin: tiap butir digate kapabilitas
// modulnya; role melihat UNION butir dari modul yang boleh diaksesnya; domain
// tampil hanya bila ≥1 KPI tampil (heading di view).
//
// BL-98 (ramping): section Sales kini MAKSIMUM 2 KPI paling penting — Win Rate &
// Deal Tutup Bulan Ini (keduanya di bawah crm:deals) — TANPA chart domain. Chart
// pipeline & lead per-sumber, serta KPI "Aktivitas Sales", dipindah ke Sales
// Report (ditautkan via "Lihat Laporan →"). REUSE agregasi: DealPipelineStats
// (won/lost → Win Rate) + DashboardDealsClosingThisMonth. F3 via
// DealsListFilterFor(dataScope). F4: murni COUNT/persen (tanpa Rp) → tak butuh
// masking.
//
// BL-141 (membalik BL-98 KHUSUS Beranda): tambah 3 chart inline, SEMUA di
// bawah gate crm:deals yang sama dgn KPI di atas (ticket BL-141 — bukan
// crm:leads terpisah utk chart Leads per Sumber, berbeda dari pola pre-BL-98
// yg pernah menggate chart itu via canViewLeads; di sini disatukan supaya
// section Sales konsisten satu kapabilitas). Win/Loss REUSE stats yg sudah
// diambil di atas (tanpa query baru); Pipeline & Leads pakai query dashboard.sql
// yang sudah ada sejak BL-59a.

// dashSalesDomain merakit section Sales. bool kedua = "punya isi" (≥1 KPI) →
// handler hanya menambahkan domain ke view bila true (heading tak muncul kosong).
func (h *Handler) dashSalesDomain(ctx context.Context, dataScope string, uid int64) (panel.DashDomain, bool, error) {
	d := panel.DashDomain{Title: "Sales"}
	q := h.q(ctx)

	if canViewDeals(ctx) {
		deal := db.DealsListFilterFor(dataScope)

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

		// Chart 1: Pipeline per Stage (bar) — deal terbuka per-stage, urut alami
		// kanban (query sudah CASE-order).
		pipeline, err := q.DashboardPipelineByStage(ctx, db.DashboardPipelineByStageParams{
			ScopeAll: deal.ScopeAll, IsOwn: deal.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		stages := make([]string, len(pipeline))
		stageCounts := make([]int64, len(pipeline))
		for i, p := range pipeline {
			stages[i] = p.Stage
			stageCounts[i] = p.DealCount
		}
		d.Charts = append(d.Charts, panel.DashChart{
			Title: "Pipeline per Stage", ChartID: "chart-sales-pipeline",
			ChartJSON: h.marshalChart(barOption(stages, stageCounts)),
		})

		// Chart 2: Leads per Sumber (pie) — filter F3 TERPISAH dari deal (sumber
		// data leads, bukan deals; LeadsListFilterFor sama pola DealsListFilterFor).
		lead := db.LeadsListFilterFor(dataScope)
		bySource, err := q.DashboardLeadsBySource(ctx, db.DashboardLeadsBySourceParams{
			ScopeAll: lead.ScopeAll, IsOwn: lead.IsOwn, Uid: &uid,
		})
		if err != nil {
			return panel.DashDomain{}, false, err
		}
		sources := make([]string, len(bySource))
		sourceCounts := make([]int64, len(bySource))
		for i, s := range bySource {
			sources[i] = s.Source
			sourceCounts[i] = s.LeadCount
		}
		d.Charts = append(d.Charts, panel.DashChart{
			Title: "Leads per Sumber", ChartID: "chart-sales-leads",
			ChartJSON: h.marshalChart(pieOption("Leads", sources, sourceCounts)),
		})

		// Chart 3: Win/Loss (pie) — REUSE stats yg sudah diambil di atas (tanpa
		// query baru). Skip total bila belum ada deal tertutup sama sekali (won+
		// lost==0) — pie kosong tak berguna, drpd render donut 0/0 yg
		// membingungkan.
		if stats.WonCount+stats.LostCount > 0 {
			d.Charts = append(d.Charts, panel.DashChart{
				Title: "Win/Loss", ChartID: "chart-sales-winloss",
				ChartJSON: h.marshalChart(pieOption("Deal Tertutup",
					[]string{"Won", "Lost"}, []int64{stats.WonCount, stats.LostCount})),
			})
		}
	}

	// BL-98: tautan Sales Report — HANYA bila role ber-crm:reports (jangan pernah
	// menautkan halaman yang akan 403; role ber-crm:deals tanpa crm:reports tetap
	// lihat KPI, tanpa tautan).
	if canViewReports(ctx) {
		d.ReportPath = wsPathOf(ctx, "/reports/sales")
	}
	return d, len(d.KPIs) > 0, nil
}

// monthRangeDates mengembalikan tanggal awal & akhir bulan kalender dari now
// (UTC date, bukan timestamp) — untuk filter expected_close_date "bulan ini".
func monthRangeDates(now time.Time) (start, end pgtype.Date) {
	first := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	last := first.AddDate(0, 1, -1)
	return pgtype.Date{Time: first, Valid: true}, pgtype.Date{Time: last, Valid: true}
}
