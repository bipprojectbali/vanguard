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
// visibility & category PUNYA CHECK di DB (kb_articles_visibility_chk 00018,
// kb_articles_category_chk 00035) — divalidasi juga di handler agar galat
// dilaporkan sebelum INSERT/UPDATE gagal, bukan sebagai 500 mentah dari
// constraint. keywords TANPA CHECK (teks bebas).
//
// category DULU teks bebas (00018) — dikunci jadi enum GLOBAL di BL-35 (00035):
// keputusan user membalik desain "kategori dinamis per workspace" demi
// konsistensi + dropdown. Kolom tetap nullable (kosong → NULL = tak
// berkategori), jadi validasi bawah: kosong DIBOLEHKAN, non-kosong wajib enum.

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

// validKBArticleCategories = domain nilai category (cermin CHECK DB
// kb_articles_category_chk, 00035). Enum GLOBAL tetap.
var validKBArticleCategories = map[string]struct{}{
	"Panduan Awal": {}, "Pembayaran": {}, "Kependudukan": {}, "Teknis": {}, "Umum": {},
}

// kbArticleCategoryOptions = opsi dropdown BERURUT (map validasi tak berurutan).
// Nilai HARUS himpunan sama dengan map validasi.
var kbArticleCategoryOptions = []string{"Panduan Awal", "Pembayaran", "Kependudukan", "Teknis", "Umum"}

// compile-time: opsi & map validasi category sepakat (panjang sama). Berbeda =
// dropdown menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(kbArticleCategoryOptions) != len(validKBArticleCategories) {
		panic("kb_articles: opsi category tak sinkron dengan map validasi")
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

	// category opsional-tapi-terkunci: kosong → NULL (dibolehkan); non-kosong
	// WAJIB enum (00035). Divalidasi sebelum optTrim agar galat enum jelas.
	f.Category = optTrim(fv("category"))
	if f.Category != nil {
		if _, ok := validKBArticleCategories[*f.Category]; !ok {
			return kbArticleForm{}, "category"
		}
	}

	// Teks bebas opsional: trim, kosong → NULL.
	f.ArticleBody = optTrim(fv("article_body"))
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
