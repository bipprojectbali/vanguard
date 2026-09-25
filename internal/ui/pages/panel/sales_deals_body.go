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
// Papan Kanban BISA di-drag (BL-75, membalik keputusan lama #1 — papan tadinya
// read-only); atribut kait drag/drop (data-stage-column, draggable, data-deal-*)
// dikonsumsi static/dealboard.js (CSP-safe, BUKAN Datastar). Kartu TETAP tautan
// <a> ke detail — drag hanya lapisan interaksi TAMBAHAN di atasnya, fallback
// non-drag (kontrol native di halaman detail) tetap utuh.

// dealKanban = papan per-stage. Lebar-konten → discroll DALAM kontainer sendiri
// (overflow-x-auto), tak meluberkan halaman (mobile-first). Tiap kolom min-w agar
// terbaca; kartu = tautan ke detail + draggable.
//
// items-stretch (BUKAN items-start) SENGAJA: kolom pendek (sedikit kartu) harus
// tetap membentang setinggi kolom tertinggi di barisnya, kalau tidak area kosong
// di bawah kartu terakhir BUKAN bagian dari elemen [data-stage-column] (flex
// item tinggi = tinggi konten sendiri saat items-start) → drop di area itu jatuh
// ke elemen lain (parent flex/overflow) yang tak listen dragover/drop, terasa
// sebagai "bisa drag tapi tidak bisa drop". data-col-body dibatasi max-h-96 +
// overflow-y-auto agar kolom dgn banyak kartu tak meledakkan tinggi SEMUA kolom
// lain via stretch.
func dealKanban(v DealPipelineView) g.Node {
	cols := make([]g.Node, 0, len(v.Stages))
	for _, col := range v.Stages {
		cols = append(cols, dealKanbanColumn(v.Base, col))
	}
	return h.Div(
		h.Class("overflow-x-auto min-w-0 pb-2"),
		h.Div(h.Class("flex gap-3 items-stretch"), g.Group(cols)),
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
		// data-stage-column = kait drop-target dealboard.js (BL-75); nilai =
		// nama stage APA ADANYA (enum internal, bukan input user). Lebar default
		// w-64 — dealboard.js toggle ke w-80 ("wide")/w-14 ("min") via tombol
		// [data-col-action] & persist localStorage (data-col-state cermin state
		// aktif, diset JS, TANPA nilai default = "normal").
		g.Attr("data-stage-column", col.Stage),
		h.Class("w-64 shrink-0 rounded-box bg-base-200 border-2 border-base-content/20 p-2 flex flex-col min-w-0"),
		// data-col-header-full = header normal/wide (nama tahap + badge + dua
		// tombol aksi); disembunyikan saat state "min" — pada w-14 (56px) baris
		// horizontal ini tak muat, nama tahap kepepet & ikon numpuk (bug
		// dilaporkan user). data-col-header-min = pengganti KHUSUS state "min":
		// badge count di atas, SATU tombol (aksi "min" yg sama — klik lagi saat
		// sudah min = toggle balik "normal", lihat listener [data-col-action] di
		// dealboard.js) di bawah, susun vertikal sempit — meniru papan referensi
		// user. applyColState (dealboard.js) yang menukar display keduanya.
		h.Div(
			g.Attr("data-col-header-full", ""),
			h.Class("flex items-center justify-between gap-1 px-1 pb-2"),
			h.Div(h.Class("flex items-center gap-1 min-w-0"),
				h.Span(h.Class("text-sm font-medium truncate"), g.Text(col.Stage)),
				h.Span(h.Class("badge badge-ghost badge-sm shrink-0"),
					g.Text(strconv.Itoa(len(col.Cards)))),
			),
			h.Div(
				h.Class("flex items-center gap-0.5 shrink-0"),
				h.Button(h.Type("button"), h.Class("btn btn-ghost btn-xs btn-circle"),
					g.Attr("data-col-action", "wide"),
					g.Attr("aria-label", "Lebarkan kolom "+col.Stage), g.Text("↔")),
				h.Button(h.Type("button"), h.Class("btn btn-ghost btn-xs btn-circle"),
					g.Attr("data-col-action", "min"),
					g.Attr("aria-label", "Kecilkan kolom "+col.Stage), g.Text("‹")),
			),
		),
		h.Div(
			g.Attr("data-col-header-min", ""),
			h.Class("flex flex-col items-center gap-1 pb-2"),
			g.Attr("style", "display:none"),
			h.Span(h.Class("badge badge-ghost badge-sm"),
				g.Text(strconv.Itoa(len(col.Cards)))),
			h.Button(h.Type("button"), h.Class("btn btn-ghost btn-xs btn-circle"),
				g.Attr("data-col-action", "min"),
				g.Attr("aria-label", "Lebarkan kolom "+col.Stage), g.Text("›")),
		),
		// data-col-body = kait dealboard.js utk sembunyikan isi saat state "min".
		// max-h-96+overflow-y-auto: batasi tinggi per-kolom (lihat catatan
		// items-stretch di dealKanban) agar kolom panjang scroll sendiri, bukan
		// menaikkan tinggi seluruh baris.
		h.Div(g.Attr("data-col-body", ""), h.Class("grid gap-2 overflow-y-auto max-h-96"),
			g.Group(cards)),
	)
}

// dealBoardControls (BL-75 revisi 25 Sep, referensi UI board eksternal user) =
// baris kendali papan: hint interaksi seleksi + tombol toggle mode Pilih —
// alternatif Ctrl/Cmd/Shift-klik utk perangkat/preferensi tanpa modifier key
// nyaman (mis. layar sentuh). Murni JS (dealboard.js), TANPA state server;
// count & tombol Batal Pilih disembunyikan default, JS tampilkan saat ada
// seleksi aktif.
func dealBoardControls() g.Node {
	return h.Div(
		h.Class("flex flex-wrap items-center justify-between gap-2"),
		h.P(h.Class("text-xs text-base-content/60"),
			g.Text("Ctrl+klik atau aktifkan mode Pilih untuk memilih beberapa kartu sekaligus.")),
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Span(h.ID("deal-select-count"), h.Class("text-xs text-base-content/70"),
				g.Attr("style", "display:none")),
			h.Button(h.Type("button"), h.ID("deal-select-clear"),
				h.Class("btn btn-ghost btn-xs min-h-11"), g.Attr("style", "display:none"),
				g.Text("Batal Pilih")),
			h.Button(h.Type("button"), h.ID("deal-select-toggle"),
				h.Class("btn btn-outline btn-sm min-h-11"), g.Text("☑ Pilih")),
		),
	)
}

// dealCard = kartu ringkas satu deal (nama · nilai · probabilitas), tautan ke
// detail + atribut drag (BL-75): draggable, data-deal-id/-stage/-name dikonsumsi
// dealboard.js utk drag-drop & multiselect; data-quote-accepted HANYA disetel
// utk kartu Negotiation dgn quote Accepted (gate drag-ke-Closed-Won, klien cermin
// resolveWonSubscriptionCore).
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
	attrs := []g.Node{
		h.Href(href),
		h.Class("card bg-base-100 border border-base-300 hover:border-primary/50 min-w-0"),
		h.Draggable("true"),
		g.Attr("data-deal-id", strconv.FormatInt(d.ID, 10)),
		g.Attr("data-deal-stage", d.Stage),
		g.Attr("data-deal-name", d.DealName),
	}
	if d.HasAcceptedQuote {
		attrs = append(attrs, g.Attr("data-quote-accepted", "1"))
	}
	attrs = append(attrs, h.Div(
		h.Class("card-body p-3 gap-1 min-w-0"),
		h.Div(h.Class("truncate font-medium text-sm"), g.Text(d.DealName)),
		ui.When(d.EntityCode != "", h.Div(
			h.Class("truncate font-mono text-xs text-base-content/50"),
			g.Text(d.EntityCode))),
		ui.When(len(meta) > 0, h.Div(
			h.Class("flex flex-wrap items-center gap-2 text-xs"), g.Group(meta))),
	))
	return h.A(attrs...)
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
					h.Th(h.Class("py-2 pr-4 font-medium"), dealSortHeader(v, "code", "Kode")),
					h.Th(h.Class("py-2 pr-4 font-medium"), dealSortHeader(v, "name", "Deal")),
					h.Th(h.Class("py-2 pr-4 font-medium"), dealSortHeader(v, "stage", "Tahap")),
					h.Th(h.Class("py-2 pr-4 font-medium"), dealSortHeader(v, "amount", "Nilai")),
					h.Th(h.Class("py-2 pr-4 font-medium"), dealSortHeader(v, "probability", "Peluang")),
					h.Th(h.Class("py-2 pr-4 font-medium"), dealSortHeader(v, "close", "Perkiraan Tutup")),
					h.Th(h.Class("py-2 font-medium"), dealSortHeader(v, "owner", "Pemilik")),
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
