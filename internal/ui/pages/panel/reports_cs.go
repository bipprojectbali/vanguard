package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// reports_cs.go — Customer Success Report (Modul 8, wireframe 8.2): view
// MURNI-DATA, handler (internal/handler/reports_cs.go) yang menghitung.
// KPI cards reuse healthKPICard (health_score_list.go, package sama) &
// HealthScoreKPIs (tipe reuse, tak diredefinisi). Tabel breakdown dibungkus
// ui.TableScroll (WAJIB, gotcha overflow). Scope 8.2 di sini HANYA
// Health/Adoption — NPS/CSAT ditunda (lihat doc comment handler).

// ReportHealthRow = satu baris tabel breakdown health status. AvgScore sudah
// diformat handler (numericStr) — "" bila belum ada desa berskor di grup.
type ReportHealthRow struct {
	Status   string
	Count    int64
	AvgScore string
}

// ReportsCSView — data siap-render /reports/customer-success.
type ReportsCSView struct {
	Base string
	KPIs HealthScoreKPIs
	Rows []ReportHealthRow
}

// ReportsCSBody merender kartu KPI (reuse healthKPICard) + tabel breakdown
// per status + link Export CSV. Mobile-first: KPI grid-cols-2 (dasar) →
// md:grid-cols-4 (pola sama healthScoreKPICards).
func ReportsCSBody(v ReportsCSView) g.Node {
	return h.Div(h.Class("grid gap-4 min-w-0"),
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Customer Success Report")),
				h.P(h.Class("text-base-content/70"), g.Text("Health & adoption desa per status — NPS/CSAT menyusul.")),
			),
			h.A(h.Href(v.Base+"/reports/customer-success/export"), h.Class("btn btn-outline min-h-11"), g.Text("Export CSV")),
		),
		reportsCSKPICards(v.KPIs),
		reportsCSTable(v),
	)
}

func reportsCSKPICards(k HealthScoreKPIs) g.Node {
	return h.Div(h.Class("grid grid-cols-2 md:grid-cols-4 gap-3"),
		healthKPICard("Desa Total", strconv.FormatInt(k.Total, 10), "text-base-content"),
		healthKPICard("Sehat", strconv.FormatInt(k.Healthy, 10), "text-success"),
		healthKPICard("Berisiko", strconv.FormatInt(k.AtRisk, 10), "text-warning"),
		healthKPICard("Kritis", strconv.FormatInt(k.Critical, 10), "text-error"),
	)
}

// reportsCSTable = tabel breakdown per status, dibungkus ui.TableScroll
// (scroll terkurung, tak meluberkan viewport 375px).
func reportsCSTable(v ReportsCSView) g.Node {
	if len(v.Rows) == 0 {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body items-start"),
				h.P(h.Class("text-base-content/70"), g.Text("Belum ada data desa.")),
			),
		)
	}
	rows := make([]g.Node, 0, len(v.Rows))
	for _, s := range v.Rows {
		rows = append(rows, h.Tr(
			h.Class("border-b border-base-300/50"),
			h.Td(h.Class("py-2 pr-4 font-medium"), g.Text(s.Status)),
			h.Td(h.Class("py-2 pr-4"), g.Text(strconv.FormatInt(s.Count, 10))),
			h.Td(h.Class("py-2"), g.Text(orDash(s.AvgScore))),
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
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Jumlah Desa")),
					h.Th(h.Class("py-2 font-medium"), g.Text("Rata-rata Skor")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}
