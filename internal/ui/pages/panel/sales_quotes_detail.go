package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_quotes_detail.go — Quote Builder (detail satu quote). Murni-data: semua
// angka SUDAH diformat handler; unit_price = SNAPSHOT harga saat item dibuat
// (harga beku, acceptance M4). Semua aksi = NATIVE POST → 303 (gotcha #16), BUKAN
// Datastar (navigasi/redirect diblokir CSP). Reuse detailCard/detailField,
// field/selectField/labelFor dari accounts_*/deals_*; quoteStatusBadge dari
// sales_quotes.go. Kontrol tulis hanya muncul bila CanWrite (gate backend tetap
// penjaga sesungguhnya).

// QuoteItemRow = satu baris item siap render. Angka diformat; Quantity/Discount
// mentah dipertahankan untuk prefill form sunting per-item.
type QuoteItemRow struct {
	ID        int64
	PlanLabel string
	Quantity  string
	UnitPrice string
	Discount  string // persen mentah (mis. "10" / ""), untuk prefill
	Subtotal  string
}

// QuotePlanOption = satu opsi picker plan pada form tambah item.
type QuotePlanOption struct {
	ID    int64
	Label string
}

// QuoteDetailView = seluruh data builder siap render. Statuses = opsi kontrol
// status. Plans = katalog aktif untuk tambah item. Subtotal/Tax/GrandTotal SUDAH
// diformat handler.
type QuoteDetailView struct {
	Base       string
	DealID     int64
	ID         int64
	EntityCode string

	QuoteName string
	Status    string
	Statuses  []string

	AccountLabel string
	Expiration   string
	PreparedBy   string
	PaymentTerms string
	NotesTerms   string

	Subtotal   string
	Tax        string
	GrandTotal string

	Items []QuoteItemRow
	Plans []QuotePlanOption

	CanWrite bool
}

// QuoteDetail merender builder: header (nama+kode+status+aksi), kartu identitas,
// tabel line items + total, lalu (bila boleh tulis) kelola item, tambah item, &
// kontrol status.
func QuoteDetail(v QuoteDetailView) g.Node {
	dealBase := v.Base + "/deals/" + strconv.FormatInt(v.DealID, 10)
	quoteBase := dealBase + "/quotes/" + strconv.FormatInt(v.ID, 10)

	title := v.QuoteName
	if title == "" {
		title = v.EntityCode
	}
	if title == "" {
		title = "Quote"
	}

	header := h.Div(
		h.Class("flex flex-wrap items-start justify-between gap-2"),
		h.Div(
			h.Class("min-w-0"),
			h.H1(h.Class("text-xl font-semibold truncate"), g.Text(title)),
			h.Div(
				h.Class("flex flex-wrap items-center gap-2 mt-1"),
				ui.When(v.EntityCode != "", h.Span(
					h.Class("badge badge-neutral font-mono"), g.Text(v.EntityCode))),
				quoteStatusBadge(v.Status),
			),
		),
		ui.When(v.CanWrite, h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.A(h.Href(quoteBase+"/edit"), h.Class("btn btn-sm min-h-11"), g.Text("Sunting")),
			deleteQuoteForm(quoteBase),
		)),
	)

	body := []g.Node{
		header,
		h.A(h.Href(dealBase+"/quotes"), h.Class("text-sm text-base-content/60"),
			g.Text("« Kembali ke daftar quote")),
		detailCard("Identitas Quote", []detailField{
			{"Desa", v.AccountLabel},
			{"Kedaluwarsa", v.Expiration},
			{"Disusun oleh", v.PreparedBy},
			{"Termin Pembayaran", v.PaymentTerms},
			{"Catatan / Syarat", v.NotesTerms},
		}),
		quoteLineItems(v, quoteBase),
	}
	if v.CanWrite {
		body = append(body,
			quoteAddItemForm(v, quoteBase),
			quoteStatusControl(v, quoteBase),
		)
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

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
	if v.CanWrite {
		head = append(head, h.Th(h.Class("py-2 font-medium"), g.Text("Aksi")))
	}

	rows := make([]g.Node, 0, len(v.Items))
	for _, it := range v.Items {
		rows = append(rows, quoteItemTableRow(v, it, quoteBase))
	}
	if len(rows) == 0 {
		span := 5
		if v.CanWrite {
			span = 6
		}
		rows = append(rows, h.Tr(h.Td(
			g.Attr("colspan", strconv.Itoa(span)),
			h.Class("py-3 text-base-content/60"),
			g.Text("Belum ada item. Tambahkan plan di bawah."),
		)))
	}

	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0 gap-3"),
			h.H2(h.Class("font-semibold"), g.Text("Line Items & Amounts")),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					g.Group(head),
				)),
				h.TBody(g.Group(rows)),
			)),
			quoteTotals(v),
		),
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
	if v.CanWrite {
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
	return h.Div(
		h.Class("ml-auto w-full sm:max-w-xs border-t border-base-300 pt-2"),
		row("Subtotal", v.Subtotal, false),
		row("Pajak", v.Tax, false),
		h.Div(h.Class("border-t border-base-300 mt-1 pt-1"),
			row("Grand Total", v.GrandTotal, true)),
	)
}

// quoteAddItemForm = form tambah item: picker plan (harga di-SNAPSHOT saat submit)
// + qty + diskon. Native POST. Kosong bila katalog plan aktif kosong (tak ada yang
// bisa dijual) → keterangan jujur, bukan form mati.
func quoteAddItemForm(v QuoteDetailView, quoteBase string) g.Node {
	if len(v.Plans) == 0 {
		return h.Div(
			h.Class("card bg-base-100 border border-dashed border-base-300 min-w-0"),
			h.Div(h.Class("card-body min-w-0"),
				h.H2(h.Class("font-semibold mb-1"), g.Text("Tambah Item")),
				h.P(h.Class("text-sm text-base-content/60"),
					g.Text("Belum ada plan aktif di katalog untuk ditambahkan."))),
		)
	}
	placeholder := []g.Node{h.Value(""), h.Disabled(), h.Selected(), g.Text("— Pilih plan —")}
	opts := []g.Node{h.Option(placeholder...)}
	for _, p := range v.Plans {
		opts = append(opts, h.Option(h.Value(strconv.FormatInt(p.ID, 10)), g.Text(p.Label)))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0 gap-3"),
			h.H2(h.Class("font-semibold"), g.Text("Tambah Item")),
			h.P(h.Class("text-sm text-base-content/60"),
				g.Text("Harga satuan dibekukan dari plan saat item ditambahkan.")),
			h.FormEl(
				h.Method("post"), h.Action(quoteBase+"/items"),
				h.Class("grid gap-3 sm:grid-cols-2 min-w-0"),
				h.Div(
					h.Class("grid gap-1 min-w-0 sm:col-span-2"),
					labelFor("Plan", "f-plan_id", true),
					h.Select(
						append([]g.Node{
							h.ID("f-plan_id"), h.Name("plan_id"), h.Required(),
							h.Class("select text-base w-full"),
						}, g.Group(opts))...,
					),
				),
				field("Kuantitas", "quantity", "1", true, "number"),
				field("Diskon (%)", "discount_pct", "", false, "number"),
				h.Div(
					h.Class("sm:col-span-2"),
					h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
						g.Text("Tambah Item")),
				),
			),
		),
	)
}

// quoteStatusControl = kontrol ganti status manual (approval flow ditunda). Native
// POST; backend memvalidasi terhadap allowlist skema.
func quoteStatusControl(v QuoteDetailView, quoteBase string) g.Node {
	opts := make([]g.Node, 0, len(v.Statuses))
	for _, s := range v.Statuses {
		attrs := []g.Node{h.Value(s)}
		if s == v.Status {
			attrs = append(attrs, h.Selected())
		}
		opts = append(opts, h.Option(append(attrs, g.Text(s))...))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0 gap-3"),
			h.H2(h.Class("font-semibold"), g.Text("Ubah Status")),
			h.FormEl(
				h.Method("post"), h.Action(quoteBase+"/status"),
				h.Class("grid gap-3 sm:grid-cols-2 min-w-0"),
				h.Div(
					h.Class("grid gap-1 min-w-0"),
					labelFor("Status", "f-quote_status", true),
					h.Select(
						append([]g.Node{
							h.ID("f-quote_status"), h.Name("quote_status"), h.Required(),
							h.Class("select text-base w-full"),
						}, g.Group(opts))...,
					),
				),
				h.Div(
					h.Class("flex items-end"),
					h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
						g.Text("Simpan Status")),
				),
			),
		),
	)
}

// deleteQuoteForm = tombol hapus quote (soft-delete). Native POST → 303 (gotcha #16).
func deleteQuoteForm(quoteBase string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(quoteBase+"/delete"),
		h.Button(h.Type("submit"), h.Class("btn btn-sm btn-error btn-outline min-h-11"),
			g.Text("Hapus")),
	)
}
