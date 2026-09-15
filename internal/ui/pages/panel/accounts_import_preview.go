package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// accounts_import_preview.go — BL-63: tabel pratinjau hasil dry-run parse+
// resolve, satu baris per baris CSV, status ✅/❌ per baris (wsErrMsg sudah
// diterjemahkan di handler — view murni-data). Dirender DI BAWAH form unggah
// (AccountImportUploadCard, accounts_import.go) pada HALAMAN YANG SAMA — POST
// /accounts/import tak lagi menavigasi ke halaman terpisah, operator melihat
// form + hasil sekaligus & bisa langsung unggah ulang. Tombol konfirmasi
// mengirim ULANG csv mentah via hidden field (raw_csv); AccountImportConfirm
// memvalidasi ulang PENUH sebelum menulis (kebijakan all-or-nothing BL-63).

// AccountImportColumn = satu kolom tabel pratinjau. Key merujuk kunci di
// AccountImportPreviewRow.Values — SAMA dgn nama header CSV (importCSVHeader,
// accounts_import.go), Label = teks header manusiawi. Kolom pratinjau SENGAJA
// disamakan dgn kolom CSV (bukan subset "Kode Desa/Desa" saja) agar operator
// bisa memverifikasi data mentah tiap baris SEBELUM konfirmasi — termasuk
// baris tak valid yang belum sempat ter-resolve ke master data.
type AccountImportColumn struct {
	Key   string
	Label string
}

// AccountImportPreviewRow = satu baris pratinjau — Values berisi nilai CSV
// APA ADANYA (belum diresolusi) supaya tetap tampil walau baris gagal
// validasi. ErrMsg == "" → baris valid. Nomor baris file TAK ditampilkan
// (kolom "Baris" dibuang) — operator menilai baris dari isinya sendiri
// (kode_desa dkk.), bukan nomor baris mentah.
type AccountImportPreviewRow struct {
	Values map[string]string
	ErrMsg string
}

// AccountImportPreviewView = data halaman pratinjau. Upload = form unggah yang
// dirender ULANG di atas hasil (lihat komentar file) — sama field-nya dgn
// AccountImportFormView, tanpa Err (galat file sudah gagal sebelum sampai ke
// pratinjau, ditangani via redirect ?err= ke halaman kosong). ValidCount &
// InvalidCount DIHITUNG DI HANDLER (bukan diturunkan dari Rows di sini) —
// view murni-data, tak boleh menghitung ulang state yang bisa dihitung sekali
// saat handler sudah punya rr.errCode per baris.
type AccountImportPreviewView struct {
	Base         string
	Upload       AccountImportFormView
	ConfirmURL   string
	RawCSV       string
	AnyFailed    bool
	ValidCount   int
	InvalidCount int
	Columns      []AccountImportColumn
	Rows         []AccountImportPreviewRow
}

// AccountImportPreview merender: header + form unggah (SAMA komponen dgn
// AccountImportForm) di atas, lalu ringkasan + tabel status per baris + form
// konfirmasi (hidden raw_csv) di bawahnya. Saat AnyFailed, tombol konfirmasi
// DINONAKTIFKAN — kebijakan all-or-nothing berarti submit pasti DITOLAK utuh
// selama masih ada baris tak valid, jadi tombol mencegah submit sia-sia
// alih-alih sekadar memperingatkan (server tetap penjaga akhir di
// AccountImportConfirm bila state ini pernah dilewati, mis. JS dimatikan).
func AccountImportPreview(v AccountImportPreviewView) g.Node {
	rows := make([]g.Node, 0, len(v.Rows))
	for _, row := range v.Rows {
		rows = append(rows, importPreviewRow(v.Columns, row))
	}

	// Ringkasan SELALU menyebut total valid & tak valid (bukan cuma "semua
	// valid" polos) — operator langsung tahu proporsinya tanpa menghitung
	// baris/badge sendiri di tabel bawahnya.
	summary := strconv.Itoa(v.ValidCount) + " dari " + strconv.Itoa(len(v.Rows)) + " baris valid — siap diimpor."
	summaryVariant := ui.VariantSuccess
	if v.AnyFailed {
		summary = strconv.Itoa(v.ValidCount) + " valid, " + strconv.Itoa(v.InvalidCount) + " tak valid dari " +
			strconv.Itoa(len(v.Rows)) + " baris — impor akan DITOLAK SELURUHNYA (all-or-nothing). Perbaiki file lalu unggah ulang."
		summaryVariant = ui.VariantWarning
	}

	headCells := make([]g.Node, 0, len(v.Columns)+1)
	for _, col := range v.Columns {
		headCells = append(headCells, h.Th(h.Class("py-2 pr-4 font-medium whitespace-nowrap"), g.Text(col.Label)))
	}
	headCells = append(headCells, h.Th(h.Class("py-2 font-medium"), g.Text("Status")))

	confirmAttrs := []g.Node{h.Type("submit"), h.Class("btn btn-primary w-fit")}
	if v.AnyFailed {
		confirmAttrs = append(confirmAttrs, h.Disabled())
	}
	confirmAttrs = append(confirmAttrs, g.Text("Konfirmasi & Impor"))

	body := []g.Node{
		accountImportHeader(v.Base),
		AccountImportUploadCard(v.Upload),
		h.H2(h.Class("text-lg font-semibold"), g.Text("Hasil Pratinjau")),
		ui.Alert(summaryVariant, "account-import-summary", g.Text(summary)),
		// ui.Card TIDAK dipakai di sini (beda dgn kartu lain di halaman ini) —
		// helper itu tak punya min-w-0 di `.card`/`.card-body`, jadi menolak
		// menyusut di bawah lebar-min tabel 17-kolom ini & mendorong SELURUH
		// halaman ke kanan (offside) alih-alih scroll terkurung di TableScroll.
		// Pola sama persis dgn memberList (members.go) untuk tabel lebar.
		h.Div(
			h.Class("card bg-base-100 border border-base-300 min-w-0"),
			h.Div(
				h.Class("card-body min-w-0"),
				ui.TableScroll(h.Table(
					h.Class("w-full text-sm"),
					h.THead(h.Tr(h.Class("border-b border-base-300 text-left text-base-content/70"), g.Group(headCells))),
					h.TBody(g.Group(rows)),
				)),
			),
		),
		h.FormEl(
			h.Class("grid gap-2"),
			h.Method("post"), h.Action(v.ConfirmURL),
			h.Input(h.Type("hidden"), h.Name("raw_csv"), h.Value(v.RawCSV)),
			h.Button(confirmAttrs...),
		),
	}

	return h.Div(h.Class("grid gap-6 min-w-0"), g.Group(body))
}

func importPreviewRow(columns []AccountImportColumn, row AccountImportPreviewRow) g.Node {
	// whitespace-nowrap WAJIB di badge status: .badge daisyUI punya height
	// TETAP tanpa white-space:nowrap bawaan — tabel w-full (table-layout auto)
	// meremas kolom terakhir ini ke lebar sisa, jadi pesan galat yang panjang
	// membungkus 2 baris di dalam pil bertinggi tetap (bentuknya rusak). Nowrap
	// memaksa satu baris & membiarkan TableScroll (overflow-x-auto) yang
	// menyerap kelebihan lebar via scroll horizontal — sama seperti kolom
	// header (whitespace-nowrap di headCells) — bukan wrap yang membelah badge.
	status := h.Span(h.Class("badge badge-success whitespace-nowrap"), g.Text("Valid"))
	if row.ErrMsg != "" {
		status = h.Span(h.Class("badge badge-error whitespace-nowrap"), g.Text(row.ErrMsg))
	}

	cells := make([]g.Node, 0, len(columns)+1)
	for _, col := range columns {
		cells = append(cells, h.Td(h.Class("py-2 pr-4 max-w-[160px]"),
			h.Span(h.Class("block truncate"), g.Text(row.Values[col.Key]))))
	}
	cells = append(cells, h.Td(h.Class("py-2 whitespace-nowrap"), status))

	return h.Tr(h.Class("border-b border-base-300/50"), g.Group(cells))
}
