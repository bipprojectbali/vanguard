package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// accounts_table.go — tabel daftar desa + pager (dipisah dari accounts.go demi
// batas File Health View/Component). Murni-data: tiap nilai SUDAH diputuskan
// handler. Dibungkus ui.TableScroll (scroll terkurung, tak mendorong lebar
// halaman di mobile); baris seluruhnya tertaut ke detail; pager = keyset.

// accountsTable = tabel daftar desa. Dibungkus ui.TableScroll (scroll terkurung,
// tak mendorong lebar halaman di mobile). Baris seluruhnya tertaut ke detail.
func accountsTable(v AccountsListView) g.Node {
	rows := make([]g.Node, 0, len(v.Items))
	for _, a := range v.Items {
		rows = append(rows, accountRow(v.Base, a))
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
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Tipe")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Kab/Kota")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Provinsi")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Owner")),
					h.Th(h.Class("py-2 font-medium"), g.Text("CS")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func accountRow(base string, a AccountRow) g.Node {
	href := base + "/accounts/" + strconv.FormatInt(a.ID, 10)
	link := func(text string, cls string) g.Node {
		return h.Td(h.Class(cls), h.A(h.Href(href), h.Class("block truncate"), g.Text(text)))
	}
	return h.Tr(
		h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), h.Class("block min-w-0"),
			h.Div(h.Class("truncate font-medium"), g.Text(a.VillageName)),
			ui.When(a.VillageCode != "", h.Div(
				h.Class("truncate text-xs text-base-content/60"),
				g.Text("Kode desa "+a.VillageCode))),
		)),
		link(a.AccountType, "py-2 pr-4"),
		link(orDash(a.Regency), "py-2 pr-4"),
		link(orDash(a.Province), "py-2 pr-4"),
		link(orDash(a.OwnerName), "py-2 pr-4"),
		link(orDash(a.CSMName), "py-2"),
	)
}

// accountsPager = jalan ke halaman berikutnya (keyset). Link biasa (navigasi:
// bookmarkable + dimuat ulang, lolos gotcha #16), tap target 44px, flex-wrap
// untuk 375px. Ujung daftar dikatakan eksplisit.
func accountsPager(v AccountsListView) g.Node {
	return ui.KeysetPager(accountsListHref(v.Base, v.ActiveView, v.Query), v.After, v.Trail, v.NextCursor)
}
