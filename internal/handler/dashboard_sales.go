package handler

import (
	"context"
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// dashboard_sales.go — BL-59a + BL-98: section domain "Sales" pada Beranda
// (Modul 1). Komposisi-per-izin: tiap butir digate kapabilitas modulnya; role
// melihat UNION butir dari modul yang boleh diaksesnya; domain tampil hanya bila
// ≥1 KPI tampil (heading di view).
//
// BL-98 (ramping): section Sales kini MAKSIMUM 2 KPI paling penting — Win Rate &
// Deal Tutup Bulan Ini (keduanya di bawah crm:deals) — TANPA chart domain. Chart
// pipeline & lead per-sumber, serta KPI "Aktivitas Sales", dipindah ke Sales
// Report (ditautkan via "Lihat Laporan →"). REUSE agregasi: DealPipelineStats
// (won/lost → Win Rate) + DashboardDealsClosingThisMonth. F3 via
// DealsListFilterFor(dataScope). F4: murni COUNT/persen (tanpa Rp) → tak butuh
// masking.

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
