package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// ── Form buat/sunting artikel ────────────────────────────────────────────

// KBArticleFormFields = nilai prefill form (semua string agar view netral
// terhadap tipe DB). Kosong (buat) atau terisi (sunting). status TAK di
// sini — transisi jalur tersendiri.
type KBArticleFormFields struct {
	ArticleTitle string
	ArticleBody  string
	Category     string
	Keywords     string
	Visibility   string
}

// KBArticleFormView = data halaman form. Action = URL POST tujuan.
// Visibility/CategoryOptions dioper handler (view tak memutuskan enum;
// category jadi enum terkunci sejak BL-35).
type KBArticleFormView struct {
	Base            string
	Action          string
	IsEdit          bool
	Err             string
	Fields          KBArticleFormFields
	CategoryOptions []string
	// VisibilityOptions SENGAJA dibuang (BL-37): Visibilitas kini field
	// read-only (visibilityLockedField), bukan dropdown yang menawarkan opsi.
	// Saat Portal v2 tiba, kembalikan field ini + selectField editable.
}

// KBArticleForm merender halaman form lengkap (native POST → 303, gotcha
// #16).
func KBArticleForm(v KBArticleFormView) g.Node {
	title, submit := "Tambah Artikel", "Simpan Artikel"
	if v.IsEdit {
		title, submit = "Sunting Artikel", "Simpan Perubahan"
	}
	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text(title)),
			h.A(h.Href(v.Base+"/kb-articles"), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke katalog")),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "kb-article-form-err", g.Text(v.Err)))
	}
	body = append(body, h.FormEl(
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),
		formCard("Identitas",
			field("Judul Artikel", "article_title", v.Fields.ArticleTitle, true, "text"),
			selectField("Kategori", "category", v.Fields.Category, v.CategoryOptions, false),
			field("Kata Kunci", "keywords", v.Fields.Keywords, false, "text"),
			visibilityLockedField(v.Fields.Visibility),
		),
		formCard("Isi",
			textareaField("Isi Artikel", "article_body", v.Fields.ArticleBody),
		),
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text(submit)),
			h.A(h.Href(v.Base+"/kb-articles"), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// visibilityLockedField menampilkan Visibilitas sebagai field TERKUNCI
// (read-only), BL-37. Dropdown editable dihapus karena `visibility`
// (Public/Internal/Portal Only) belum punya konsumen: Portal self-service (v2)
// yang membacanya SENGAJA ditunda (migrasi 00018). Field disabled TAK
// ikut ter-submit → handler menetapkan nilainya (Internal saat create,
// pertahankan nilai lama saat update), BUKAN dari form. Meniru pola phoneField
// (field terkunci + keterangan). Nilai "" (jaga-jaga) tampil "Internal".
//
// TODO(kb-portal): saat Portal v2 dibangun, kembalikan jadi selectField
// editable + baca lagi `visibility` di parseKBArticleForm — utang eksplisit
// BL-37 (§17 CLAUDE.md).
func visibilityLockedField(val string) g.Node {
	if val == "" {
		val = "Internal"
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelFor("Visibilitas", "f-visibility_ro", false),
		ui.Input(
			h.ID("f-visibility_ro"), h.Type("text"), h.Value(val),
			h.Disabled(), h.Class("input text-base w-full"),
		),
		h.P(h.Class("text-xs text-base-content/60"),
			g.Text("Terkunci ke Internal — Portal self-service belum tersedia, "+
				"jadi visibilitas belum berefek.")),
	)
}
