package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_deals.go — view hub Deal: pipeline (KPI + Kanban) & tampilan Tabel.
// Murni-data: Amount & PipelineValue SUDAH diformat/disamarkan F4 di handler.
// Meniru sales_leads.go/accounts.go. Ganti stage = kontrol aksi di detail (native
// POST), BUKAN drag-drop (keputusan terkunci #1) — papan ini read-only.

// DealRow = satu deal untuk kartu Kanban / baris Tabel. Amount SUDAH diformat &
// disamarkan F4 di handler. Owner = nama orang.
type DealRow struct {
	ID            int64
	EntityCode    string
	DealName      string
	Stage         string
	Amount        string
	Probability   string
	ExpectedClose string
	Owner         string
}

// DealStageColumn = satu kolom Kanban (satu stage + kartu di dalamnya, sudah
// terurut handler).
type DealStageColumn struct {
	Stage string
	Cards []DealRow
}

// DealPipelineView = data halaman Deal. View "" = pipeline (Kanban+KPI), "table"
// = Tabel berkeyset. KPI (OpenCount/PipelineValue/WinRate) hanya dipakai view
// pipeline; Items/NextCursor hanya view Tabel.
type DealPipelineView struct {
	Base     string
	View     string
	CanWrite bool
	Err      string
	Msg      string

	OpenCount     string
	PipelineValue string
	WinRate       string
	Stages        []DealStageColumn

	StageFilter string
	Query       string // ?q= pencarian bebas (BL-6, hanya view Tabel); "" = tak mencari
	Items       []DealRow
	NextCursor  string
}

// DealPipeline merender halaman: header + toggle tampilan + alert + (Kanban+KPI
// | Tabel).
func DealPipeline(v DealPipelineView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Deals")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Pipeline penjualan — pantau tahap, nilai, dan peluang menang.")),
			),
			ui.When(v.CanWrite, h.A(
				h.Href(v.Base+"/deals/new"), h.Class("btn btn-primary min-h-11"),
				g.Text("Deal Baru"),
			)),
		),
		dealViewToggle(v),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "deals-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "deals-ok", g.Text(v.Msg)))
	}

	if v.View == "table" {
		// Search hanya di view Tabel (berkeyset); papan Kanban di luar lingkup slice.
		body = append(body, searchBox(v.Base+"/deals", v.Query,
			"Cari deal — nama atau kode…", "Cari deal",
			hiddenField{"view", "table"}, hiddenField{"stage", v.StageFilter}))
		if len(v.Items) == 0 {
			body = append(body, emptyDeals(v))
		} else {
			body = append(body, dealsTable(v), dealsPager(v))
		}
	} else {
		body = append(body, dealKPIs(v), dealKanban(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// dealViewToggle = dua LINK <a> (Pipeline / Tabel) — navigasi bookmarkable
// (lolos gotcha #16). Tampilan aktif ditandai.
func dealViewToggle(v DealPipelineView) g.Node {
	tab := func(label, view string) g.Node {
		href := v.Base + "/deals"
		if view != "" {
			href += "?view=" + view
		}
		cls := "tab"
		if v.View == view {
			cls += " tab-active font-medium"
		}
		return h.A(h.Href(href), h.Class(cls+" min-h-11"), g.Text(label))
	}
	return h.Div(h.Role("tablist"), h.Class("tabs tabs-bordered flex-wrap"),
		tab("Pipeline", ""), tab("Tabel", "table"))
}

// dealKPIs = tiga kartu ringkas: deal terbuka, nilai pipeline (F4), win rate.
func dealKPIs(v DealPipelineView) g.Node {
	card := func(label, value string) g.Node {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300 min-w-0"),
			h.Div(h.Class("card-body p-4 min-w-0"),
				h.P(h.Class("text-sm text-base-content/60"), g.Text(label)),
				h.P(h.Class("text-xl font-semibold truncate"), g.Text(orDash(value))),
			),
		)
	}
	return h.Div(
		h.Class("grid gap-3 grid-cols-1 sm:grid-cols-3 min-w-0"),
		card("Deal Terbuka", v.OpenCount),
		card("Nilai Pipeline", v.PipelineValue),
		card("Win Rate", v.WinRate),
	)
}

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

func emptyDeals(v DealPipelineView) g.Node {
	if v.NextCursor == "" && v.StageFilter == "" && v.Query == "" {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body"),
				h.P(h.Class("text-base-content/70"), g.Text("Belum ada deal."))),
		)
	}
	// Kembali ke awal mempertahankan view=table + stage + q agar tak melompat keluar.
	back := withQuery(v.Base+"/deals", v.Query,
		hiddenField{"view", "table"}, hiddenField{"stage", v.StageFilter})
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body items-start"),
			h.P(h.Class("text-base-content/70"),
				g.Text("Belum ada deal yang cocok pada tampilan ini.")),
			h.A(h.Href(back), h.Class("btn btn-ghost btn-sm min-h-11"),
				g.Text("« Kembali ke awal")),
		),
	)
}

func dealsPager(v DealPipelineView) g.Node {
	if v.NextCursor == "" {
		return h.Div(h.Class("flex flex-wrap items-center gap-2"),
			h.Span(h.Class("text-sm text-base-content/60"), g.Text("Ujung daftar.")))
	}
	href := v.Base + "/deals?view=table&after=" + v.NextCursor
	if v.StageFilter != "" {
		href += "&stage=" + v.StageFilter
	}
	href = appendQuery(href, v.Query) // q bertahan ke halaman berikutnya
	return h.Div(
		h.Class("flex flex-wrap items-center gap-2"),
		h.A(h.Href(href), h.Class("btn min-h-11"), g.Text("Berikutnya »")),
	)
}
