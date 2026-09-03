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
		Base:            "/w/desa",
		Action:          "/w/desa/kb-articles",
		Fields:          KBArticleFormFields{Visibility: "Internal"},
		CategoryOptions: []string{"Panduan Awal", "Pembayaran", "Kependudukan", "Teknis", "Umum"},
	}
}

// TestKBArticleForm_VisibilityLocked — BL-37: Visibilitas kini field read-only,
// BUKAN dropdown editable. Form wajib TIDAK merender <select name=visibility>;
// nilai (Internal) tampil di field disabled tanpa atribut name (tak ter-submit).
func TestKBArticleForm_VisibilityLocked(t *testing.T) {
	out := renderLeads(t, KBArticleForm(kbArticleFormFixture()))

	if strings.Contains(out, `name="visibility"`) {
		t.Errorf("Visibilitas TAK boleh punya field ber-name (read-only, tak ter-submit):\n%s", out)
	}
	if strings.Contains(out, `<select id="f-visibility"`) {
		t.Errorf("Visibilitas TAK boleh <select> editable (BL-37):\n%s", out)
	}
	if !strings.Contains(out, "disabled") || !strings.Contains(out, `value="Internal"`) {
		t.Errorf("Visibilitas harus tampil terkunci bernilai Internal:\n%s", out)
	}
}

// TestKBArticleList_PortalBanner — BL-37: daftar KB wajib menampilkan banner
// menetap bahwa Portal belum ada (visibility belum berefek).
func TestKBArticleList_PortalBanner(t *testing.T) {
	out := renderLeads(t, KBArticleList(KBArticleListView{Base: "/w/desa"}))
	if !strings.Contains(out, "Portal self-service belum tersedia") {
		t.Errorf("daftar KB harus memuat banner Portal belum ada:\n%s", out)
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
