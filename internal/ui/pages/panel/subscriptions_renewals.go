package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// subscriptions_renewals.go — view dasbor Renewals (Modul 5, Menu 5.2, READ-ONLY).
// Murni-data: RenewalDate/DaysLeft/Prev→Current SUDAH diformat di handler. Tab
// jendela = LINK <a> (navigasi bookmarkable, lolos gotcha #16), bukan Datastar.
// TANPA aksi perpanjangan (dasbor baca) — renew/approve tetap di detail langganan.
// Meniru subscriptions.go (tabel + tab filter + pager keyset).

// RenewalRow = satu langganan untuk baris tabel Renewals. Semua nilai SUDAH
// diformat di handler (RenewalDate, DaysLeft, PrevValue, CurrentMRR = string).
type RenewalRow struct {
	ID          int64
	Village     string
	Plan        string
	RenewalDate string
	DaysLeft    string
	Type        string
	Status      string
	PrevValue   string
	CurrentMRR  string
}

// RenewalWindow = satu tab jendela (key untuk href + label tampilan). Dioper
// handler; view tak memutuskan enum jendela.
type RenewalWindow struct{ Key, Label string }

// RenewalsView = data halaman /subscriptions/renewals. Window = jendela aktif
// terpilih; Windows = opsi tab dioper handler. Keyset lewat NextCursor.
type RenewalsView struct {
	Base       string
	Window     string
	Windows    []RenewalWindow
	Err        string
	Items      []RenewalRow
	NextCursor string
	After      string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail      string // BL-7: jejak cursor halaman sebelumnya (?trail=)
}

// RenewalsList merender halaman: header + tab jendela + alert + tabel + pager.
func RenewalsList(v RenewalsView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Renewals")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Jatuh tempo & status perpanjangan langganan. Aksi perpanjangan dikerjakan di detail langganan.")),
			),
		),
		renewalWindowTabs(v),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "renewals-err", g.Text(v.Err)))
	}
	if len(v.Items) == 0 {
		body = append(body, emptyRenewals())
	} else {
		body = append(body, renewalsTable(v), renewalsPager(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// renewalWindowTabs = baris tab LINK jendela renewal. Navigasi bookmarkable
// (gotcha #16); flex-wrap agar tak mendorong lebar di mobile.
func renewalWindowTabs(v RenewalsView) g.Node {
	tabs := make([]g.Node, 0, len(v.Windows))
	for _, wnd := range v.Windows {
		cls := "tab min-h-11"
		if v.Window == wnd.Key {
			cls += " tab-active font-medium"
		}
		tabs = append(tabs, h.A(
			h.Href(v.Base+"/subscriptions/renewals?window="+wnd.Key),
			h.Class(cls), g.Text(wnd.Label)))
	}
	return h.Div(h.Role("tablist"), h.Class("tabs tabs-bordered flex-wrap"), g.Group(tabs))
}

func emptyRenewals() g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body items-start"),
			h.P(h.Class("text-base-content/70"), g.Text("Tak ada langganan pada jendela ini.")),
		),
	)
}

// renewalsTable = tabel renewal, dibungkus ui.TableScroll (scroll terkurung, tak
// meluberkan viewport 375px). Kolom mengikuti wireframe 5.2.
func renewalsTable(v RenewalsView) g.Node {
	rows := make([]g.Node, 0, len(v.Items))
	for _, s := range v.Items {
		rows = append(rows, renewalTableRow(v.Base, s))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Desa")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Paket")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Tgl Perpanjang")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Sisa Hari")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Jenis")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
					h.Th(h.Class("py-2 font-medium"), g.Text("Prev → Kini")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func renewalTableRow(base string, s RenewalRow) g.Node {
	href := base + "/subscriptions/" + strconv.FormatInt(s.ID, 10)
	link := func(text, cls string) g.Node {
		return h.Td(h.Class(cls), h.A(h.Href(href), h.Class("block truncate"), g.Text(text)))
	}
	return h.Tr(
		h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), h.Class("block truncate font-medium"),
			g.Text(orDash(s.Village)))),
		link(orDash(s.Plan), "py-2 pr-4"),
		link(orDash(s.RenewalDate), "py-2 pr-4"),
		link(orDash(s.DaysLeft), "py-2 pr-4"),
		link(orDash(s.Type), "py-2 pr-4"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), subStatusBadge(s.Status))),
		h.Td(h.Class("py-2"), h.A(h.Href(href), h.Class("block truncate"),
			g.Text(orDash(s.PrevValue)+" → "+orDash(s.CurrentMRR)))),
	)
}

func renewalsPager(v RenewalsView) g.Node {
	base := v.Base + "/subscriptions/renewals?window=" + v.Window
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}
