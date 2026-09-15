package ui

import (
	"strings"
	"testing"
)

// region_search_test.go — BL-163. Regresi bug laporan user: ketikan di modal
// tak memicu perubahan apa pun. Akar masalah: RegionSearchModal merakit URL
// @post sebagai path ABSOLUT "/regions/search", padahal endpoint-nya ter-NEST
// di bawah rute workspace (routes.go: r.Post di dalam grup "/w/{workspace}")
// — request selalu 404 diam-diam (gagal fetch, bukan error Datastar yang
// terlihat). Fix: RegionSearchModal terima wsBase, dirakit dari
// ShellData.WSBase (appshell.go) yang diisi render_shell.go dari
// slugFromRequest(r).

// TestRegionSearchModal_PostURLIncludesWorkspacePrefix membuktikan expr
// @post di input live-search memuat wsBase — TANPA ini, ketikan di modal
// selalu 404 diam-diam (bug yang dilaporkan user).
func TestRegionSearchModal_PostURLIncludesWorkspacePrefix(t *testing.T) {
	var sb strings.Builder
	if err := RegionSearchModal("/w/desa-plus").Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := sb.String()
	// g.Text/atribut HTML-escape kutip tunggal jadi &#39; — cek bentuk yang
	// SUNGGUHAN dirender, bukan JS mentah.
	if !strings.Contains(out, "@post(&#39;/w/desa-plus/regions/search?q=&#39;") {
		t.Errorf("expr @post harus diawali prefix workspace \"/w/desa-plus\":\n%s", out)
	}
	// REGRESI konkret: path absolut lama TANPA prefix workspace tak boleh
	// muncul sebagai target @post (akan 404 di rute nyata).
	if strings.Contains(out, "@post(&#39;/regions/search?q=&#39;") {
		t.Errorf("expr @post TIDAK boleh path absolut tanpa prefix workspace (404 di rute nyata):\n%s", out)
	}
}

// TestRegionSearchModal_EmptyBase: di luar konteks workspace (/dev,
// /notifications — trigger belum dipasang di sana), wsBase kosong tetap
// menghasilkan markup valid (bukan panic/URL rusak "//regions/search").
func TestRegionSearchModal_EmptyBase(t *testing.T) {
	var sb strings.Builder
	if err := RegionSearchModal("").Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "@post(&#39;/regions/search?q=&#39;") {
		t.Errorf("wsBase kosong harus tetap hasilkan \"/regions/search\" (bukan \"//regions/search\" atau kosong):\n%s", out)
	}
}
