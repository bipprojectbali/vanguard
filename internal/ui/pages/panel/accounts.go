package panel

import (
	"net/url"
	"strconv"
	"strings"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// accounts.go — view hub Desa (Account): daftar, detail, penolakan. Murni-data:
// setiap nilai (termasuk yang disamarkan F4) SUDAH diputuskan handler; view tak
// memanggil authz/session. String kosong = "tak ada / tak berhak" — dirender apa
// adanya, bukan diisi placeholder yang menyesatkan.

// AccountRow = satu baris daftar desa. Nomor HP TIDAK di sini (PII; hanya di
// detail). AccountType sudah berupa label Indonesia (accountTypeLabel di handler).
// OwnerName/CSMName = nama anggota (bukan id) sudah diresolusi handler; "" =
// belum ditugaskan → dirender "—".
type AccountRow struct {
	ID          int64
	EntityCode  string
	VillageName string
	VillageCode string
	AccountType string
	Regency     string
	Province    string
	OwnerName   string
	CSMName     string
}

// Konstanta tab daftar desa — sumber tunggal nama view, dipakai handler
// (normalisasi ?view=) & view (tautan tab + pager). all = default (semua desa).
const (
	AccViewAll     = "all"
	AccViewMy      = "my"
	AccViewUnowned = "unowned"
)

// AccountsListView = data halaman daftar. CanWrite = tombol "Tambah Desa" tampil
// (F2 write & workspace tak read-only, diputuskan handler). NextCursor "" =
// halaman terakhir. Err/Msg = alert galat/sukses (dari ?err=/?ok=).
//
// ShowTabs = tampilkan bilah tab All/My/Belum-ada-Owner — HANYA untuk peran
// ber-cakupan 'all' (Manager/Admin); handler yang memutuskan. Untuk peran 'own'
// (Sales/CSM) All≡My (mereka cuma lihat desanya), jadi tab redundan & disembunyikan.
// ActiveView = tab aktif (AccView*), menentukan sorotan & param yang diteruskan pager.
//
// Query = kata kunci pencarian aktif (BL-6). "" = tanpa pencarian. Diteruskan
// ke kotak cari (mengisi ulang input), pager, dan tautan tab agar pencarian
// bertahan saat pindah halaman/tab. Handler yang men-TrimSpace & menyaring.
type AccountsListView struct {
	Base       string
	Items      []AccountRow
	CanWrite   bool
	ShowTabs   bool
	ActiveView string
	Query      string
	NextCursor string
	Err        string
	Msg        string
}

// AccountsList merender daftar desa: header + aksi tambah, alert, tabel keyset.
func AccountsList(v AccountsListView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Desa")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Direktori desa & kelurahan yang Anda kelola.")),
			),
			ui.When(v.CanWrite, h.A(
				h.Href(v.Base+"/accounts/new"), h.Class("btn btn-primary min-h-11"),
				g.Text("Tambah Desa"),
			)),
		),
	}
	if v.ShowTabs {
		body = append(body, accountsTabs(v))
	}
	body = append(body, accountsSearch(v))
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "accounts-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "accounts-ok", g.Text(v.Msg)))
	}
	if len(v.Items) == 0 {
		body = append(body, emptyAccounts(v))
	} else {
		body = append(body, accountsTable(v))
		body = append(body, accountsPager(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// accountsTabs = bilah tab cakupan (All/My/Belum-ada-Owner) untuk peran 'all',
// plus satu baris penjelasan di bawahnya yang MENYESUAIKAN tab aktif (bukan
// satu kalimat statis merangkum ketiganya) — arti tab yang sedang dilihat
// lebih relevan daripada penjelasan tab lain yang tak aktif; lihat
// accountsTabDesc. Navigasi tautan <a> biasa (bookmarkable + reload penuh,
// lolos CSP gotcha #16), BUKAN Datastar. flex-wrap agar tak mendorong lebar
// di 375px; tiap tab min-h-11 (tap target 44px). role="tablist" seperti
// bilah tab panel lain (a11y).
//
// Varian tabs-box (BUKAN tabs-boxed — nama daisyUI v4 yang sudah tak ada di v5,
// jadi tree-shaken → tak bergaya; gotcha #4): satu-satunya varian yang ikut
// terkompilasi ke app.css. tabs-box sudah memberi permukaan berkotak sendiri,
// jadi tak perlu bg/border manual.
func accountsTabs(v AccountsListView) g.Node {
	tab := func(label, view string) g.Node {
		cls := "tab min-h-11"
		if v.ActiveView == view {
			cls += " tab-active"
		}
		return h.A(h.Href(accountsListHref(v.Base, view, v.Query)), h.Class(cls), g.Text(label))
	}
	return g.Group([]g.Node{
		h.Div(
			h.Role("tablist"),
			h.Class("tabs tabs-box flex-wrap"),
			tab("Semua Desa", AccViewAll),
			tab("Desa Saya", AccViewMy),
			tab("Belum ada Owner", AccViewUnowned),
		),
		h.P(h.Class("text-xs text-base-content/60"), g.Text(accountsTabDesc(v.ActiveView))),
	})
}

// accountsTabDesc = penjelasan satu tab AKTIF (bukan ketiganya sekaligus).
// Default (AccViewAll & nilai tak dikenal) sengaja sama — normalizeAccountsView
// sudah menjatuhkan nilai liar ke AccViewAll sebelum sampai sini.
func accountsTabDesc(view string) string {
	switch view {
	case AccViewMy:
		return "Desa yang Anda kelola sebagai Owner atau CS (utama/cadangan)."
	case AccViewUnowned:
		return "Desa yang belum punya penanggung jawab (Owner)."
	default: // AccViewAll
		return "Seluruh desa di workspace ini."
	}
}

// accountsListHref merakit URL daftar untuk sebuah view + pencarian. all/"" =
// tanpa param view (URL kanonik daftar). q di-QueryEscape (bisa berisi spasi/
// karakter khusus); view aman (enum internal). Param after SENGAJA tak ikut:
// tautan tab/reset selalu mulai dari halaman pertama.
func accountsListHref(base, view, query string) string {
	parts := make([]string, 0, 2)
	if view != "" && view != AccViewAll {
		parts = append(parts, "view="+view)
	}
	if query != "" {
		parts = append(parts, "q="+url.QueryEscape(query))
	}
	if len(parts) == 0 {
		return base + "/accounts"
	}
	return base + "/accounts?" + strings.Join(parts, "&")
}

// accountsSearch = kotak pencarian (BL-6). Form GET murni (navigasi bookmarkable,
// lolos CSP gotcha #16) — submit membuang `after` sehingga hasil selalu mulai
// dari halaman pertama (reset paging saat kueri berubah). Tab aktif dijaga lewat
// input tersembunyi. Mobile-first: input text-base (≥16px → iOS tak auto-zoom),
// min-h-11 (tap ≥44px), flex-wrap agar tak meluber di 375px.
func accountsSearch(v AccountsListView) g.Node {
	fields := []g.Node{
		h.Input(
			h.Type("search"), h.Name("q"), h.Value(v.Query),
			h.Placeholder("Cari desa — nama atau kode…"),
			h.Class("input input-bordered text-base w-full sm:max-w-xs min-h-11 min-w-0"),
			g.Attr("aria-label", "Cari desa"),
		),
	}
	if v.ActiveView != "" && v.ActiveView != AccViewAll {
		fields = append(fields, h.Input(h.Type("hidden"), h.Name("view"), h.Value(v.ActiveView)))
	}
	fields = append(fields, h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text("Cari")))
	if v.Query != "" {
		fields = append(fields, h.A(
			h.Href(accountsListHref(v.Base, v.ActiveView, "")),
			h.Class("btn btn-ghost min-h-11"), g.Text("Reset")))
	}
	return h.Form(
		h.Method("get"), h.Action(v.Base+"/accounts"),
		h.Class("flex flex-wrap items-center gap-2 min-w-0"),
		g.Group(fields),
	)
}

// emptyAccounts = pesan kosong jujur. Halaman pertama benar-benar kosong vs
// halaman kedua yang kosong (setelah cursor) dibedakan: yang kedua menawarkan
// jalan kembali alih-alih "belum ada desa" yang berbohong.
func emptyAccounts(v AccountsListView) g.Node {
	if v.Query != "" {
		// Pencarian tak berhasil: bukan "belum ada desa" (yang berbohong), tapi
		// "tak ada yang cocok" + jalan keluar menghapus pencarian.
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body items-start"),
				h.P(h.Class("text-base-content/70"),
					g.Text("Tak ada desa yang cocok dengan pencarian Anda.")),
				h.A(h.Href(accountsListHref(v.Base, v.ActiveView, "")), h.Class("btn btn-ghost btn-sm min-h-11"),
					g.Text("« Hapus pencarian")),
			),
		)
	}
	if v.NextCursor == "" {
		// Bisa halaman-setelah-cursor yang kebetulan habis: tawarkan kembali.
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body items-start"),
				h.P(h.Class("text-base-content/70"),
					g.Text("Belum ada desa yang cocok. Tambah desa untuk memulai.")),
				h.A(h.Href(accountsListHref(v.Base, v.ActiveView, "")), h.Class("btn btn-ghost btn-sm min-h-11"),
					g.Text("« Kembali ke awal")),
			),
		)
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body"),
			h.P(h.Class("text-base-content/70"), g.Text("Belum ada desa."))),
	)
}

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
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Kode")),
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
		link(orDash(a.EntityCode), "py-2 pr-4 font-mono text-xs"),
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
	if v.NextCursor == "" {
		return h.Div(h.Class("flex flex-wrap items-center gap-2"),
			h.Span(h.Class("text-sm text-base-content/60"), g.Text("Ujung daftar.")))
	}
	href := v.Base + "/accounts?after=" + v.NextCursor
	if v.ActiveView != "" && v.ActiveView != AccViewAll {
		href += "&view=" + v.ActiveView
	}
	if v.Query != "" {
		href += "&q=" + url.QueryEscape(v.Query)
	}
	return h.Div(
		h.Class("flex flex-wrap items-center gap-2"),
		h.A(h.Href(href), h.Class("btn min-h-11"), g.Text("Berikutnya »")),
	)
}

// AccountsForbidden = penolakan 403 bagi anggota tanpa peran CRM. Menyebut SIAPA
// yang bisa membantu, bukan sekadar "akses ditolak": penolakan telanjang membuat
// orang mengira ada yang rusak alih-alih memahaminya sebagai batas peran.
func AccountsForbidden() g.Node {
	return h.Div(
		h.Class("grid gap-4 min-w-0 max-w-xl mx-auto py-8"),
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Desa")),
		h.P(h.Class("text-base-content/70"),
			g.Text("Modul Desa hanya bisa dibuka oleh pemegang peran CRM "+
				"(admin, manajer, sales, atau CS).")),
		h.P(h.Class("text-sm text-base-content/60"),
			g.Text("Hubungi admin workspace bila Anda perlu akses ke data desa.")),
	)
}

// orDash menampilkan "—" untuk nilai kosong di sel tabel: sel hampa terbaca
// seperti kolom yang rusak, tanda pisah menyatakan "memang belum diisi".
func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
