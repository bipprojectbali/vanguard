package panel

import (
	"strconv"

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
type AccountRow struct {
	ID          int64
	EntityCode  string
	VillageName string
	VillageCode string
	AccountType string
	Regency     string
	Province    string
}

// AccountsListView = data halaman daftar. CanWrite = tombol "Tambah Desa" tampil
// (F2 write & workspace tak read-only, diputuskan handler). NextCursor "" =
// halaman terakhir. Err/Msg = alert galat/sukses (dari ?err=/?ok=).
type AccountsListView struct {
	Base       string
	Items      []AccountRow
	CanWrite   bool
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

// emptyAccounts = pesan kosong jujur. Halaman pertama benar-benar kosong vs
// halaman kedua yang kosong (setelah cursor) dibedakan: yang kedua menawarkan
// jalan kembali alih-alih "belum ada desa" yang berbohong.
func emptyAccounts(v AccountsListView) g.Node {
	if v.NextCursor == "" {
		// Bisa halaman-setelah-cursor yang kebetulan habis: tawarkan kembali.
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body items-start"),
				h.P(h.Class("text-base-content/70"),
					g.Text("Belum ada desa yang cocok. Tambah desa untuk memulai.")),
				h.A(h.Href(v.Base+"/accounts"), h.Class("btn btn-ghost btn-sm min-h-11"),
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
					h.Th(h.Class("py-2 font-medium"), g.Text("Provinsi")),
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
		link(orDash(a.Province), "py-2"),
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
	return h.Div(
		h.Class("flex flex-wrap items-center gap-2"),
		h.A(
			h.Href(v.Base+"/accounts?after="+v.NextCursor),
			h.Class("btn min-h-11"), g.Text("Berikutnya »"),
		),
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
				"(admin, manajer, sales, atau CSM).")),
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
