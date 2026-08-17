package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// reports_sales.go — Sales Report (Modul 8 M8-1, wireframe 8.1): view
// MURNI-DATA, handler (internal/handler/reports_sales.go) yang menghitung &
// memformat (formatRupiah). Kartu KPI meniru dashboardKPICard (dashboard.go,
// package sama); tabel stage dibungkus ui.TableScroll (WAJIB, gotcha overflow).

// ReportStageRow = satu baris tabel pipeline per-stage. Count/Value SUDAH
// diformat di handler.
type ReportStageRow struct {
	Stage string
	Count int64
	Value string
}

// ReportsSalesView — data siap-render /reports/sales.
type ReportsSalesView struct {
	Base          string
	OpenCount     int64
	PipelineValue string
	WinRate       string
	Stages        []ReportStageRow
}

// ReportsSalesBody merender kartu KPI (pipeline terbuka, nilai pipeline, win
// rate) + tabel per-stage + link Export CSV. Mobile-first: KPI grid-cols-1
// (dasar) → md:grid-cols-3.
func ReportsSalesBody(v ReportsSalesView) g.Node {
	return h.Div(h.Class("grid gap-4 min-w-0"),
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Sales Report")),
				h.P(h.Class("text-base-content/70"), g.Text("Pipeline & win-loss seluruh stage, termasuk Closed Won/Lost.")),
			),
			h.A(h.Href(v.Base+"/reports/sales/export"), h.Class("btn btn-outline min-h-11"), g.Text("Export CSV")),
		),
		reportsSalesKPICards(v),
		reportsSalesTable(v),
	)
}

func reportsSalesKPICards(v ReportsSalesView) g.Node {
	return h.Div(h.Class("grid grid-cols-1 md:grid-cols-3 gap-3"),
		dashboardKPICard("Pipeline Terbuka", strconv.FormatInt(v.OpenCount, 10), "text-base-content"),
		dashboardKPICard("Nilai Pipeline", v.PipelineValue, "text-primary"),
		dashboardKPICard("Win Rate", v.WinRate, "text-success"),
	)
}

// reportsSalesTable = tabel per-stage, dibungkus ui.TableScroll (scroll
// terkurung, tak meluberkan viewport 375px).
func reportsSalesTable(v ReportsSalesView) g.Node {
	if len(v.Stages) == 0 {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body items-start"),
				h.P(h.Class("text-base-content/70"), g.Text("Belum ada data deal.")),
			),
		)
	}
	rows := make([]g.Node, 0, len(v.Stages))
	for _, s := range v.Stages {
		rows = append(rows, h.Tr(
			h.Class("border-b border-base-300/50"),
			h.Td(h.Class("py-2 pr-4 font-medium"), g.Text(s.Stage)),
			h.Td(h.Class("py-2 pr-4"), g.Text(strconv.FormatInt(s.Count, 10))),
			h.Td(h.Class("py-2"), g.Text(s.Value)),
		))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Stage")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Jumlah Deal")),
					h.Th(h.Class("py-2 font-medium"), g.Text("Nilai")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}
