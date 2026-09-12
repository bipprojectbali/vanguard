package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_deals_detail_quotes.go — kartu pratinjau Quote (Modul 4) di halaman
// detail deal. Tipe & kartu lain di sales_deals_detail.go.

// dealQuotesCard = pratinjau Quote deal ini (Modul 4). Daftar ringkas + tautan
// ke builder/daftar penuh. Tombol "Buat Quote" hanya bila boleh tulis. Quote
// hidup DI BAWAH deal (nested) — semua tautan lewat base deal.
func dealQuotesCard(v DealDetailView) g.Node {
	idStr := strconv.FormatInt(v.ID, 10)
	quotesBase := v.Base + "/deals/" + idStr + "/quotes"

	head := h.Div(
		h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
		h.Div(h.Class("flex flex-wrap items-center gap-2 min-w-0"),
			h.H2(h.Class("font-semibold"), g.Text("Quote")),
			ui.When(v.QuotesSummary.Total > 0, quotesSummaryBadge(v.QuotesSummary)),
		),
		ui.When(v.CanCreateQuote, h.A(
			h.Href(quotesBase+"/new"), h.Class("btn btn-sm btn-primary min-h-11"),
			g.Text("Buat Quote"))),
		// BL-86: saat boleh tulis tapi stage di luar jendela quotable, tombol
		// disembunyikan; tampilkan hint singkat agar user paham kapan bisa.
		ui.When(v.CanWrite && !v.CanCreateQuote && v.QuoteStageLockMsg != "",
			h.Span(h.Class("text-xs text-base-content/60"),
				g.Text(v.QuoteStageLockMsg))),
	)

	var content g.Node
	if len(v.Quotes) == 0 {
		content = h.P(h.Class("text-sm text-base-content/60"),
			g.Text("Belum ada quote untuk deal ini."))
	} else {
		rows := make([]g.Node, 0, len(v.Quotes))
		for _, q := range v.Quotes {
			href := quotesBase + "/" + strconv.FormatInt(q.ID, 10)
			rows = append(rows, h.A(
				h.Href(href),
				h.Class("flex flex-wrap items-center justify-between gap-2 py-2 "+
					"border-b border-base-300/50 last:border-0 hover:bg-base-200/50"),
				h.Span(h.Class("min-w-0 truncate font-medium"),
					g.Text(quoteRowLabel(q))),
				h.Span(h.Class("flex flex-wrap items-center gap-2 shrink-0"),
					quoteStatusBadge(q.Status),
					ui.When(q.Expired, quoteExpiredBadge()),
					h.Span(h.Class("text-sm text-base-content/70"), g.Text(orDash(q.GrandTotal)))),
			))
		}
		content = h.Div(h.Class("min-w-0"),
			h.Div(h.Class("grid"), g.Group(rows)),
			h.A(h.Href(quotesBase), h.Class("text-sm text-base-content/60 mt-2 inline-block"),
				g.Text("Lihat semua quote »")),
		)
	}

	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(h.Class("card-body min-w-0"), head, content),
	)
}

// quoteRowLabel = label ringkas satu quote di kartu deal: nama bila ada, jatuh ke
// kode, lalu id.
func quoteRowLabel(q QuoteRow) string {
	if q.QuoteName != "" {
		return q.QuoteName
	}
	if q.EntityCode != "" {
		return q.EntityCode
	}
	return "Quote #" + strconv.FormatInt(q.ID, 10)
}
