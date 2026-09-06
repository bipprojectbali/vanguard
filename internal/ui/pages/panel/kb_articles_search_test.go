package panel

import (
	"strings"
	"testing"
)

// kb_articles_search_test.go — regresi BL-36: kotak pencarian + penerusan ?q=
// pada daftar Knowledge Base. Kembaran subscriptions_search_test.go. Murni-data
// (tanpa DB): yang dijaga adalah KONTRAK URL — kotak cari native GET
// (bookmarkable, buang after), q diisi ulang & ter-escape, dan SEMUA jalur
// navigasi (tab Aktif/Arsip, pager "Berikutnya", tautan reset) membawa/menjaga
// q agar pencarian bertahan lintas halaman & tab. Penyempitan hasil diuji di
// sisi handler (kb_articles_search_test.go paket handler). Memakai ulang
// assertSearchBox/assertEscapedQ/renderLeads dari sales_search_test.go.

func TestKBArticleList_SearchBoxAndThreading(t *testing.T) {
	out := renderLeads(t, KBArticleList(KBArticleListView{
		Base:       "/w/desa",
		Tab:        "aktif",
		Query:      "kartu keluarga", // ada spasi → wajib ter-escape
		Items:      []KBArticleRow{{ID: 1, ArticleTitle: "Artikel Cocok", Status: "Published"}},
		NextCursor: "99_9",
	}))

	assertSearchBox(t, out, "/w/desa/kb-articles", "kartu keluarga")
	// BL-68: tab Aktif/Arsip & kotak cari sebaris dalam satu wrapper justify-between.
	assertTabSearchRow(t, out, "/w/desa/kb-articles")
	// Pager membawa after + q ('&' dirender &amp; → q dicek terpisah).
	if !strings.Contains(out, "after=99_9") {
		t.Errorf("pager harus membawa after=99_9:\n%s", out)
	}
	// q muncul di pager DAN di tautan tab (ganti tab tak menjatuhkan q).
	assertEscapedQ(t, out, "kartu keluarga")
}

// TestKBArticleList_SearchKeepsArsipTab — di tab Arsip, kotak cari menjaga
// tab lewat input tersembunyi, dan tautan tab/pager membawa ?tab=arsip + q.
func TestKBArticleList_SearchKeepsArsipTab(t *testing.T) {
	out := renderLeads(t, KBArticleList(KBArticleListView{
		Base:       "/w/desa",
		Tab:        "arsip",
		Query:      "sandi",
		Items:      []KBArticleRow{{ID: 2, ArticleTitle: "Artikel Arsip", Status: "Archived"}},
		NextCursor: "50_5",
	}))

	if !strings.Contains(out, `<input type="hidden" name="tab" value="arsip">`) {
		t.Errorf("form cari di tab Arsip harus menjaga tab via input tersembunyi tab=arsip:\n%s", out)
	}
	if !strings.Contains(out, "tab=arsip") {
		t.Errorf("pager/tab harus membawa tab=arsip:\n%s", out)
	}
	assertEscapedQ(t, out, "sandi")
}

func TestKBArticleList_SearchNoMatchEmptyState(t *testing.T) {
	out := renderLeads(t, KBArticleList(KBArticleListView{
		Base: "/w/desa", Tab: "aktif",
		Query: "zzz", Items: nil, NextCursor: "",
	}))
	if !strings.Contains(out, "Belum ada artikel yang cocok pencarian") {
		t.Errorf("kueri tanpa hasil harus empty-state pencarian, bukan 'belum ada artikel di katalog':\n%s", out)
	}
	// Tautan reset mengosongkan q (tab Aktif = default → URL polos tanpa param).
	if !strings.Contains(out, `href="/w/desa/kb-articles"`) {
		t.Errorf("tautan reset harus tanpa q (URL polos di tab Aktif):\n%s", out)
	}
}

// TestKBArticleList_SearchResetKeepsArsip — Reset di tab Arsip membuang q tapi
// mempertahankan ?tab=arsip (kembali ke awal arsip, bukan melebar ke Aktif).
func TestKBArticleList_SearchResetKeepsArsip(t *testing.T) {
	out := renderLeads(t, KBArticleList(KBArticleListView{
		Base: "/w/desa", Tab: "arsip",
		Query: "zzz", Items: nil, NextCursor: "",
	}))
	if !strings.Contains(out, `href="/w/desa/kb-articles?tab=arsip"`) {
		t.Errorf("tautan reset di tab Arsip harus tanpa q tapi menjaga tab=arsip:\n%s", out)
	}
}
