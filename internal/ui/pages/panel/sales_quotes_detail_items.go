package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_quotes_detail_items.go — tabel Line Items & totals (quoteLineItems,
// quoteItemTableRow, quoteItemActions, quoteTotals), dipisah dari
// sales_quotes_detail.go agar tiap file di bawah ambang tipe View/Component (300).
// Shell quote + form aksi (add/status/delete) tetap di sales_quotes_detail.go.

// quoteLineItems = kartu tabel Line Items & Amounts + baris Subtotal/Tax/Grand.
// Bila CanWrite, tiap baris punya kolom Aksi (sunting inline qty/diskon + hapus).
func quoteLineItems(v QuoteDetailView, quoteBase string) g.Node {
	head := []g.Node{
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Produk / Plan")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Qty")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Harga Satuan")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Diskon %")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Subtotal")),
	}
	if v.CanMutate() {
		head = append(head, h.Th(h.Class("py-2 font-medium"), g.Text("Aksi")))
	}

	rows := make([]g.Node, 0, len(v.Items))
	for _, it := range v.Items {
		rows = append(rows, quoteItemTableRow(v, it, quoteBase))
	}
	if len(rows) == 0 {
		span := 5
		if v.CanMutate() {
			span = 6
		}
		rows = append(rows, h.Tr(h.Td(
			g.Attr("colspan", strconv.Itoa(span)),
			h.Class("py-3 text-base-content/60"),
			g.Text("Belum ada item. Tambahkan lewat tombol Tambah Item."),
		)))
	}

	// Header: judul + (bila boleh mutasi) tombol pemicu modal Tambah Item & Edit
	// Pajak (BL-70) — aksi sesekali kini on-demand, tak lagi kartu selalu-tampil.
	header := []g.Node{h.H2(h.Class("font-semibold"), g.Text("Line Items & Amounts"))}
	if v.CanMutate() {
		header = append(header, h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			modalTrigger("quote-additem", "Tambah Item", "btn btn-sm btn-primary min-h-11"),
			modalTrigger("quote-tax", "Edit Pajak", "btn btn-sm min-h-11"),
		))
	}

	body := []g.Node{
		h.Div(h.Class("flex flex-wrap items-center justify-between gap-2"), g.Group(header)),
		ui.TableScroll(h.Table(
			h.Class("w-full text-sm"),
			h.THead(h.Tr(
				h.Class("border-b border-base-300 text-left text-base-content/70"),
				g.Group(head),
			)),
			h.TBody(g.Group(rows)),
		)),
		quoteTotals(v),
	}
	// Modal dialog (checkbox-toggle CSP-safe) berisi form add-item & pajak; form
	// native POST → 303 di dalamnya utuh. Dirender hanya bila boleh mutasi.
	if v.CanMutate() {
		body = append(body,
			modalDialog("quote-additem", "Tambah Item", quoteAddItemBody(v, quoteBase)),
			modalDialog("quote-tax", "Pajak", quoteTaxBody(v, quoteBase)),
		)
	}

	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(h.Class("card-body min-w-0 gap-3"), g.Group(body)),
	)
}

func quoteItemTableRow(v QuoteDetailView, it QuoteItemRow, quoteBase string) g.Node {
	discount := it.Discount
	if discount == "" {
		discount = "0"
	}
	cells := []g.Node{
		h.Td(h.Class("py-2 pr-4 font-medium"), g.Text(it.PlanLabel)),
		h.Td(h.Class("py-2 pr-4"), g.Text(it.Quantity)),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(it.UnitPrice))),
		h.Td(h.Class("py-2 pr-4"), g.Text(discount)),
		h.Td(h.Class("py-2 pr-4 font-medium"), g.Text(orDash(it.Subtotal))),
	}
	if v.CanMutate() {
		cells = append(cells, h.Td(h.Class("py-2"), quoteItemActions(it, quoteBase)))
	}
	return h.Tr(h.Class("border-b border-base-300/50 align-top"), g.Group(cells))
}

// quoteItemActions = sunting inline (qty + diskon dalam satu form kecil) + hapus
// (form terpisah). unit_price TAK bisa disunting — beku snapshot. Semua native POST.
func quoteItemActions(it QuoteItemRow, quoteBase string) g.Node {
	itemBase := quoteBase + "/items/" + strconv.FormatInt(it.ID, 10)
	return h.Div(
		h.Class("flex flex-wrap items-end gap-2 min-w-0"),
		h.FormEl(
			h.Method("post"), h.Action(itemBase),
			h.Class("flex flex-wrap items-end gap-2"),
			h.Div(
				h.Class("grid gap-1"),
				h.Label(h.Class("text-xs text-base-content/60"), g.Text("Qty")),
				ui.Input(
					h.Type("number"), h.Name("quantity"), h.Value(it.Quantity),
					h.Required(), g.Attr("min", "1"),
					h.Class("input input-sm text-base w-20"),
				),
			),
			h.Div(
				h.Class("grid gap-1"),
				h.Label(h.Class("text-xs text-base-content/60"), g.Text("Diskon %")),
				ui.Input(
					h.Type("number"), h.Name("discount_pct"), h.Value(it.Discount),
					g.Attr("min", "0"), g.Attr("max", "100"), g.Attr("step", "0.01"),
					h.Class("input input-sm text-base w-24"),
				),
			),
			h.Button(h.Type("submit"), h.Class("btn btn-sm min-h-11"), g.Text("Simpan")),
		),
		h.FormEl(
			h.Method("post"), h.Action(itemBase+"/delete"),
			h.Button(h.Type("submit"), h.Class("btn btn-sm btn-error btn-outline min-h-11"),
				g.Text("Hapus")),
		),
	)
}

// quoteTotals = ringkasan Subtotal / Pajak / Grand Total (rata kanan). Grand Total
// ditebalkan. Semua sudah diformat handler.
func quoteTotals(v QuoteDetailView) g.Node {
	row := func(label, value string, strong bool) g.Node {
		valCls := "text-right"
		if strong {
			valCls += " font-semibold text-base"
		}
		return h.Div(
			h.Class("flex items-center justify-between gap-4 py-1"),
			h.Span(h.Class("text-sm text-base-content/70"), g.Text(label)),
			h.Span(h.Class(valCls), g.Text(orDash(value))),
		)
	}
	taxLabel := v.TaxLabel
	if taxLabel == "" {
		taxLabel = "Pajak"
	}
	return h.Div(
		h.Class("ml-auto w-full sm:max-w-xs border-t border-base-300 pt-2"),
		row("Subtotal", v.Subtotal, false),
		row(taxLabel, v.Tax, false),
		h.Div(h.Class("border-t border-base-300 mt-1 pt-1"),
			row("Grand Total", v.GrandTotal, true)),
	)
}
