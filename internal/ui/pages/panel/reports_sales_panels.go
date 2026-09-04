package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// reports_sales_panels.go — fungsi render 5 panel Sales Report 8.1 (BL-43).
// Data & pembungkus (reportPanelCard/reportBar/reportEmpty) di reports_sales.go.
// Tiap <table> dibungkus ui.TableScroll (WAJIB — tabel telanjang meluber di
// viewport 375px). Semua nilai Rp SUDAH diformat & di-mask di handler; view tak
// memutuskan tampil/sembunyi sendiri.

// th/td helper ringkas agar tabel padat tak berulang kelas panjang.
func rTh(label string) g.Node {
	return h.Th(h.Class("py-2 pr-4 font-medium"), g.Text(label))
}

func rTd(class string, node g.Node) g.Node {
	return h.Td(h.Class("py-2 pr-4 "+class), node)
}

// ── Panel 1: Pipeline Report ────────────────────────────────────────────────

func reportsPipelinePanel(v ReportsSalesView) g.Node {
	var body g.Node
	if len(v.Pipeline) == 0 {
		body = reportEmpty("Belum ada data deal.")
	} else {
		rows := make([]g.Node, 0, len(v.Pipeline))
		for _, r := range v.Pipeline {
			rows = append(rows, h.Tr(
				h.Class("border-b border-base-300/50"),
				rTd("font-medium", g.Text(r.Stage)),
				rTd("", g.Text(strconv.FormatInt(r.Count, 10))),
				rTd("", g.Text(r.Value)),
				rTd("", g.Text(r.Probability)),
				rTd("", g.Text(r.Weighted)),
				h.Td(h.Class("py-2 w-24"), reportBar(r.BarPct)),
			))
		}
		body = ui.TableScroll(h.Table(
			h.Class("w-full text-sm"),
			h.THead(h.Tr(
				h.Class("border-b border-base-300 text-left text-base-content/70"),
				rTh("Stage"), rTh("Deal"), rTh("Nilai"), rTh("Probability"), rTh("Weighted"), rTh(""),
			)),
			h.TBody(g.Group(rows)),
		))
	}
	return reportPanelCard(v.Base, "Pipeline Report", "pipeline", body)
}

// ── Panel 2: Sales Forecast ─────────────────────────────────────────────────

func reportsForecastPanel(v ReportsSalesView) g.Node {
	var body g.Node
	if len(v.Forecast) == 0 {
		body = reportEmpty("Belum ada deal terbuka dengan tanggal perkiraan tutup.")
	} else {
		rows := make([]g.Node, 0, len(v.Forecast))
		for _, r := range v.Forecast {
			rows = append(rows, h.Tr(
				h.Class("border-b border-base-300/50"),
				rTd("font-medium", g.Text(r.Period)),
				rTd("", g.Text(r.Forecast)),
				h.Td(h.Class("py-2 w-24"), reportBar(r.BarPct)),
			))
		}
		body = ui.TableScroll(h.Table(
			h.Class("w-full text-sm"),
			h.THead(h.Tr(
				h.Class("border-b border-base-300 text-left text-base-content/70"),
				rTh("Periode"), rTh("Forecast"), rTh(""),
			)),
			h.TBody(g.Group(rows)),
		))
	}
	return reportPanelCard(v.Base, "Sales Forecast", "forecast", body)
}

// ── Panel 3: Win/Loss Analysis ──────────────────────────────────────────────

func reportsWinLossPanel(v ReportsSalesView) g.Node {
	cards := h.Div(h.Class("grid grid-cols-2 gap-3 mb-3"),
		reportStat("Menang", v.WinPct, "text-success"),
		reportStat("Kalah", v.LossPct, "text-error"),
	)
	var reasons g.Node
	if len(v.LossReasons) == 0 {
		reasons = reportEmpty("Belum ada deal kalah.")
	} else {
		rows := make([]g.Node, 0, len(v.LossReasons))
		for _, r := range v.LossReasons {
			rows = append(rows, h.Tr(
				h.Class("border-b border-base-300/50"),
				rTd("font-medium break-words", g.Text(r.Reason)),
				rTd("", g.Text(strconv.FormatInt(r.Count, 10))),
				rTd("", g.Text(r.Porsi)),
				h.Td(h.Class("py-2 w-24"), reportBar(r.BarPct)),
			))
		}
		reasons = ui.TableScroll(h.Table(
			h.Class("w-full text-sm"),
			h.THead(h.Tr(
				h.Class("border-b border-base-300 text-left text-base-content/70"),
				rTh("Alasan Kalah"), rTh("Deal"), rTh("Porsi"), rTh(""),
			)),
			h.TBody(g.Group(rows)),
		))
	}
	return reportPanelCard(v.Base, "Win/Loss Analysis", "winloss", cards, reasons)
}

// reportStat = kartu angka tunggal kecil (Menang/Kalah %). Meniru dashboardKPICard
// tapi lebih ringkas untuk dalam-panel.
func reportStat(label, value, valueClass string) g.Node {
	return h.Div(h.Class("rounded-box bg-base-200 p-3 min-w-0"),
		h.P(h.Class("text-xs text-base-content/60"), g.Text(label)),
		h.P(h.Class("text-xl font-bold truncate "+valueClass), g.Text(value)),
	)
}

// ── Panel 4: Lead Conversion ────────────────────────────────────────────────

func reportsFunnelPanel(v ReportsSalesView) g.Node {
	steps := make([]g.Node, 0, len(v.Funnel))
	for _, s := range v.Funnel {
		steps = append(steps, h.Div(h.Class("min-w-0"),
			h.Div(h.Class("flex flex-wrap items-baseline justify-between gap-1 mb-1"),
				h.Span(h.Class("text-sm font-medium"), g.Text(s.Label)),
				h.Span(h.Class("text-sm text-base-content/70"),
					g.Text(strconv.FormatInt(s.Count, 10)+" · "+s.Pct)),
			),
			reportBar(s.BarPct),
		))
	}
	stats := h.Div(h.Class("grid grid-cols-1 sm:grid-cols-3 gap-3 mt-3"),
		reportStat("Konversi Lead→Menang", v.Conversion, "text-primary"),
		reportStat("Rata Lead→Deal", v.AvgLeadToDeal, "text-base-content"),
		reportStat("Rata Deal→Menang", v.AvgDealToWon, "text-base-content"),
	)
	return reportPanelCard(v.Base, "Lead Conversion", "funnel",
		h.Div(h.Class("grid gap-3 min-w-0"), g.Group(steps)), stats)
}

// ── Panel 5: Sales Activity Report ──────────────────────────────────────────

func reportsActivityPanel(v ReportsSalesView) g.Node {
	var body g.Node
	if len(v.Activity) == 0 {
		body = reportEmpty("Belum ada aktivitas sales.")
	} else {
		rows := make([]g.Node, 0, len(v.Activity))
		for _, r := range v.Activity {
			rows = append(rows, h.Tr(
				h.Class("border-b border-base-300/50"),
				rTd("font-medium break-words", g.Text(r.Owner)),
				rTd("", g.Text(strconv.FormatInt(r.Call, 10))),
				rTd("", g.Text(strconv.FormatInt(r.Email, 10))),
				rTd("", g.Text(strconv.FormatInt(r.Meeting, 10))),
				rTd("", g.Text(strconv.FormatInt(r.Total, 10))),
				rTd("", g.Text(strconv.FormatInt(r.Won, 10))),
				rTd("", g.Text(r.PerDeal)),
			))
		}
		body = ui.TableScroll(h.Table(
			h.Class("w-full text-sm"),
			h.THead(h.Tr(
				h.Class("border-b border-base-300 text-left text-base-content/70"),
				rTh("Sales"), rTh("Panggilan"), rTh("Email"), rTh("Meeting"),
				rTh("Total"), rTh("Deal Menang"), rTh("Aktivitas/Deal"),
			)),
			h.TBody(g.Group(rows)),
		))
	}
	return reportPanelCard(v.Base, "Sales Activity Report", "activity", body)
}
