package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// contacts.go — view modul Kontak (orang di dalam sebuah desa): daftar per-desa,
// daftar global, penolakan. Murni-data: setiap nilai (termasuk yang disamarkan
// F4) SUDAH diputuskan handler; view tak memanggil authz/session. Penanda primary/
// opt-out dirender sebagai badge agar terbaca sekilas (kriteria M3: "opt-out
// tampil jelas").

// ContactRow = satu baris daftar kontak. WhatsApp IKUT (kolom daftar, wireframe M3)
// tapi SUDAH disamarkan handler (F4) bila penglihatnya bukan Sales — nomor asli tak
// pernah dioper ke view. Village = nama desa induk (hanya terisi di daftar global,
// dari JOIN accounts). LastActivity = ringkasan "terakhir dihubungi" — DITUNDA
// sampai modul Activities; handler mengisi "" → dirender "—". AccountID untuk
// menautkan ke desa induk saat di daftar global.
type ContactRow struct {
	ID               int64
	AccountID        int64
	Name             string
	Village          string
	PositionCategory string
	ContactRole      string
	Whatsapp         string
	LastActivity     string
	IsPrimary        bool
	IsTechnical      bool
	EmailOptOut      bool
	DoNotContact     bool
}

// Konstanta tab daftar kontak global — sumber tunggal nama view, dipakai handler
// (normalisasi ?view=) & view (tautan tab + pager). all = default (semua kontak di
// desa yang dikelola). Kembaran AccView* di accounts, tanpa "unowned" (kontak tak
// punya kolom owner sendiri — kepemilikan diwarisi desa).
const (
	ContactViewAll = "all"
	ContactViewMy  = "my"
)

// ContactsListView = data daftar kontak SATU desa. AccountBase = URL desa induk
// (aksi & tautan kembali). CanWrite = tombol "Tambah Kontak" tampil.
type ContactsListView struct {
	Base        string
	AccountBase string
	AccountName string
	Items       []ContactRow
	CanWrite    bool
	NextCursor  string
	After       string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail       string // BL-7: jejak cursor halaman sebelumnya (?trail=)
	Err         string
	Msg         string
}

// ContactsAllView = data daftar kontak LINTAS-desa. Baris menautkan ke detail
// kontak di bawah desa induknya (butuh AccountID per baris).
//
// ShowTabs = tampilkan bilah tab Semua/Kontak Saya — HANYA untuk peran ber-cakupan
// 'all' (Manager/Admin); handler yang memutuskan. Untuk peran 'own' (Sales/CSM)
// Semua≡Saya (mereka cuma lihat kontak di desanya), jadi tab redundan & disembunyikan.
// ActiveView = tab aktif (ContactView*), menentukan sorotan & param yang diteruskan pager.
type ContactsAllView struct {
	Base       string
	Items      []ContactRow
	ShowTabs   bool
	ActiveView string
	Query      string // ?q= pencarian bebas (BL-6, hanya daftar global); "" = tak mencari
	NextCursor string
	After      string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail      string // BL-7: jejak cursor halaman sebelumnya (?trail=)
	// CanWrite = tombol "Tambah Kontak" tampil. Handler menyalakannya HANYA bila
	// aktor boleh menulis DAN punya ≥1 desa dalam cakupannya (ada induk yang bisa
	// dipilih); tanpa desa, tombol menuju form kosong tanpa pilihan → disembunyikan.
	CanWrite bool
	Err      string
	Msg      string
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
		// Daftar per-desa: tanpa kolom Desa (redundan — sudah di judul halaman).
		body = append(body, contactsTable(v.AccountBase, v.Items, false))
		body = append(body, contactsPager(v.AccountBase+"/contacts", "", v.NextCursor, "", v.After, v.Trail))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// ContactsAll merender daftar kontak lintas-desa (global). Tiap baris menautkan
// ke detail di bawah desa induknya.
func ContactsAll(v ContactsAllView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.Class("min-w-0"),
				h.H1(h.Class("text-xl font-semibold truncate"), g.Text("Kontak")),
				h.P(h.Class("text-base-content/70 truncate"),
					g.Text("Semua kontak di desa yang Anda kelola.")),
			),
			ui.When(v.CanWrite, h.A(
				h.Href(v.Base+"/contacts/new"), h.Class("btn btn-primary min-h-11"),
				g.Text("Tambah Kontak"),
			)),
		),
	}
	if v.ShowTabs {
		body = append(body, contactsTabs(v))
	}
	// Search hanya di daftar GLOBAL (per-desa gerbangnya desa induk). view aktif
	// dipertahankan agar tab & pencarian tak saling menghapus.
	viewKeep := ""
	if v.ActiveView != "" && v.ActiveView != ContactViewAll {
		viewKeep = v.ActiveView
	}
	body = append(body, searchBox(v.Base+"/contacts", v.Query,
		"Cari kontak — nama atau desa…", "Cari kontak", hiddenField{"view", viewKeep}))
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "contacts-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "contacts-ok", g.Text(v.Msg)))
	}
	if len(v.Items) == 0 {
		body = append(body, emptyContacts(contactsListHref(v.Base, v.ActiveView, v.Query),
			v.NextCursor, "Belum ada kontak yang cocok."))
	} else {
		// base per-baris = URL desa induk masing-masing (dirakit dari AccountID).
		body = append(body, contactsGlobalTable(v.Base, v.Items))
		body = append(body, contactsPager(v.Base+"/contacts", v.ActiveView, v.NextCursor, v.Query, v.After, v.Trail))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// contactsTabs = bilah tab cakupan (Semua/Kontak Saya) untuk peran 'all'. Navigasi
// tautan <a> biasa (bookmarkable + reload penuh, lolos CSP gotcha #16), BUKAN
// Datastar. flex-wrap agar tak mendorong lebar di 375px; tiap tab min-h-11 (tap
// target 44px). Kembaran accountsTabs (varian tabs-box, lihat gotcha #4).
func contactsTabs(v ContactsAllView) g.Node {
	tab := func(label, view string) g.Node {
		cls := "tab min-h-11"
		if v.ActiveView == view {
			cls += " tab-active"
		}
		return h.A(h.Href(contactsListHref(v.Base, view, v.Query)), h.Class(cls), g.Text(label))
	}
	return h.Div(
		h.Role("tablist"),
		h.Class("tabs tabs-box flex-wrap"),
		tab("Semua Kontak", ContactViewAll),
		tab("Kontak Saya", ContactViewMy),
	)
}

// contactsListHref merakit URL daftar global untuk sebuah view + q opsional.
// all/"" = tanpa param view (URL kanonik); q kosong = tanpa param q. Urutan &
// ditentukan url.Values.Encode (withQuery) — konsisten & ter-escape.
func contactsListHref(base, view, query string) string {
	viewKeep := ""
	if view != "" && view != ContactViewAll {
		viewKeep = view
	}
	return withQuery(base+"/contacts", query, hiddenField{"view", viewKeep})
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

// contactsPager = jalan ke halaman berikutnya (keyset). Link biasa (navigasi:
// bookmarkable + dimuat ulang, lolos gotcha #16), tap target 44px, flex-wrap 375px.
// view (opsional, "" di daftar per-desa) diteruskan agar tab aktif bertahan antar
// halaman: ?after= lebih dulu, lalu &view= (kembaran accountsPager).
func contactsPager(listHref, view, nextCursor, query, after, trail string) g.Node {
	viewKeep := ""
	if view != "" && view != ContactViewAll {
		viewKeep = view
	}
	base := panelListHref(listHref, [2]string{"view", viewKeep}, [2]string{"q", query})
	return ui.KeysetPager(base, after, trail, nextCursor)
}

// ContactsForbidden = penolakan 403 bagi anggota tanpa peran CRM (kembaran
// AccountsForbidden). Menyebut SIAPA yang bisa membantu, bukan sekadar "ditolak".
func ContactsForbidden() g.Node {
	return h.Div(
		h.Class("grid gap-4 min-w-0 max-w-xl mx-auto py-8"),
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Kontak")),
		h.P(h.Class("text-base-content/70"),
			g.Text("Modul Kontak hanya bisa dibuka oleh pemegang peran CRM "+
				"(admin, manajer, sales, atau CS).")),
		h.P(h.Class("text-sm text-base-content/60"),
			g.Text("Hubungi admin workspace bila Anda perlu akses ke data kontak.")),
	)
}
