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

// TestRegionSearchModal_TopPosition membuktikan backdrop diposisikan di ATAS
// viewport (permintaan user), BUKAN center-viewport lagi (regresi posisi).
func TestRegionSearchModal_TopPosition(t *testing.T) {
	var sb strings.Builder
	if err := RegionSearchModal("/w/desa-plus").Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := sb.String()
	// Cek CLASS BACKDROP spesifik (bukan "items-center" bare — string itu
	// juga dipakai header internal kartu utk flex-align tak terkait posisi
	// modal, false positive bila dicek longgar).
	if !strings.Contains(out, `class="fixed inset-0 z-50 flex items-start justify-center`) {
		t.Errorf("class backdrop harus memuat \"items-start\" (posisi atas):\n%s", out)
	}
	if strings.Contains(out, `class="fixed inset-0 z-50 flex items-center justify-center`) {
		t.Errorf("class backdrop TIDAK boleh lagi memuat \"items-center\" (posisi lama):\n%s", out)
	}
}

// TestRegionSearchModal_Tabs membuktikan dua tombol tab ("Kode/Nama Desa" &
// "Wilayah") ada, masing-masing meng-set regionSearchTab lewat klik.
func TestRegionSearchModal_Tabs(t *testing.T) {
	var sb strings.Builder
	if err := RegionSearchModal("/w/desa-plus").Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "Kode/Nama Desa") {
		t.Errorf("label tab \"Kode/Nama Desa\" tak ditemukan:\n%s", out)
	}
	if !strings.Contains(out, "Wilayah") {
		t.Errorf("label tab \"Wilayah\" tak ditemukan:\n%s", out)
	}
	if !strings.Contains(out, "$regionSearchTab = &#39;kode&#39;") {
		t.Errorf("tab \"kode\" harus set $regionSearchTab = 'kode':\n%s", out)
	}
	if !strings.Contains(out, "$regionSearchTab = &#39;wilayah&#39;") {
		t.Errorf("tab \"wilayah\" harus set $regionSearchTab = 'wilayah':\n%s", out)
	}
}

// TestRegionSearchModal_TreePanel_PostURLIncludesWorkspacePrefix membuktikan
// select Kecamatan panel "Wilayah" merakit @post dgn prefix wsBase — sama
// pola bug 404 diam-diam yang sudah diperbaiki di tab "kode"
// (TestRegionSearchModal_PostURLIncludesWorkspacePrefix), regresi utk cabang
// district= tab baru.
func TestRegionSearchModal_TreePanel_PostURLIncludesWorkspacePrefix(t *testing.T) {
	var sb strings.Builder
	if err := RegionSearchModal("/w/desa-plus").Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "@post(&#39;/w/desa-plus/regions/search?district=&#39;") {
		t.Errorf("expr @post select Kecamatan harus diawali prefix workspace \"/w/desa-plus\":\n%s", out)
	}
	if !strings.Contains(out, "data-tree-url=\"/w/desa-plus/regions/tree\"") {
		t.Errorf("data-tree-url harus diawali prefix workspace \"/w/desa-plus\":\n%s", out)
	}
}

// TestRegionSearchModal_TreePanel_EmptyBase: wsBase kosong tetap
// menghasilkan URL valid (bukan "//regions/search"/"//regions/tree") — sama
// alasan TestRegionSearchModal_EmptyBase, utk panel "Wilayah".
func TestRegionSearchModal_TreePanel_EmptyBase(t *testing.T) {
	var sb strings.Builder
	if err := RegionSearchModal("").Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "@post(&#39;/regions/search?district=&#39;") {
		t.Errorf("wsBase kosong harus tetap hasilkan \"/regions/search?district=\":\n%s", out)
	}
	if !strings.Contains(out, "data-tree-url=\"/regions/tree\"") {
		t.Errorf("wsBase kosong harus tetap hasilkan data-tree-url=\"/regions/tree\":\n%s", out)
	}
}
