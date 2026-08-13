package handler

import (
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// kb_articles_form.go — parsing & validasi form Artikel KB, dipakai bersama
// create & update. Dipisah dari handler aksi agar aturan validasi (enum,
// wajib) punya SATU tempat: create & edit tak boleh menerima nilai yang
// berbeda sahnya untuk kolom yang sama. Meniru playbooks_form.go (slice A2).
//
// visibility PUNYA CHECK di DB (kb_articles_visibility_chk, migrasi 00018)
// — divalidasi juga di handler agar galat dilaporkan sebelum INSERT/UPDATE
// gagal, bukan sebagai 500 mentah dari constraint. category & keywords TANPA
// CHECK (teks bebas — kategori KB dinamis per workspace, beda dari
// trigger_scenario Playbooks yang enum tetap).

const maxKBArticleTitleLen = 300

// validKBArticleVisibilities = domain nilai visibility (cermin CHECK DB
// kb_articles_visibility_chk).
var validKBArticleVisibilities = map[string]struct{}{
	"Public": {}, "Internal": {}, "Portal Only": {},
}

// kbArticleVisibilityOptions = opsi dropdown BERURUT (map validasi tak
// berurutan). Nilai HARUS himpunan sama dengan map validasi.
var kbArticleVisibilityOptions = []string{"Public", "Internal", "Portal Only"}

// compile-time: opsi & map validasi sepakat (panjang sama). Berbeda =
// dropdown menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(kbArticleVisibilityOptions) != len(validKBArticleVisibilities) {
		panic("kb_articles: opsi visibility tak sinkron dengan map validasi")
	}
	return struct{}{}
}()

// kbArticleForm = nilai form Artikel KB yang SUDAH divalidasi & siap
// dipetakan ke Create/UpdateKBArticleParams. Kolom opsional pointer (nil =
// NULL). status TAK di sini — transisi status jalur tersendiri.
type kbArticleForm struct {
	ArticleTitle string
	ArticleBody  *string
	Category     *string
	Keywords     *string
	Visibility   string
}

// parseKBArticleForm membaca & memvalidasi form. (form, "") bila sah, atau
// (zero, kode) yang dipetakan kbArticlesErrMsg. Nilai user-controlled →
// validasi backend adalah penjaga sesungguhnya.
func parseKBArticleForm(fv func(string) string) (kbArticleForm, string) {
	var f kbArticleForm

	f.ArticleTitle = strings.TrimSpace(fv("article_title"))
	if f.ArticleTitle == "" || len(f.ArticleTitle) > maxKBArticleTitleLen {
		return kbArticleForm{}, "required"
	}

	// visibility wajib enum; kosong dari form (mis. JS dimatikan) tetap
	// dijaga default "Internal" di sisi handler pemanggil (KBArticleNew).
	f.Visibility = strings.TrimSpace(fv("visibility"))
	if _, ok := validKBArticleVisibilities[f.Visibility]; !ok {
		return kbArticleForm{}, "visibility"
	}

	// Teks bebas opsional: trim, kosong → NULL.
	f.ArticleBody = optTrim(fv("article_body"))
	f.Category = optTrim(fv("category"))
	f.Keywords = optTrim(fv("keywords"))

	return f, ""
}

// kbArticleFormFields memetakan Artikel KB → nilai prefill form (semua
// string; nil → "").
func kbArticleFormFields(a db.KbArticle) panel.KBArticleFormFields {
	return panel.KBArticleFormFields{
		ArticleTitle: a.ArticleTitle,
		ArticleBody:  deref(a.ArticleBody),
		Category:     deref(a.Category),
		Keywords:     deref(a.Keywords),
		Visibility:   a.Visibility,
	}
}
