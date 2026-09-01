package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_quotes.go — view DAFTAR quote satu deal (ter-nest di bawah deal).
// Murni-data: GrandTotal SUDAH diformat handler. Meniru sales_deals.go (Tabel).
// Quote hidup DI BAWAH deal (keputusan scope 3) → semua path lewat DealID; tak
// ada daftar quote global. quoteStatusBadge dipakai bersama builder detail.

// QuoteRow = satu quote untuk baris daftar / kartu di detail deal. Angka SUDAH
// diformat handler; Status mentah (badge diputuskan view).
type QuoteRow struct {
	ID         int64
	EntityCode string
	QuoteName  string
	Status     string
	GrandTotal string
	Expiration string
}

// QuotesListView = data halaman daftar quote satu deal. Quotable (BL-13) =
// deal di jendela quoting → boleh buat quote; StageLockMsg = alasan terkunci
// (dipakai banner saat pemegang tulis diblokir stage). Flag di-precompute handler.
type QuotesListView struct {
	Base         string
	DealID       int64
	DealName     string
	CanWrite     bool
	Quotable     bool
	StageLockMsg string
	Err          string
	Msg          string
	Items        []QuoteRow
	NextCursor   string
}

// QuotesList merender header + alert + tabel quote (atau state kosong) + pager.
func QuotesList(v QuotesListView) g.Node {
	dealBase := v.Base + "/deals/" + strconv.FormatInt(v.DealID, 10)
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.Class("min-w-0"),
				h.H1(h.Class("text-xl font-semibold truncate"), g.Text("Quote")),
				h.P(h.Class("text-base-content/70 truncate"),
					g.Text("Penawaran untuk deal "+v.DealName+".")),
			),
			ui.When(v.CanWrite && v.Quotable, h.A(
				h.Href(dealBase+"/quotes/new"), h.Class("btn btn-primary min-h-11"),
				g.Text("Buat Quote"),
			)),
		),
		h.A(h.Href(dealBase), h.Class("text-sm text-base-content/60"),
			g.Text("« Kembali ke deal")),
	}
	// BL-13: pemegang tulis yang diblokir stage diberi alasan (bukan tombol hilang senyap).
	if v.CanWrite && !v.Quotable && v.StageLockMsg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "quotes-lock", g.Text(v.StageLockMsg)))
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "quotes-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "quotes-ok", g.Text(v.Msg)))
	}

	if len(v.Items) == 0 {
		body = append(body, emptyQuotes())
	} else {
		body = append(body, quotesTable(dealBase, v.Items), quotesPager(v, dealBase))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// quotesTable = tabel quote, dibungkus ui.TableScroll (scroll terkurung, mobile-first).
func quotesTable(dealBase string, items []QuoteRow) g.Node {
	rows := make([]g.Node, 0, len(items))
	for _, q := range items {
		rows = append(rows, quoteTableRow(dealBase, q))
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
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Grand Total")),
					h.Th(h.Class("py-2 font-medium"), g.Text("Kedaluwarsa")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func quoteTableRow(dealBase string, q QuoteRow) g.Node {
	href := dealBase + "/quotes/" + strconv.FormatInt(q.ID, 10)
	link := func(text, cls string) g.Node {
		return h.Td(h.Class(cls), h.A(h.Href(href), h.Class("block truncate"), g.Text(text)))
	}
	return h.Tr(
		h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
		link(orDash(q.EntityCode), "py-2 pr-4 font-mono text-xs"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), h.Class("block truncate font-medium"),
			g.Text(orDash(q.QuoteName)))),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), quoteStatusBadge(q.Status))),
		link(orDash(q.GrandTotal), "py-2 pr-4"),
		link(orDash(q.Expiration), "py-2"),
	)
}

// quoteStatusBadge = badge status quote (token semantik daisyUI, bukan absolut).
// Accepted=success, Rejected/Expired=error, Sent/Under Review=info, Draft=ghost.
func quoteStatusBadge(status string) g.Node {
	cls := "badge badge-ghost"
	switch status {
	case "Accepted":
		cls = "badge badge-success"
	case "Rejected", "Expired":
		cls = "badge badge-error"
	case "Sent", "Under Review":
		cls = "badge badge-info"
	}
	return h.Span(h.Class(cls), g.Text(orDash(status)))
}

func emptyQuotes() g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body"),
			h.P(h.Class("text-base-content/70"), g.Text("Belum ada quote untuk deal ini."))),
	)
}

func quotesPager(v QuotesListView, dealBase string) g.Node {
	if v.NextCursor == "" {
		return h.Div(h.Class("flex flex-wrap items-center gap-2"),
			h.Span(h.Class("text-sm text-base-content/60"), g.Text("Ujung daftar.")))
	}
	return h.Div(
		h.Class("flex flex-wrap items-center gap-2"),
		h.A(h.Href(dealBase+"/quotes?after="+v.NextCursor), h.Class("btn min-h-11"),
			g.Text("Berikutnya »")),
	)
}
