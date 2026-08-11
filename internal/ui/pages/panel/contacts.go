package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// contacts.go — view modul Kontak (orang di dalam sebuah desa): daftar per-desa,
// daftar global, penolakan. Murni-data: setiap nilai (termasuk yang disamarkan
// F4) SUDAH diputuskan handler; view tak memanggil authz/session. Penanda primary/
// opt-out dirender sebagai badge agar terbaca sekilas (kriteria M3: "opt-out
// tampil jelas").

// ContactRow = satu baris daftar kontak. Nomor TIDAK di sini (PII; hanya di
// detail). AccountID untuk menautkan ke desa induk saat di daftar global.
type ContactRow struct {
	ID               int64
	AccountID        int64
	Name             string
	JobTitle         string
	PositionCategory string
	ContactRole      string
	IsPrimary        bool
	IsTechnical      bool
	EmailOptOut      bool
	DoNotContact     bool
}

// ContactsListView = data daftar kontak SATU desa. AccountBase = URL desa induk
// (aksi & tautan kembali). CanWrite = tombol "Tambah Kontak" tampil.
type ContactsListView struct {
	Base        string
	AccountBase string
	AccountName string
	Items       []ContactRow
	CanWrite    bool
	NextCursor  string
	Err         string
	Msg         string
}

// ContactsAllView = data daftar kontak LINTAS-desa. Baris menautkan ke detail
// kontak di bawah desa induknya (butuh AccountID per baris).
type ContactsAllView struct {
	Base       string
	Items      []ContactRow
	NextCursor string
	Err        string
	Msg        string
}

// ContactsList merender daftar kontak satu desa: header + aksi tambah, alert,
// tabel keyset, pager.
func ContactsList(v ContactsListView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.Class("min-w-0"),
				h.H1(h.Class("text-xl font-semibold truncate"), g.Text("Kontak")),
				h.P(h.Class("text-base-content/70 truncate"),
					g.Text("Perangkat desa & narahubung di "+v.AccountName+".")),
			),
			ui.When(v.CanWrite, h.A(
				h.Href(v.AccountBase+"/contacts/new"), h.Class("btn btn-primary min-h-11"),
				g.Text("Tambah Kontak"),
			)),
		),
		h.A(h.Href(v.AccountBase), h.Class("text-sm text-base-content/60"),
			g.Text("« Kembali ke desa")),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "contacts-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "contacts-ok", g.Text(v.Msg)))
	}
	if len(v.Items) == 0 {
		body = append(body, emptyContacts(v.AccountBase, v.NextCursor,
			"Belum ada kontak untuk desa ini."))
	} else {
		body = append(body, contactsTable(v.AccountBase, v.Items, true))
		body = append(body, contactsPager(v.AccountBase+"/contacts", v.NextCursor))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// ContactsAll merender daftar kontak lintas-desa (global). Tiap baris menautkan
// ke detail di bawah desa induknya.
func ContactsAll(v ContactsAllView) g.Node {
	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text("Kontak")),
			h.P(h.Class("text-base-content/70"),
				g.Text("Semua kontak di desa yang Anda kelola.")),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "contacts-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "contacts-ok", g.Text(v.Msg)))
	}
	if len(v.Items) == 0 {
		body = append(body, emptyContacts(v.Base+"/contacts", v.NextCursor,
			"Belum ada kontak yang cocok."))
	} else {
		// base per-baris = URL desa induk masing-masing (dirakit dari AccountID).
		body = append(body, contactsGlobalTable(v.Base, v.Items))
		body = append(body, contactsPager(v.Base+"/contacts", v.NextCursor))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// emptyContacts = pesan kosong jujur. backHref menawarkan jalan kembali bila ini
// bisa jadi halaman-setelah-cursor yang kebetulan habis.
func emptyContacts(backHref, nextCursor, msg string) g.Node {
	if nextCursor == "" {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body items-start"),
				h.P(h.Class("text-base-content/70"), g.Text(msg))),
		)
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body items-start"),
			h.P(h.Class("text-base-content/70"),
				g.Text("Belum ada kontak yang cocok di halaman ini.")),
			h.A(h.Href(backHref), h.Class("btn btn-ghost btn-sm min-h-11"),
				g.Text("« Kembali ke awal")),
		),
	)
}

// contactsTable = tabel kontak satu desa. Dibungkus ui.TableScroll (scroll
// terkurung; tak mendorong lebar halaman di mobile). Baris tertaut ke detail.
func contactsTable(accountBase string, items []ContactRow, _ bool) g.Node {
	rows := make([]g.Node, 0, len(items))
	for _, c := range items {
		rows = append(rows, contactRow(accountBase, c))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Nama")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Jabatan")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Peran")),
					h.Th(h.Class("py-2 font-medium"), g.Text("Penanda")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

// contactsGlobalTable = tabel kontak lintas-desa. Kolom sama, tapi tiap baris
// menautkan ke desa induknya masing-masing (base per-baris dari AccountID).
func contactsGlobalTable(wsBase string, items []ContactRow) g.Node {
	rows := make([]g.Node, 0, len(items))
	for _, c := range items {
		accountBase := wsBase + "/accounts/" + strconv.FormatInt(c.AccountID, 10)
		rows = append(rows, contactRow(accountBase, c))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Nama")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Jabatan")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Peran")),
					h.Th(h.Class("py-2 font-medium"), g.Text("Penanda")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func contactRow(accountBase string, c ContactRow) g.Node {
	href := accountBase + "/contacts/" + strconv.FormatInt(c.ID, 10)
	link := func(text, cls string) g.Node {
		return h.Td(h.Class(cls), h.A(h.Href(href), h.Class("block truncate"), g.Text(text)))
	}
	return h.Tr(
		h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), h.Class("block min-w-0"),
			h.Div(h.Class("truncate font-medium"), g.Text(c.Name)),
			ui.When(c.PositionCategory != "", h.Div(
				h.Class("truncate text-xs text-base-content/60"),
				g.Text(c.PositionCategory))),
		)),
		link(orDash(c.JobTitle), "py-2 pr-4"),
		link(orDash(c.ContactRole), "py-2 pr-4"),
		h.Td(h.Class("py-2"), contactBadges(c)),
	)
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

// contactsPager = jalan ke halaman berikutnya (keyset). Link biasa (navigasi:
// bookmarkable + dimuat ulang, lolos gotcha #16), tap target 44px, flex-wrap 375px.
func contactsPager(listHref, nextCursor string) g.Node {
	if nextCursor == "" {
		return h.Div(h.Class("flex flex-wrap items-center gap-2"),
			h.Span(h.Class("text-sm text-base-content/60"), g.Text("Ujung daftar.")))
	}
	return h.Div(
		h.Class("flex flex-wrap items-center gap-2"),
		h.A(
			h.Href(listHref+"?after="+nextCursor),
			h.Class("btn min-h-11"), g.Text("Berikutnya »"),
		),
	)
}

// ContactsForbidden = penolakan 403 bagi anggota tanpa peran CRM (kembaran
// AccountsForbidden). Menyebut SIAPA yang bisa membantu, bukan sekadar "ditolak".
func ContactsForbidden() g.Node {
	return h.Div(
		h.Class("grid gap-4 min-w-0 max-w-xl mx-auto py-8"),
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Kontak")),
		h.P(h.Class("text-base-content/70"),
			g.Text("Modul Kontak hanya bisa dibuka oleh pemegang peran CRM "+
				"(admin, manajer, sales, atau CSM).")),
		h.P(h.Class("text-sm text-base-content/60"),
			g.Text("Hubungi admin workspace bila Anda perlu akses ke data kontak.")),
	)
}
