package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_deals_body.go — perender badan pipeline Deal: Kanban (papan per-stage) &
// Tabel (dipisah dari sales_deals.go demi batas File Health View/Component).
// Murni-data: Amount & PipelineValue SUDAH diformat/disamarkan F4 di handler.
// Papan read-only (bukan drag-drop, keputusan terkunci #1); tiap kartu/baris =
// tautan ke detail.

// dealKanban = papan per-stage. Lebar-konten → discroll DALAM kontainer sendiri
// (overflow-x-auto), tak meluberkan halaman (mobile-first). Tiap kolom min-w agar
// terbaca; kartu = tautan ke detail.
func dealKanban(v DealPipelineView) g.Node {
	cols := make([]g.Node, 0, len(v.Stages))
	for _, col := range v.Stages {
		cols = append(cols, dealKanbanColumn(v.Base, col))
	}
	return h.Div(
		h.Class("overflow-x-auto min-w-0 pb-2"),
		h.Div(h.Class("flex gap-3 items-start"), g.Group(cols)),
	)
}

func dealKanbanColumn(base string, col DealStageColumn) g.Node {
	cards := make([]g.Node, 0, len(col.Cards))
	for _, d := range col.Cards {
		cards = append(cards, dealCard(base, d))
	}
	if len(cards) == 0 {
		cards = append(cards, h.P(h.Class("text-xs text-base-content/50 py-2"),
			g.Text("Tak ada deal.")))
	}
	return h.Div(
		h.Class("w-64 shrink-0 rounded-box bg-base-200 p-2"),
		h.Div(
			h.Class("flex items-center justify-between px-1 pb-2"),
			h.Span(h.Class("text-sm font-medium truncate"), g.Text(col.Stage)),
			h.Span(h.Class("badge badge-ghost badge-sm"),
				g.Text(strconv.Itoa(len(col.Cards)))),
		),
		h.Div(h.Class("grid gap-2"), g.Group(cards)),
	)
}

// dealCard = kartu ringkas satu deal (nama · nilai · probabilitas), tautan ke detail.
func dealCard(base string, d DealRow) g.Node {
	href := base + "/deals/" + strconv.FormatInt(d.ID, 10)
	meta := []g.Node{}
	if d.Amount != "" {
		meta = append(meta, h.Span(h.Class("font-medium"), g.Text(d.Amount)))
	}
	if d.Probability != "" {
		meta = append(meta, h.Span(h.Class("text-base-content/60"),
			g.Text(d.Probability+"%")))
	}
	return h.A(
		h.Href(href),
		h.Class("card bg-base-100 border border-base-300 hover:border-primary/50 min-w-0"),
		h.Div(
			h.Class("card-body p-3 gap-1 min-w-0"),
			h.Div(h.Class("truncate font-medium text-sm"), g.Text(d.DealName)),
			ui.When(d.EntityCode != "", h.Div(
				h.Class("truncate font-mono text-xs text-base-content/50"),
				g.Text(d.EntityCode))),
			ui.When(len(meta) > 0, h.Div(
				h.Class("flex flex-wrap items-center gap-2 text-xs"), g.Group(meta))),
		),
	)
}

// dealsTable = tampilan Tabel deal, dibungkus ui.TableScroll (scroll terkurung).
func dealsTable(v DealPipelineView) g.Node {
	rows := make([]g.Node, 0, len(v.Items))
	for _, d := range v.Items {
		rows = append(rows, dealTableRow(v.Base, d))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Kode")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Deal")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Tahap")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Nilai")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Peluang")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Perkiraan Tutup")),
					h.Th(h.Class("py-2 font-medium"), g.Text("Pemilik")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func dealTableRow(base string, d DealRow) g.Node {
	href := base + "/deals/" + strconv.FormatInt(d.ID, 10)
	link := func(text, cls string) g.Node {
		return h.Td(h.Class(cls), h.A(h.Href(href), h.Class("block truncate"), g.Text(text)))
	}
	prob := ""
	if d.Probability != "" {
		prob = d.Probability + "%"
	}
	return h.Tr(
		h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
		link(orDash(d.EntityCode), "py-2 pr-4 font-mono text-xs"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), h.Class("block truncate font-medium"),
			g.Text(d.DealName))),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), dealStageBadge(d.Stage))),
		link(orDash(d.Amount), "py-2 pr-4"),
		link(orDash(prob), "py-2 pr-4"),
		link(orDash(d.ExpectedClose), "py-2 pr-4"),
		link(orDash(d.Owner), "py-2"),
	)
}

// dealStageBadge = badge tahap berwarna semantik daisyUI (token, bukan absolut).
func dealStageBadge(stage string) g.Node {
	cls := "badge badge-ghost"
	switch stage {
	case "Closed Won":
		cls = "badge badge-success"
	case "Closed Lost":
		cls = "badge badge-error"
	case "Negotiation", "Proposal":
		cls = "badge badge-info"
	}
	return h.Span(h.Class(cls), g.Text(orDash(stage)))
}
