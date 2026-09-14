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
// Header POLOS (tanpa sort — daftar per-desa selalu default created_at DESC,
// BL-157d cuma menyentuh daftar global). Dibungkus ui.TableScroll (scroll
// terkurung; tak mendorong lebar halaman di mobile). Baris tertaut ke detail.
func contactsTable(accountBase string, items []ContactRow, showVillage bool) g.Node {
	rows := make([]g.Node, 0, len(items))
	for _, c := range items {
		rows = append(rows, contactRow(accountBase, c, showVillage))
	}
	return contactsTableCard(contactPlainHeaders(showVillage), rows)
}

// contactsGlobalTable = tabel kontak lintas-desa (dengan kolom Desa). Tiap baris
// menautkan ke desa induknya masing-masing (base per-baris dari AccountID).
// Header Kode/Nama/Peran/Desa BISA di-sort (BL-157d, klon accountsGlobalTable);
// HP/WhatsApp (F4, side-channel risk bila sortable), Penanda (4 boolean lepas,
// tak ada kunci sort tunggal wajar), Terakhir (placeholder DITUNDA, selalu "—")
// SENGAJA tetap polos.
func contactsGlobalTable(v ContactsAllView) g.Node {
	rows := make([]g.Node, 0, len(v.Items))
	for _, c := range v.Items {
		accountBase := v.Base + "/accounts/" + strconv.FormatInt(c.AccountID, 10)
		rows = append(rows, contactRow(accountBase, c, true))
	}
	headers := append(contactSortableHeaders(v), contactTrailingHeaders()...)
	return contactsTableCard(headers, rows)
}

// contactPlainHeaders = header tanpa sort (daftar per-desa). showVillage
// (selalu false di sini, dipertahankan agar simetris dgn contactsTable) menambah
// kolom Desa.
func contactPlainHeaders(showVillage bool) []g.Node {
	headers := []g.Node{
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Kode")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Nama")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Peran")),
	}
	if showVillage {
		headers = append(headers, h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Desa")))
	}
	return append(headers, contactTrailingHeaders()...)
}

// contactSortableHeaders = 4 kolom yang bisa di-sort (BL-157d): Kode, Nama,
// Peran, Desa — hanya dipakai daftar global (contactsGlobalTable).
func contactSortableHeaders(v ContactsAllView) []g.Node {
	return []g.Node{
		h.Th(h.Class("py-2 pr-4 font-medium"), contactSortHeader(v, "code", "Kode")),
		h.Th(h.Class("py-2 pr-4 font-medium"), contactSortHeader(v, "name", "Nama")),
		h.Th(h.Class("py-2 pr-4 font-medium"), contactSortHeader(v, "role", "Peran")),
		h.Th(h.Class("py-2 pr-4 font-medium"), contactSortHeader(v, "village", "Desa")),
	}
}

// contactTrailingHeaders = 3 kolom yang SENGAJA tetap polos (lihat rasional di
// contactsGlobalTable): HP/WhatsApp, Penanda, Terakhir.
func contactTrailingHeaders() []g.Node {
	return []g.Node{
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("HP / WhatsApp")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Penanda")),
		h.Th(h.Class("py-2 font-medium"), g.Text("Terakhir")),
	}
}

// contactSortHeader = header kolom jadi tautan sort (BL-157d, klon persis
// leadSortHeader/accSortHeader). Native <a href> (bookmarkable, lolos gotcha
// #16), BUKAN Datastar. Klik saat non-aktif → sort=col&dir=asc; klik saat aktif
// → toggle arah. Tautan sort SENDIRI tak membawa after/trail (submit baru reset
// ke hal 1). view/q dipertahankan.
func contactSortHeader(v ContactsAllView, col, label string) g.Node {
	active := v.Sort == col
	nextDir := "asc"
	if active && v.Dir == "asc" {
		nextDir = "desc"
	}
	viewKeep := ""
	if v.ActiveView != "" && v.ActiveView != ContactViewAll {
		viewKeep = v.ActiveView
	}
	href := withQuery(v.Base+"/contacts", v.Query,
		hiddenField{"view", viewKeep}, hiddenField{"sort", col}, hiddenField{"dir", nextDir})
	text := label
	if active {
		arrow := "▲"
		if v.Dir == "desc" {
			arrow = "▼"
		}
		text = label + " " + arrow
	}
	return h.A(h.Href(href), h.Class("hover:underline"), g.Text(text))
}

// contactsTableCard membungkus header + baris dalam kartu ber-scroll.
func contactsTableCard(headers []g.Node, rows []g.Node) g.Node {
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
		link(orDash(c.EntityCode), "py-2 pr-4 font-mono text-xs"),
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
