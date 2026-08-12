package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// subscriptions_detail.go — halaman detail satu langganan + riwayat rantai
// renewal (Modul 5, M5-3b GET-only). Murni-data: MRR/ARR SUDAH disamarkan handler
// (F4; ARR bisa flsHidden). Reuse detailCard/detailField & subStatusBadge dari
// subscriptions.go. Aksi (renew/churn) belum ada — hanya bacaan.

// SubDetailView = seluruh data satu langganan siap render. Village/Plan sudah
// diresolusi handler (best-effort, label cadangan bila di luar tenant/terhapus).
// Chain = riwayat rantai renewal (lama→baru), tiap periode ARR-nya disamarkan.
type SubDetailView struct {
	Base       string
	ID         int64
	EntityCode string

	Village   string
	AccountID int64
	Plan      string
	Status    string

	MRR          string
	ARR          string
	BillingCycle string
	AutoRenew    bool
	Start        string
	End          string
	Seats        string
	PaymentState string
	Owner        string

	Chain []SubChainRow
}

// SubChainRow = satu periode di rantai renewal. IsThis menandai baris yang sedang
// dibuka. MRR/ARR sudah diformat & disamarkan handler.
type SubChainRow struct {
	ID     int64
	IsThis bool
	Status string
	MRR    string
	ARR    string
	Start  string
	End    string
}

// SubDetail merender detail: header (desa + kode + status), kartu identitas,
// kartu nilai & masa berlaku, lalu riwayat rantai renewal.
func SubDetail(v SubDetailView) g.Node {
	header := h.Div(
		h.Class("flex flex-wrap items-start justify-between gap-2"),
		h.Div(
			h.Class("min-w-0"),
			h.H1(h.Class("text-xl font-semibold truncate"), g.Text(orDash(v.Village))),
			h.Div(
				h.Class("flex flex-wrap items-center gap-2 mt-1"),
				ui.When(v.EntityCode != "", h.Span(
					h.Class("badge badge-neutral font-mono"), g.Text(v.EntityCode))),
				subStatusBadge(v.Status),
			),
		),
	)

	villageLink := h.A(
		h.Href(v.Base+"/accounts/"+strconv.FormatInt(v.AccountID, 10)),
		h.Class("link link-hover"), g.Text(orDash(v.Village)))

	return h.Div(
		h.Class("grid gap-4 min-w-0"),
		header,
		h.A(h.Href(v.Base+"/subscriptions"), h.Class("text-sm text-base-content/60"),
			g.Text("« Kembali ke daftar langganan")),
		subIdentityCard(v, villageLink),
		detailCard("Nilai & Masa Berlaku", []detailField{
			{"MRR", v.MRR},
			{"ARR", v.ARR},
			{"Siklus Tagih", v.BillingCycle},
			{"Perpanjang Otomatis", autoRenewLabel(v.AutoRenew)},
			{"Mulai", v.Start},
			{"Berakhir", v.End},
			{"Jumlah Seat", v.Seats},
			{"Status Pembayaran", v.PaymentState},
		}),
		subRenewalChainCard(v),
	)
}

// subIdentityCard = kartu inti; Desa dirender sebagai TAUTAN (membuka detail
// desa), sisanya field biasa.
func subIdentityCard(v SubDetailView, villageLink g.Node) g.Node {
	row := func(label string, value g.Node) g.Node {
		return h.Div(
			h.Class("grid gap-1 sm:grid-cols-3 sm:gap-2 py-2 border-b border-base-300/50 last:border-0"),
			h.Dt(h.Class("text-sm text-base-content/60"), g.Text(label)),
			h.Dd(h.Class("sm:col-span-2 break-words"), value),
		)
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-2"), g.Text("Identitas Langganan")),
			h.Dl(
				h.Class("min-w-0"),
				row("Desa", villageLink),
				row("Paket", g.Text(orDash(v.Plan))),
				row("Pemilik", g.Text(orDash(v.Owner))),
			),
		),
	)
}

// subRenewalChainCard = riwayat rantai renewal (lama→baru). Baris yang sedang
// dibuka ditandai. Dibungkus ui.TableScroll (scroll terkurung di mobile).
func subRenewalChainCard(v SubDetailView) g.Node {
	head := h.H2(h.Class("font-semibold mb-2"), g.Text("Riwayat Perpanjangan"))
	if len(v.Chain) <= 1 {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300 min-w-0"),
			h.Div(h.Class("card-body min-w-0"), head,
				h.P(h.Class("text-sm text-base-content/60"),
					g.Text("Belum ada perpanjangan — ini periode langganan pertama."))),
		)
	}
	rows := make([]g.Node, 0, len(v.Chain))
	for _, c := range v.Chain {
		rows = append(rows, subChainRow(v.Base, c))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			head,
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Periode")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("MRR")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("ARR")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Mulai")),
					h.Th(h.Class("py-2 font-medium"), g.Text("Berakhir")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func subChainRow(base string, c SubChainRow) g.Node {
	href := base + "/subscriptions/" + strconv.FormatInt(c.ID, 10)
	period := g.Node(h.A(h.Href(href), h.Class("link link-hover"),
		g.Text("Langganan #"+strconv.FormatInt(c.ID, 10))))
	if c.IsThis {
		period = h.Span(h.Class("flex items-center gap-2"),
			g.Text("Langganan #"+strconv.FormatInt(c.ID, 10)),
			h.Span(h.Class("badge badge-primary badge-sm"), g.Text("Ini")))
	}
	return h.Tr(
		h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
		h.Td(h.Class("py-2 pr-4 font-medium"), period),
		h.Td(h.Class("py-2 pr-4"), subStatusBadge(c.Status)),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(c.MRR))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(c.ARR))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(c.Start))),
		h.Td(h.Class("py-2"), g.Text(orDash(c.End))),
	)
}

// autoRenewLabel = tampilan boolean perpanjang-otomatis dalam bahasa manusia.
func autoRenewLabel(on bool) string {
	if on {
		return "Ya"
	}
	return "Tidak"
}
