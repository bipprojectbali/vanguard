package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_quotes_index.go — daftar quote LINTAS-deal (menu sidebar "Quotes"). Beda
// dari QuotesList (per-deal): ada kolom "Deal" & tiap baris menaut ke quote di
// bawah deal induknya (/deals/{dealID}/quotes/{quoteID}). Tanpa tombol "Buat
// Quote" — quote dibuat dari detail deal (mewarisi account/deal). Murni-data:
// handler sudah memformat angka & memutuskan cakupan (F3).

// QuoteIndexRow = satu quote pada daftar global. Angka SUDAH diformat handler;
// Status mentah (badge diputuskan view). DealID dibawa untuk merakit tautan.
type QuoteIndexRow struct {
	QuoteID    int64
	DealID     int64
	EntityCode string
	QuoteName  string
	DealName   string
	Status     string
	GrandTotal string
}

// QuotesIndexView = data halaman daftar quote global.
type QuotesIndexView struct {
	Base       string
	Err        string
	Msg        string
	Query      string // ?q= pencarian bebas (BL-6); "" = tak mencari
	Items      []QuoteIndexRow
	NextCursor string
	After      string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail      string // BL-7: jejak cursor halaman sebelumnya (?trail=)
}

// QuotesIndex merender header + alert + tabel quote lintas-deal (atau state kosong)
// + pager keyset.
func QuotesIndex(v QuotesIndexView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("min-w-0 mb-2"),
			h.H1(h.Class("text-xl font-semibold truncate"), g.Text("Quote")),
			h.P(h.Class("text-base-content/70 truncate"),
				g.Text("Semua penawaran dalam cakupan Anda. Buat quote dari detail deal.")),
		),
	}
	body = append(body, searchBox(v.Base+"/quotes", v.Query,
		"Cari quote — nama, kode, atau deal…", "Cari quote"))
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "quotes-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "quotes-ok", g.Text(v.Msg)))
	}

	if len(v.Items) == 0 {
		body = append(body, emptyQuotesIndex(v))
	} else {
		body = append(body, quotesIndexTable(v.Base, v.Items), quotesIndexPager(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// quotesIndexTable = tabel quote global, dibungkus ui.TableScroll (scroll terkurung,
// mobile-first) — kolom "Deal" tak ada di daftar per-deal.
func quotesIndexTable(base string, items []QuoteIndexRow) g.Node {
	rows := make([]g.Node, 0, len(items))
	for _, q := range items {
		rows = append(rows, quotesIndexRow(base, q))
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
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Nama")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Deal")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
					h.Th(h.Class("py-2 font-medium"), g.Text("Grand Total")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func quotesIndexRow(base string, q QuoteIndexRow) g.Node {
	href := base + "/deals/" + strconv.FormatInt(q.DealID, 10) +
		"/quotes/" + strconv.FormatInt(q.QuoteID, 10)
	dealHref := base + "/deals/" + strconv.FormatInt(q.DealID, 10)
	link := func(text, cls string) g.Node {
		return h.Td(h.Class(cls), h.A(h.Href(href), h.Class("block truncate"), g.Text(text)))
	}
	return h.Tr(
		h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
		link(orDash(q.EntityCode), "py-2 pr-4 font-mono text-xs"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), h.Class("block truncate font-medium"),
			g.Text(orDash(q.QuoteName)))),
		// Kolom Deal menaut ke detail DEAL induk (bukan quote) — konteks berbeda.
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(dealHref),
			h.Class("block truncate text-base-content/80 hover:underline"),
			g.Text(orDash(q.DealName)))),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), quoteStatusBadge(q.Status))),
		link(orDash(q.GrandTotal), "py-2"),
	)
}

// emptyQuotesIndex = state kosong jujur. Pencarian/halaman-setelah-cursor yang
// kosong menawarkan jalan kembali (mempertahankan q); daftar yang benar-benar
// kosong mengarahkan cara membuat quote (dari detail deal).
func emptyQuotesIndex(v QuotesIndexView) g.Node {
	if v.NextCursor == "" && v.Query == "" {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body"),
				h.P(h.Class("text-base-content/70"),
					g.Text("Belum ada quote. Buka sebuah deal untuk membuat penawaran."))),
		)
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body items-start"),
			h.P(h.Class("text-base-content/70"),
				g.Text("Belum ada quote yang cocok pada tampilan ini.")),
			h.A(h.Href(withQuery(v.Base+"/quotes", v.Query)),
				h.Class("btn btn-ghost btn-sm min-h-11"), g.Text("« Kembali ke awal")),
		),
	)
}

func quotesIndexPager(v QuotesIndexView) g.Node {
	base := panelListHref(v.Base+"/quotes", [2]string{"q", v.Query})
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}
