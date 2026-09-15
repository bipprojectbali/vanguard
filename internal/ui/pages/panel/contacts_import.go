package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// contacts_import.go — BL-134: form unggah CSV massal kontak. Native
// multipart/form-data POST → 303 (gotcha #16), pola identik
// accounts_import.go (BL-63) dengan tipe & teks sendiri (Kontak, bukan Desa)
// — TIDAK reuse AccountImportFormView/AccountImportUploadCard krn teks kolom
// wajib berbeda domain ("kode_desa, first_name" vs "kode_desa, tipe_akun").
// ContactImportUploadCard dipakai bersama oleh halaman form kosong
// (ContactImportForm) DAN halaman pratinjau (contacts_import_preview.go).

// ContactImportFormView = data form unggah (dipakai halaman kosong maupun
// sebagai bagian atas halaman pratinjau).
type ContactImportFormView struct {
	Base         string
	Action       string
	TemplateURL  string
	Err          string
	MaxRows      int
	MaxFileBytes int
}

// ContactImportUploadCard merender kartu unggah (info kolom wajib + tautan
// template + form file). Dipakai APA ADANYA oleh ContactImportForm & di atas
// tabel pratinjau ContactImportPreview.
func ContactImportUploadCard(v ContactImportFormView) g.Node {
	maxMiB := strconv.Itoa(v.MaxFileBytes / (1 << 20))
	return ui.Card(
		h.P(h.Class("text-sm"),
			g.Text("Kolom wajib: "), h.Code(g.Text("kode_desa")), g.Text(", "), h.Code(g.Text("first_name")),
			g.Text(". Unduh template untuk daftar lengkap kolom opsional & format nilainya.")),
		h.A(h.Href(v.TemplateURL), h.Class("link link-primary text-sm w-fit"),
			g.Text("Unduh Template CSV")),
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

// contactImportHeader = judul + deskripsi + tautan kembali — SAMA di halaman
// form kosong maupun pratinjau (satu halaman logis, dua state render).
func contactImportHeader(base string) g.Node {
	return h.Div(
		h.H1(h.Class("text-xl font-semibold"), g.Text("Impor Kontak (CSV)")),
		h.P(h.Class("text-sm text-base-content/60 mb-1"),
			g.Text("Unggah banyak kontak sekaligus dari file CSV. Kontak ditautkan ke desa yang sudah ada lewat kode_desa. Satu baris tak valid membatalkan seluruh file — tak ada impor sebagian.")),
		h.A(h.Href(base+"/contacts"), h.Class("text-sm text-base-content/60"),
			g.Text("« Kembali ke daftar kontak")),
	)
}

// ContactImportForm merender form unggah CSV kosong + tautan unduh template.
func ContactImportForm(v ContactImportFormView) g.Node {
	body := []g.Node{contactImportHeader(v.Base)}
	if v.Err != "" {
		body = append(body, ui.Toast(ui.VariantDestructive, "contact-import-err", g.Text(v.Err)))
	}
	body = append(body, ContactImportUploadCard(v))
	return h.Div(h.Class("grid gap-6 min-w-0"), g.Group(body))
}
