package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// subscriptions_detail_items.go — SubItemRow + subItemsCard (kartu "Rincian
// Paket", BL-88 PR2b) untuk SubDetail (subscriptions_detail.go). Dipisah krn
// ambang File Health (View/Component 300 baris); isi APA ADANYA, hanya dipindah.

// SubItemRow = satu baris paket langganan (subscription_items). Nilai komersial
// (Subtotal/MRR/ARR) SUDAH diformat & disamarkan handler (F4); PlanName kosong →
// dirender "—".
type SubItemRow struct {
	PlanName  string
	Quantity  string
	UnitPrice string
	Subtotal  string
	MRR       string
	ARR       string
}

// subItemsCard = rincian paket langganan (subscription_items, BL-88 PR2b). Kosong
// (langganan lama tanpa item) → kartu tak dirender (nil). Dibungkus ui.TableScroll
// (scroll terkurung di mobile), mirror subRenewalChainCard.
func subItemsCard(v SubDetailView) g.Node {
	if len(v.Items) == 0 {
		return nil
	}
	rows := make([]g.Node, 0, len(v.Items))
	for _, it := range v.Items {
		rows = append(rows, h.Tr(
			h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
			h.Td(h.Class("py-2 pr-4 font-medium"), g.Text(orDash(it.PlanName))),
			h.Td(h.Class("py-2 pr-4 text-right"), g.Text(orDash(it.Quantity))),
			h.Td(h.Class("py-2 pr-4 text-right"), g.Text(orDash(it.UnitPrice))),
			h.Td(h.Class("py-2 pr-4 text-right"), g.Text(orDash(it.Subtotal))),
			h.Td(h.Class("py-2 pr-4 text-right"), g.Text(orDash(it.MRR))),
			h.Td(h.Class("py-2 text-right"), g.Text(orDash(it.ARR))),
		))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-2"), g.Text("Rincian Paket")),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Paket")),
					h.Th(h.Class("py-2 pr-4 font-medium text-right"), g.Text("Qty")),
					h.Th(h.Class("py-2 pr-4 font-medium text-right"), g.Text("Harga Satuan")),
					h.Th(h.Class("py-2 pr-4 font-medium text-right"), g.Text("Subtotal")),
					h.Th(h.Class("py-2 pr-4 font-medium text-right"), g.Text("MRR")),
					h.Th(h.Class("py-2 font-medium text-right"), g.Text("ARR")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}
