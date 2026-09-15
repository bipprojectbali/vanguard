package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// contacts_import_preview.go — BL-134: tabel pratinjau hasil dry-run parse+
// resolve kontak, satu baris per baris CSV, status ✅/❌ per baris (wsErrMsg
// sudah diterjemahkan di handler — view murni-data). Pola identik
// accounts_import_preview.go (BL-63): dirender DI BAWAH form unggah
// (ContactImportUploadCard, contacts_import.go) pada HALAMAN YANG SAMA;
// tombol konfirmasi mengirim ULANG csv mentah via hidden field (raw_csv),
// ContactImportConfirm memvalidasi ulang PENUH sebelum menulis (kebijakan
// all-or-nothing BL-134).

// AccountImportColumn (accounts_import_preview.go) DIPAKAI ULANG apa adanya
// utk kolom tabel pratinjau kontak — struct {Key, Label} generik, tak
// spesifik domain Desa; membuat tipe baru identik hanya menambah duplikasi.

// ContactImportPreviewRow = satu baris pratinjau — Values berisi nilai CSV
// APA ADANYA (belum diresolusi) supaya tetap tampil walau baris gagal
// validasi. ErrMsg == "" → baris valid.
type ContactImportPreviewRow struct {
	Values map[string]string
	ErrMsg string
}

// ContactImportPreviewView = data halaman pratinjau. Upload = form unggah
// yang dirender ULANG di atas hasil. ValidCount & InvalidCount DIHITUNG DI
// HANDLER (bukan diturunkan dari Rows di sini) — view murni-data.
type ContactImportPreviewView struct {
	Base         string
	Upload       ContactImportFormView
	ConfirmURL   string
	RawCSV       string
	AnyFailed    bool
	ValidCount   int
	InvalidCount int
	Columns      []AccountImportColumn
	Rows         []ContactImportPreviewRow
}

// ContactImportPreview merender: header + form unggah di atas, lalu
// ringkasan + tabel status per baris + form konfirmasi (hidden raw_csv) di
// bawahnya. Saat AnyFailed, tombol konfirmasi DINONAKTIFKAN — kebijakan
// all-or-nothing berarti submit pasti DITOLAK utuh selama masih ada baris
// tak valid.
func ContactImportPreview(v ContactImportPreviewView) g.Node {
	rows := make([]g.Node, 0, len(v.Rows))
	for _, row := range v.Rows {
		rows = append(rows, contactImportPreviewRow(v.Columns, row))
	}

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
		contactImportHeader(v.Base),
		ContactImportUploadCard(v.Upload),
		h.H2(h.Class("text-lg font-semibold"), g.Text("Hasil Pratinjau")),
		ui.Alert(summaryVariant, "contact-import-summary", g.Text(summary)),
		// ui.Card TIDAK dipakai (pola sama accounts_import_preview.go) — tak
		// punya min-w-0 di `.card`/`.card-body`, menolak menyusut di bawah
		// lebar-min tabel banyak kolom ini.
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

func contactImportPreviewRow(columns []AccountImportColumn, row ContactImportPreviewRow) g.Node {
	// whitespace-nowrap WAJIB di badge status (pola sama importPreviewRow,
	// accounts_import_preview.go) — .badge daisyUI tak punya white-space:
	// nowrap bawaan, TableScroll yang menyerap kelebihan lebar via scroll.
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
