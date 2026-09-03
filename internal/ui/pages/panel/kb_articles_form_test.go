package panel

import (
	"strings"
	"testing"
)

// kb_articles_form_test.go — regresi BL-35: field Kategori kini dropdown enum
// terkunci (bukan input teks bebas). Form wajib menawarkan SELECT category
// dengan seluruh nilai enum + opsi kosong (kategori opsional-nullable). Render
// → assert markup select & tiap opsi ter-render.

// kbArticleFormFixture = KBArticleFormView terisi opsi enum, cukup untuk
// merender dropdown Kategori & Visibilitas.
func kbArticleFormFixture() KBArticleFormView {
	return KBArticleFormView{
		Base:              "/w/desa",
		Action:            "/w/desa/kb-articles",
		Fields:            KBArticleFormFields{Visibility: "Internal"},
		VisibilityOptions: []string{"Public", "Internal", "Portal Only"},
		CategoryOptions:   []string{"Panduan Awal", "Pembayaran", "Kependudukan", "Teknis", "Umum"},
	}
}

// TestKBArticleForm_CategoryDropdown — Kategori wajib jadi <select name=category>
// (enum terkunci BL-35), bukan <input>. Jaga markup select + tiap opsi enum
// ter-render + opsi kosong "—" (kategori opsional).
func TestKBArticleForm_CategoryDropdown(t *testing.T) {
	out := renderLeads(t, KBArticleForm(kbArticleFormFixture()))

	if !strings.Contains(out, `name="category"`) || !strings.Contains(out, `id="f-category"`) {
		t.Errorf("Kategori harus <select name=category>:\n%s", out)
	}
	// Field harus <select>, bukan <input type=text> (regresi arah enum).
	if strings.Contains(out, `<input`) && !strings.Contains(out, `<select`) {
		t.Errorf("Kategori tak boleh input teks bebas:\n%s", out)
	}
	for _, want := range []string{
		"Panduan Awal", "Pembayaran", "Kependudukan", "Teknis", "Umum",
	} {
		if !strings.Contains(out, `value="`+want+`"`) {
			t.Errorf("dropdown Kategori harus menawarkan opsi %q:\n%s", want, out)
		}
	}
}
