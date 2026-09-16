package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_leads_import_preview.go — BL-133: tabel pratinjau hasil dry-run
// parse+resolve, satu baris per baris CSV, status ✅/❌ per baris (wsErrMsg
// sudah diterjemahkan di handler — view murni-data). Dirender DI BAWAH form
// unggah (LeadImportUploadCard, sales_leads_import.go) pada HALAMAN YANG SAMA
// — POST /leads/import tak lagi menavigasi ke halaman terpisah, operator
// melihat form + hasil sekaligus & bisa langsung unggah ulang. Tombol
// konfirmasi mengirim ULANG csv mentah via hidden field (raw_csv);
// LeadImportConfirm memvalidasi ulang PENUH sebelum menulis (kebijakan
// all-or-nothing BL-133). Mirror accounts_import_preview.go (BL-63).

// LeadImportColumn = satu kolom tabel pratinjau. Key merujuk kunci di
// LeadImportPreviewRow.Values — SAMA dgn nama header CSV (leadImportCSVHeader,
// sales_leads_import.go), Label = teks header manusiawi. Kolom pratinjau
// SENGAJA disamakan dgn kolom CSV agar operator bisa memverifikasi data
// mentah tiap baris SEBELUM konfirmasi — termasuk baris tak valid yang belum
// sempat ter-resolve ke master data.
type LeadImportColumn struct {
	Key   string
	Label string
}

// LeadImportPreviewRow = satu baris pratinjau — Values berisi nilai CSV APA
// ADANYA (belum diresolusi) supaya tetap tampil walau baris gagal validasi.
// ErrMsg == "" → baris valid. WarnMsg (BL-133 follow-up) = peringatan
// NON-BLOCKING (lead_name dan/atau kecamatan sudah dipakai lead lain di
// tenant) — HANYA diisi pada baris yang sudah valid (ErrMsg == ""); baris
// tak valid tak butuh peringatan tambahan, statusnya sudah jelas dari ErrMsg.
type LeadImportPreviewRow struct {
	Values  map[string]string
	ErrMsg  string
	WarnMsg string
}

// LeadImportPreviewView = data halaman pratinjau. Upload = form unggah yang
// dirender ULANG di atas hasil (lihat komentar file) — sama field-nya dgn
// LeadImportFormView, tanpa Err (galat file sudah gagal sebelum sampai ke
// pratinjau, ditangani via redirect ?err= ke halaman kosong). ValidCount,
// InvalidCount & WarnCount DIHITUNG DI HANDLER (bukan diturunkan dari Rows di
// sini) — view murni-data, tak boleh menghitung ulang state yang bisa
// dihitung sekali saat handler sudah punya rr.errCode/WarnMsg per baris.
// WarnCount TIDAK mempengaruhi AnyFailed — peringatan duplikat (BL-133
// follow-up) SOFT-WARNING, baris berperingatan tetap terhitung ValidCount
// dan tombol konfirmasi tetap aktif.
type LeadImportPreviewView struct {
	Base         string
	Upload       LeadImportFormView
	ConfirmURL   string
	RawCSV       string
	AnyFailed    bool
	ValidCount   int
	InvalidCount int
	WarnCount    int
	Columns      []LeadImportColumn
	Rows         []LeadImportPreviewRow
}

// LeadImportPreview merender: header + form unggah (SAMA komponen dgn
// LeadImportForm) di atas, lalu ringkasan + tabel status per baris + form
// konfirmasi (hidden raw_csv) di bawahnya. Saat AnyFailed, tombol konfirmasi
// DINONAKTIFKAN — kebijakan all-or-nothing berarti submit pasti DITOLAK utuh
// selama masih ada baris tak valid (server tetap penjaga akhir di
// LeadImportConfirm bila state ini pernah dilewati, mis. JS dimatikan).
func LeadImportPreview(v LeadImportPreviewView) g.Node {
	rows := make([]g.Node, 0, len(v.Rows))
	for _, row := range v.Rows {
		rows = append(rows, leadImportPreviewRow(v.Columns, row))
	}

	// Ringkasan SELALU menyebut total valid & tak valid (bukan cuma "semua
	// valid" polos) — operator langsung tahu proporsinya tanpa menghitung
	// baris/badge sendiri di tabel bawahnya. AnyFailed tetap presedan
	// TERTINGGI (all-or-nothing menolak SELURUH impor); WarnCount (BL-133
	// follow-up, soft-warning) hanya ditambahkan bila TAK ADA baris gagal —
	// duplikat nama/kecamatan tak pernah menggagalkan impor.
	summary := strconv.Itoa(v.ValidCount) + " dari " + strconv.Itoa(len(v.Rows)) + " baris valid — siap diimpor."
	summaryVariant := ui.VariantSuccess
	if v.AnyFailed {
		summary = strconv.Itoa(v.ValidCount) + " valid, " + strconv.Itoa(v.InvalidCount) + " tak valid dari " +
			strconv.Itoa(len(v.Rows)) + " baris — impor akan DITOLAK SELURUHNYA (all-or-nothing). Perbaiki file lalu unggah ulang."
		summaryVariant = ui.VariantWarning
	} else if v.WarnCount > 0 {
		summary += " " + strconv.Itoa(v.WarnCount) + " baris punya peringatan duplikat (lihat kolom Status) — tetap bisa diimpor."
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
		leadImportHeader(v.Base),
		LeadImportUploadCard(v.Upload),
		h.H2(h.Class("text-lg font-semibold"), g.Text("Hasil Pratinjau")),
		ui.Alert(summaryVariant, "lead-import-summary", g.Text(summary)),
		// ui.Card TIDAK dipakai di sini — helper itu tak punya min-w-0 di
		// `.card`/`.card-body`, jadi menolak menyusut di bawah lebar-min tabel
		// banyak-kolom ini & mendorong SELURUH halaman ke kanan (offside)
		// alih-alih scroll terkurung di TableScroll. Pola sama persis dgn
		// accounts_import_preview.go/memberList.
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

func leadImportPreviewRow(columns []LeadImportColumn, row LeadImportPreviewRow) g.Node {
	// whitespace-nowrap WAJIB di badge status — pola sama importPreviewRow
	// (accounts_import_preview.go): .badge daisyUI tak punya white-space:
	// nowrap bawaan, tabel w-full meremas kolom terakhir ke lebar sisa.
	// Prioritas: ErrMsg (gagal validasi) > WarnMsg (BL-133 follow-up,
	// duplikat non-blocking) > "Valid" polos.
	status := h.Span(h.Class("badge badge-success whitespace-nowrap"), g.Text("Valid"))
	switch {
	case row.ErrMsg != "":
		status = h.Span(h.Class("badge badge-error whitespace-nowrap"), g.Text(row.ErrMsg))
	case row.WarnMsg != "":
		status = h.Span(h.Class("badge badge-warning whitespace-nowrap"), g.Text(row.WarnMsg))
	}

	cells := make([]g.Node, 0, len(columns)+1)
	for _, col := range columns {
		cells = append(cells, h.Td(h.Class("py-2 pr-4 max-w-[160px]"),
			h.Span(h.Class("block truncate"), g.Text(row.Values[col.Key]))))
	}
	cells = append(cells, h.Td(h.Class("py-2 whitespace-nowrap"), status))

	return h.Tr(h.Class("border-b border-base-300/50"), g.Group(cells))
}
