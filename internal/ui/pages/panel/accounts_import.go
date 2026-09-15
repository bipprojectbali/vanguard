package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// accounts_import.go — BL-63: form unggah CSV massal desa. Native
// multipart/form-data POST → 303 (gotcha #16 + belum ada presedan Datastar
// untuk unggah file di repo — jangan dicampur SSE/@post). Kartu unggah
// (AccountImportUploadCard) dipakai bersama oleh halaman form kosong
// (AccountImportForm) DAN halaman pratinjau (accounts_import_preview.go) —
// pratinjau dirender DI BAWAH form yang SAMA, bukan navigasi terpisah, agar
// operator bisa langsung unggah ulang tanpa balik halaman.

// AccountImportFormView = data form unggah (dipakai halaman kosong maupun
// sebagai bagian atas halaman pratinjau).
type AccountImportFormView struct {
	Base         string
	Action       string
	TemplateURL  string
	Err          string
	MaxRows      int
	MaxFileBytes int
}

// AccountImportUploadCard merender kartu unggah (info kolom wajib + tautan
// template + form file). Dipakai APA ADANYA oleh AccountImportForm & di atas
// tabel pratinjau AccountImportPreview.
func AccountImportUploadCard(v AccountImportFormView) g.Node {
	maxMiB := strconv.Itoa(v.MaxFileBytes / (1 << 20))
	return ui.Card(
		h.P(h.Class("text-sm"),
			g.Text("Kolom wajib: "), h.Code(g.Text("kode_desa")), g.Text(", "), h.Code(g.Text("tipe_akun")),
			g.Text(". Unduh template untuk daftar lengkap kolom opsional & format nilainya.")),
		h.A(h.Href(v.TemplateURL), h.Class("link link-primary text-sm w-fit"),
			g.Text("Unduh Template CSV")),
		// BL-163: rujukan cepat kode Kemendagri saat menyiapkan file CSV di
		// luar aplikasi (kode_desa wajib per baris, operator sering perlu
		// mencari kodenya dulu sebelum mengisi spreadsheet).
		ui.RegionSearchTrigger(),
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

// accountImportHeader = judul + deskripsi + tautan kembali — SAMA di halaman
// form kosong maupun pratinjau (satu halaman logis, dua state render).
func accountImportHeader(base string) g.Node {
	return h.Div(
		h.H1(h.Class("text-xl font-semibold"), g.Text("Impor Desa (CSV)")),
		h.P(h.Class("text-sm text-base-content/60 mb-1"),
			g.Text("Unggah banyak desa sekaligus dari file CSV. Satu baris tak valid membatalkan seluruh file — tak ada impor sebagian.")),
		h.A(h.Href(base+"/accounts"), h.Class("text-sm text-base-content/60"),
			g.Text("« Kembali ke daftar desa")),
	)
}

// AccountImportForm merender form unggah CSV kosong + tautan unduh template.
func AccountImportForm(v AccountImportFormView) g.Node {
	body := []g.Node{accountImportHeader(v.Base)}
	if v.Err != "" {
		body = append(body, ui.Toast(ui.VariantDestructive, "account-import-err", g.Text(v.Err)))
	}
	body = append(body, AccountImportUploadCard(v))
	return h.Div(h.Class("grid gap-6 min-w-0"), g.Group(body))
}
