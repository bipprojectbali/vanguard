package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_leads_import.go — BL-133: form unggah CSV massal Lead. Native
// multipart/form-data POST → 303 (gotcha #16 — sama alasan accounts_import.go
// BL-63). Kartu unggah (LeadImportUploadCard) dipakai bersama oleh halaman
// form kosong (LeadImportForm) DAN halaman pratinjau
// (sales_leads_import_preview.go) — pratinjau dirender DI BAWAH form yang
// SAMA, bukan navigasi terpisah, agar operator bisa langsung unggah ulang
// tanpa balik halaman.

// LeadImportFormView = data form unggah (dipakai halaman kosong maupun
// sebagai bagian atas halaman pratinjau).
type LeadImportFormView struct {
	Base         string
	Action       string
	TemplateURL  string
	Err          string
	MaxRows      int
	MaxFileBytes int
}

// LeadImportUploadCard merender kartu unggah (info kolom wajib + tautan
// template + form file). Dipakai APA ADANYA oleh LeadImportForm & di atas
// tabel pratinjau LeadImportPreview.
func LeadImportUploadCard(v LeadImportFormView) g.Node {
	maxMiB := strconv.Itoa(v.MaxFileBytes / (1 << 20))
	return ui.Card(
		h.P(h.Class("text-sm"),
			g.Text("Kolom wajib: "), h.Code(g.Text("lead_name")), g.Text(", "), h.Code(g.Text("kode_kecamatan")),
			g.Text(". Unduh template untuk daftar lengkap kolom opsional & format nilainya.")),
		// Tautan template & trigger pencarian kode wilayah digrup satu baris
		// flex (konvensi mobile-first — baris tombol horizontal wajib
		// flex-wrap), pola sama accounts_import.go.
		h.Div(h.Class("flex flex-wrap items-center gap-x-4 gap-y-2"),
			h.A(h.Href(v.TemplateURL), h.Class("link link-primary text-sm w-fit"),
				g.Text("Unduh Template CSV")),
			// BL-163: rujukan cepat kode Kemendagri Kecamatan saat menyiapkan
			// file CSV di luar aplikasi.
			ui.RegionSearchTrigger(),
		),
		h.FormEl(
			h.Method("post"), h.Action(v.Action), h.EncType("multipart/form-data"),
			h.Class("grid gap-4 min-w-0"),
			h.Div(
				ui.Label("File CSV"),
				h.Input(h.Type("file"), h.Name("csv_file"), h.Accept(".csv,text/csv"), h.Required(),
					h.Class("file-input w-full")),
				h.P(h.Class("text-xs text-base-content/60 mt-1"),
					g.Text("Maks "+strconv.Itoa(v.MaxRows)+" baris data, ukuran file "+maxMiB+" MiB.")),
			),
			h.Button(h.Type("submit"), h.Class("btn btn-primary w-fit"), g.Text("Pratinjau")),
		),
	)
}

// leadImportHeader = judul + deskripsi + tautan kembali — SAMA di halaman
// form kosong maupun pratinjau (satu halaman logis, dua state render).
func leadImportHeader(base string) g.Node {
	return h.Div(
		h.H1(h.Class("text-xl font-semibold"), g.Text("Impor Lead (CSV)")),
		h.P(h.Class("text-sm text-base-content/60 mb-1"),
			g.Text("Unggah banyak lead sekaligus dari file CSV. Satu baris tak valid membatalkan seluruh file — tak ada impor sebagian.")),
		h.A(h.Href(base+"/leads"), h.Class("text-sm text-base-content/60"),
			g.Text("« Kembali ke daftar lead")),
	)
}

// LeadImportForm merender form unggah CSV kosong + tautan unduh template.
func LeadImportForm(v LeadImportFormView) g.Node {
	body := []g.Node{leadImportHeader(v.Base)}
	if v.Err != "" {
		body = append(body, ui.Toast(ui.VariantDestructive, "lead-import-err", g.Text(v.Err)))
	}
	body = append(body, LeadImportUploadCard(v))
	return h.Div(h.Class("grid gap-6 min-w-0"), g.Group(body))
}
