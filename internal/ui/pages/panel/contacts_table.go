package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// contacts_table.go — perenderan tabel kontak (contactsTable, contactsGlobalTable,
// contactsTableCard, contactRow, contactBadges), dipisah dari contacts.go agar
// tiap file di bawah ambang tipe View/Component (300). Shell daftar/tab/pager
// tetap di contacts.go — satu paket.

// contactsTable = tabel kontak satu desa (showVillage=false → tanpa kolom Desa).
// Dibungkus ui.TableScroll (scroll terkurung; tak mendorong lebar halaman di
// mobile). Baris tertaut ke detail.
func contactsTable(accountBase string, items []ContactRow, showVillage bool) g.Node {
	rows := make([]g.Node, 0, len(items))
	for _, c := range items {
		rows = append(rows, contactRow(accountBase, c, showVillage))
	}
	return contactsTableCard(showVillage, rows)
}

// contactsGlobalTable = tabel kontak lintas-desa (dengan kolom Desa). Tiap baris
// menautkan ke desa induknya masing-masing (base per-baris dari AccountID).
func contactsGlobalTable(wsBase string, items []ContactRow) g.Node {
	rows := make([]g.Node, 0, len(items))
	for _, c := range items {
		accountBase := wsBase + "/accounts/" + strconv.FormatInt(c.AccountID, 10)
		rows = append(rows, contactRow(accountBase, c, true))
	}
	return contactsTableCard(true, rows)
}

// contactsTableCard membungkus header + baris dalam kartu ber-scroll. Kolom Desa
// (showVillage) hanya di daftar global; "Terakhir" = ringkasan aktivitas (DITUNDA
// modul Activities, kini "—").
func contactsTableCard(showVillage bool, rows []g.Node) g.Node {
	headers := []g.Node{h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Nama"))}
	headers = append(headers, h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Peran")))
	if showVillage {
		headers = append(headers, h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Desa")))
	}
	headers = append(headers,
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("HP / WhatsApp")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Penanda")),
		h.Th(h.Class("py-2 font-medium"), g.Text("Terakhir")),
	)
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					append([]g.Node{h.Class("border-b border-base-300 text-left text-base-content/70")},
						headers...)...,
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

// contactRow = satu baris kontak. Kolom Nama membawa kategori jabatan sebagai
// sub-teks (menggantikan kolom Jabatan tersendiri, wireframe M3). Nomor HP/WhatsApp
// SUDAH disamarkan handler (F4). showVillage menyisipkan kolom Desa (daftar global).
func contactRow(accountBase string, c ContactRow, showVillage bool) g.Node {
	href := accountBase + "/contacts/" + strconv.FormatInt(c.ID, 10)
	link := func(text, cls string) g.Node {
		return h.Td(h.Class(cls), h.A(h.Href(href), h.Class("block truncate"), g.Text(text)))
	}
	cells := []g.Node{
		h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), h.Class("block min-w-0"),
			h.Div(h.Class("truncate font-medium"), g.Text(c.Name)),
			ui.When(c.PositionCategory != "", h.Div(
				h.Class("truncate text-xs text-base-content/60"),
				g.Text(c.PositionCategory))),
		)),
		link(orDash(c.ContactRole), "py-2 pr-4"),
	}
	if showVillage {
		cells = append(cells, link(orDash(c.Village), "py-2 pr-4"))
	}
	cells = append(cells,
		link(orDash(c.Phone), "py-2 pr-4"),
		h.Td(h.Class("py-2 pr-4"), contactBadges(c)),
		link(orDash(c.LastActivity), "py-2 text-base-content/60"),
	)
	return h.Tr(cells...)
}

// contactBadges merender penanda status kontak. Primary & opt-out/do-not-contact
// dibuat MENONJOL (badge berwarna) — "opt-out tampil jelas" adalah kriteria modul,
// bukan detail yang boleh terkubur.
func contactBadges(c ContactRow) g.Node {
	badges := []g.Node{h.Class("flex flex-wrap gap-1")}
	if c.IsPrimary {
		badges = append(badges, h.Span(h.Class("badge badge-primary badge-sm"), g.Text("Utama")))
	}
	if c.IsTechnical {
		badges = append(badges, h.Span(h.Class("badge badge-ghost badge-sm"), g.Text("Teknis")))
	}
	if c.EmailOptOut {
		badges = append(badges, h.Span(h.Class("badge badge-warning badge-sm"), g.Text("Opt-out email")))
	}
	if c.DoNotContact {
		badges = append(badges, h.Span(h.Class("badge badge-error badge-sm"), g.Text("Jangan hubungi")))
	}
	if len(badges) == 1 { // hanya kelas, tak ada badge
		return h.Span(h.Class("text-base-content/40"), g.Text("—"))
	}
	return h.Div(badges...)
}
