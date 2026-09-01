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
	// Expired (BL-17) = penanda kedaluwarsa computed-on-read, di-precompute
	// handler (Draft dikecualikan). Ditampilkan sebagai badge terpisah dari
	// status — quote bisa kedaluwarsa walau status belum diubah manual ke "Expired".
	Expired bool
}

// QuotesSummary = ringkasan agregat quote satu deal (BL-18), di-precompute
// handler. Total = quote hidup; Expired = kedaluwarsa (aturan BL-17, Draft
// dikecualikan); Active = belum kedaluwarsa & status bukan Rejected/Expired.
// Ditampilkan sebagai teks kecil ("N quote · M kedaluwarsa") di kartu detail deal
// & header daftar penuh — TAK ada aksi baru (Opsi A: quote tetap independen).
type QuotesSummary struct {
	Total   int
	Expired int
	Active  int
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
	After        string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail        string // BL-7: jejak cursor halaman sebelumnya (?trail=)
	// Summary (BL-18) = ringkasan agregat quote deal ini, di-precompute handler
	// (dari SEMUA quote hidup, bukan hanya halaman termuat).
	Summary QuotesSummary
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
				ui.When(v.Summary.Total > 0, h.Div(h.Class("mt-1"),
					quotesSummaryBadge(v.Summary))),
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
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), h.Class("flex flex-wrap items-center gap-1"),
			quoteStatusBadge(q.Status), ui.When(q.Expired, quoteExpiredBadge()))),
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

// quoteExpiredBadge = penanda "Kedaluwarsa" (BL-17), terpisah dari status. Token
// semantik daisyUI `warning` (peringatan, bukan galat/status terminal) — quote
// mungkin masih bisa direvisi ulang dengan re-tanggal. Dipakai list, kartu deal,
// & baris identitas builder.
func quoteExpiredBadge() g.Node {
	return h.Span(h.Class("badge badge-warning badge-sm"), g.Text("Kedaluwarsa"))
}

// quotesSummaryLabel = teks ringkas ringkasan quote: "N quote" (+ " · M
// kedaluwarsa" bila ada yang kedaluwarsa). Total nol → "" (pemanggil sudah punya
// state kosong sendiri).
func quotesSummaryLabel(s QuotesSummary) string {
	if s.Total == 0 {
		return ""
	}
	label := strconv.Itoa(s.Total) + " quote"
	if s.Expired > 0 {
		label += " · " + strconv.Itoa(s.Expired) + " kedaluwarsa"
	}
	return label
}

// quotesSummaryBadge = badge ringkasan agregat (BL-18). Token warning bila ada
// yang kedaluwarsa (tarik perhatian, selaras quoteExpiredBadge), selain itu ghost
// netral. Kosong bila tak ada quote (pemanggil menjaga via ui.When).
func quotesSummaryBadge(s QuotesSummary) g.Node {
	cls := "badge badge-ghost badge-sm"
	if s.Expired > 0 {
		cls = "badge badge-warning badge-sm"
	}
	return h.Span(h.Class(cls), g.Text(quotesSummaryLabel(s)))
}

func emptyQuotes() g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body"),
			h.P(h.Class("text-base-content/70"), g.Text("Belum ada quote untuk deal ini."))),
	)
}

func quotesPager(v QuotesListView, dealBase string) g.Node {
	return ui.KeysetPager(dealBase+"/quotes", v.After, v.Trail, v.NextCursor)
}
