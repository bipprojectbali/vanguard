package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// reports_support.go — Support Report (Modul 8, wireframe 8.3): view
// MURNI-DATA, handler (internal/handler/reports_support.go) yang menghitung.
// KPI cards reuse ticketKPICard (tickets_list.go, package sama) & TicketKPIs
// (tipe reuse, tak diredefinisi) — TANPA href (report bukan navigasi filter
// tab seperti /tickets). Tabel breakdown dibungkus ui.TableScroll (WAJIB).

// ReportTicketStatusRow = satu baris tabel breakdown status tiket.
// AvgHoursToResolve sudah diformat handler (numericStr) — "" bila belum ada
// tiket selesai di grup.
type ReportTicketStatusRow struct {
	Status            string
	Count             int64
	Breached          int64
	AvgHoursToResolve string
}

// ReportsSupportView — data siap-render /reports/support.
type ReportsSupportView struct {
	Base string
	KPIs TicketKPIs
	Rows []ReportTicketStatusRow
}

// ReportsSupportBody merender kartu KPI (reuse ticketKPICard, tanpa href) +
// tabel breakdown per status + link Export CSV. Mobile-first: KPI
// grid-cols-2 (dasar) → md:grid-cols-5 (pola sama ticketKPICards).
func ReportsSupportBody(v ReportsSupportView) g.Node {
	return h.Div(h.Class("grid gap-4 min-w-0"),
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Support Report")),
				h.P(h.Class("text-base-content/70"), g.Text("Volume tiket, SLA, dan resolusi per status.")),
			),
			h.A(h.Href(v.Base+"/reports/support/export"), h.Class("btn btn-outline min-h-11"), g.Text("Export CSV")),
		),
		reportsSupportKPICards(v.KPIs),
		reportsSupportTable(v),
	)
}

func reportsSupportKPICards(k TicketKPIs) g.Node {
	return h.Div(h.Class("grid grid-cols-2 md:grid-cols-5 gap-3 min-w-0"),
		reportsSupportKPICard("Tiket Terbuka", strconv.Itoa(k.Open), ""),
		reportsSupportKPICard("Belum Ditugaskan", strconv.Itoa(k.Unassigned), "text-warning"),
		reportsSupportKPICard("SLA Berisiko", strconv.Itoa(k.AtRisk), "text-warning"),
		reportsSupportKPICard("SLA Terlanggar", strconv.Itoa(k.Breached), "text-error"),
		reportsSupportKPICard("Selesai Hari Ini", strconv.Itoa(k.ResolvedToday), "text-success"),
	)
}

// reportsSupportKPICard = kartu KPI statis (TANPA href — beda dari
// ticketKPICard/tickets_list.go yang link ke tab filter; report bukan
// halaman ber-tab).
func reportsSupportKPICard(label, value, colorCls string) g.Node {
	numCls := "text-2xl font-bold"
	if colorCls != "" {
		numCls += " " + colorCls
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body p-3"),
			h.P(h.Class("text-xs text-base-content/60 truncate"), g.Text(label)),
			h.P(h.Class(numCls), g.Text(value)),
		),
	)
}

// reportsSupportTable = tabel breakdown per status, dibungkus ui.TableScroll
// (scroll terkurung, tak meluberkan viewport 375px).
func reportsSupportTable(v ReportsSupportView) g.Node {
	if len(v.Rows) == 0 {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body items-start"),
				h.P(h.Class("text-base-content/70"), g.Text("Belum ada data tiket.")),
			),
		)
	}
	rows := make([]g.Node, 0, len(v.Rows))
	for _, s := range v.Rows {
		rows = append(rows, h.Tr(
			h.Class("border-b border-base-300/50"),
			h.Td(h.Class("py-2 pr-4 font-medium"), g.Text(s.Status)),
			h.Td(h.Class("py-2 pr-4"), g.Text(strconv.FormatInt(s.Count, 10))),
			h.Td(h.Class("py-2 pr-4"), g.Text(strconv.FormatInt(s.Breached, 10))),
			h.Td(h.Class("py-2"), g.Text(orDash(s.AvgHoursToResolve))),
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
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Jumlah")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("SLA Terlanggar")),
					h.Th(h.Class("py-2 font-medium"), g.Text("Rata-rata Resolusi (jam)")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}
