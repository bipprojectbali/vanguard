package ui

// region_search.go — BL-163: modal global "Cari Kode Desa/Kecamatan". Dirender
// SEKALI di AppShell (pola sama ConfirmModal/ChangelogModal: signal Datastar
// boolean, inline display:none anti-FOUC), dipicu dari MANA PUN via
// RegionSearchTrigger (mis. form Akun, halaman impor CSV — lihat call site).
// Pencarian live: input ter-debounce men-@post fragmen hasil (SearchRegions{
// ByCode,ByName} di handler regions_search.go) — modal TETAP TERBUKA sepanjang
// round-trip (SSE PatchElements fragmen, bukan navigasi; lolos gotcha #16).
//
// Perluasan (lanjutan BL-163): modal diposisikan di ATAS viewport (bukan
// center) + DUA tab — "Kode/Nama Desa" (perilaku di atas, tak berubah) dan
// "Wilayah" (BARU: cascading Provinsi→Kabupaten/Kota→Kecamatan, lazy-fetch
// via static/regiontree.js + regions_tree.go, memicu pencarian yang SAMA
// lewat district=<id> — lihat regionSearchTreePanel).

import (
	"strconv"
	"time"

	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// regionSearchSignal = signal Datastar boolean pengendali modal.
const regionSearchSignal = "regionSearchOpen"

// regionSearchTabSignal = signal Datastar string ("kode"|"wilayah") pengendali
// tab AKTIF modal (perluasan BL-163). Signal (bukan <a href>+query-param
// seperti tab lain di repo, mis. accountsTablist) karena modal ini harus
// TETAP TERBUKA saat pindah tab — navigasi/reload akan menutupnya. Konsisten
// dgn pola modal ini sendiri yang sudah signal-driven (regionSearchSignal).
const regionSearchTabSignal = "regionSearchTab"

// regionSearchTreeGroup = embedID/group pembeda cascading tab "Wilayah" dari
// grup regionSelect form lain (internal/ui/pages/panel/regionselect.go) yang
// mungkin ada di halaman yang sama — static/regiontree.js menyaring elemen
// via [data-tree-group="..."].
const regionSearchTreeGroup = "region-search-tree"

// RegionSearchMinChars = panjang minimum ketikan sebelum live-search jalan
// (keputusan desain BL-163). Dipakai DUA sisi: expr client (skip @post di
// bawah panjang ini — tak ada request terkirim sama sekali) dan guard server
// (regions_search.go, pertahanan jika client dilewati/dimanipulasi).
const RegionSearchMinChars = 3

// regionSearchResultsID = id kontainer hasil yang di-SSE-patch oleh handler.
const regionSearchResultsID = "region-search-results"

// RegionSearchHintNoResults = pesan kontainer hasil saat sudah cari (≥
// RegionSearchMinChars karakter, atau Kecamatan terpilih di tab Wilayah) tapi
// nihil — DIBEDAKAN dari pesan default "belum cukup ketikan" (RegionSearchResults
// tanpa hint eksplisit) agar user yang sudah mencari tak dikira belum apa-apa.
const RegionSearchHintNoResults = "Tidak ada hasil ditemukan."

// regionSearchHintTooShort = pesan default kontainer hasil (belum ada
// pencarian / ketikan < RegionSearchMinChars). var (bukan const) karena
// dirakit dari RegionSearchMinChars via strconv — satu sumber kebenaran,
// tak ada angka "3" terduplikasi di string literal.
var regionSearchHintTooShort = "Ketik minimal " + strconv.Itoa(RegionSearchMinChars) + " karakter untuk mencari, atau pilih tab \"Wilayah\"."

// RegionSearchRow = SATU baris hasil pencarian, bentuk TAMPILAN murni (handler
// memetakan dari db.SearchRegionsBy{Code,Name,District}Row) — Desa "" menandakan
// baris Kecamatan (level 3, tak punya desa), dirender "-".
type RegionSearchRow struct {
	Code      string
	Desa      string
	Kecamatan string
	Kabupaten string
	Provinsi  string
}

// RegionSearchTrigger = tombol pembuka modal, disisipkan di halaman mana pun
// yang butuh rujukan kode wilayah (form Akun, impor CSV Desa, dst). Ikon +
// label kecil agar jelas fungsinya tanpa memakan banyak ruang di form.
func RegionSearchTrigger() g.Node {
	return h.Button(
		h.Type("button"),
		h.Class("btn btn-ghost btn-xs gap-1 text-primary"),
		data.On("click", "$"+regionSearchSignal+" = true"),
		lucide.Search(h.Class("size-3.5")),
		g.Text("Cari Kode Desa/Kecamatan"),
	)
}

// RegionSearchModal = dialog pencarian. Pola identik ConfirmModal/ChangelogModal
// (signal + backdrop + stop-propagation kartu dalam), diposisikan di ATAS
// viewport (items-start, bukan items-center — permintaan user), body berisi
// tab bar + panel per tab + kontainer hasil SHARED yang di-patch handler.
// wsBase = prefix workspace aktif ("/w/{slug}", dari ShellData.WSBase) —
// endpoint /regions/search DAN /regions/tree ter-NEST di bawah rute workspace
// (routes.go), BUKAN path absolut; tanpa prefix ini @post/fetch 404 diam-diam
// (bug BL-163 sebelumnya: ketikan tak memicu perubahan apa pun karena request
// tak pernah sampai ke handler).
func RegionSearchModal(wsBase string) g.Node {
	openExpr := "$" + regionSearchSignal
	tabExpr := "$" + regionSearchTabSignal
	searchURL := wsBase + "/regions/search"
	treeURL := wsBase + "/regions/tree"
	minChars := strconv.Itoa(RegionSearchMinChars)
	// Guard panjang di EKSPRESI sendiri (bukan cuma placeholder): && pendek-
	// sirkuit — di bawah minChars, @post tak pernah dievaluasi = tak ada
	// request terkirim (kasus uji "<3 karakter no-request").
	inputExpr := "el.value.trim().length >= " + minChars +
		" && @post('" + searchURL + "?q='+encodeURIComponent(el.value.trim()))"
	return h.Div(
		h.Class("fixed inset-0 z-50 flex items-start justify-center bg-black/50 p-4 pt-10 sm:pt-16"),
		g.Attr("style", "display:none"),
		data.Show(openExpr),
		data.On("click", openExpr+" = false"),
		h.Div(
			h.Class("card bg-base-100 shadow-lg w-full max-w-2xl flex flex-col"),
			g.Attr("style", "max-height:85vh"),
			data.On("click", "evt.stopPropagation()"),
			// Header tetap.
			h.Div(
				h.Class("flex items-center justify-between gap-2 px-5 pt-5 pb-3 border-b border-base-300"),
				h.H2(
					h.Class("text-lg font-semibold flex items-center gap-2"),
					lucide.Search(h.Class("size-5 text-primary")),
					g.Text("Cari Kode Desa/Kecamatan"),
				),
				h.Button(
					h.Type("button"),
					h.Class("btn btn-ghost btn-sm btn-circle"),
					g.Attr("aria-label", "Tutup"),
					data.On("click", openExpr+" = false"),
					lucide.X(h.Class("size-5")),
				),
			),
			regionSearchTabs(tabExpr),
			// Body: panel per tab + hasil (scroll, SATU kontainer shared).
			h.Div(
				h.Class("px-5 py-4 flex flex-col gap-3 overflow-y-auto min-w-0"),
				h.Div(
					data.Show(tabExpr+" == 'kode'"),
					h.Input(
						h.Type("text"), h.Name("q"),
						h.Class("input input-bordered w-full text-base"),
						h.Placeholder("Ketik kode (mis. 32.01.01) atau nama Kecamatan/Desa, minimal "+minChars+" karakter…"),
						g.Attr("autocomplete", "off"),
						data.On("input", inputExpr, data.ModifierDebounce, data.Duration(400*time.Millisecond)),
					),
				),
				h.Div(
					data.Show(tabExpr+" == 'wilayah'"),
					regionSearchTreePanel(searchURL, treeURL),
				),
				RegionSearchResults(nil, ""),
			),
		),
	)
}

// regionSearchTabs = bilah 2 tab (perluasan BL-163): "Kode/Nama Desa"
// (perilaku lama) vs "Wilayah" (cascading Provinsi→Kabupaten/Kota→Kecamatan).
// Signal-driven (regionSearchTabSignal) — PERTAMA di repo (tab lain, mis.
// accountsTablist, pakai <a href>+query-param+reload), tapi konsisten dgn
// pola modal ini sendiri: navigasi/reload akan menutup modal, jadi harus
// tetap dalam paradigma Datastar (CLAUDE.md §Batasan: satu paradigma).
// tabs-box (BUKAN tabs-boxed — nama daisyUI v4, tree-shaken di v5, gotcha #4)
// sama dgn accountsTablist.go; tab-active via ClassOn (gotcha #5, bukan
// data.Class mentah).
func regionSearchTabs(tabExpr string) g.Node {
	tab := func(label, value string) g.Node {
		return h.Button(
			h.Type("button"),
			h.Role("tab"),
			h.Class("tab min-h-11"),
			ClassOn("tab-active", tabExpr+" == '"+value+"'"),
			data.On("click", tabExpr+" = '"+value+"'"),
			g.Text(label),
		)
	}
	return h.Div(
		h.Role("tablist"),
		h.Class("tabs tabs-box flex-wrap px-5 pt-3"),
		tab("Kode/Nama Desa", "kode"),
		tab("Wilayah", "wilayah"),
	)
}

// regionSearchTreePanel = panel tab "Wilayah" (BARU): 3 <select> cascading
// Provinsi→Kabupaten/Kota→Kecamatan, LAZY-FETCH per level lewat treeURL
// (wsBase + "/regions/tree", handler regions_tree.go) — BUKAN embed dataset
// penuh seperti regionSelect (internal/ui/pages/panel/regionselect.go):
// modal ini dirender di SETIAP halaman lewat AppShell, embed ~7.817 baris di
// sana jadi regresi payload global. static/regiontree.js mem-populate opsi;
// Kecamatan (level 3) memicu pencarian LANGSUNG lewat atribut Datastar
// data-on:change DI SINI (bukan dari JS) — JS hanya isi <option> &
// enable/disable, trigger @post tetap satu paradigma Datastar. Grid 1 kolom
// mobile, 3 kolom ≥sm (mobile-first, CLAUDE.md).
func regionSearchTreePanel(searchURL, treeURL string) g.Node {
	group := regionSearchTreeGroup
	fieldSelect := func(label, level string) g.Node {
		id := "f-tree-l" + level + "-" + group
		return h.Div(
			h.Class("grid gap-1 min-w-0"),
			h.Label(h.For(id), h.Class("label text-xs"), g.Text(label)),
			h.Select(
				h.ID(id), h.Class("select select-sm text-base w-full"), h.Disabled(),
				g.Attr("data-tree-level", level),
				h.Option(h.Value(""), g.Text("— Pilih "+label+" —")),
			),
		)
	}
	districtID := "f-tree-l3-" + group
	districtSelect := h.Div(
		h.Class("grid gap-1 min-w-0"),
		h.Label(h.For(districtID), h.Class("label text-xs"), g.Text("Kecamatan")),
		h.Select(
			h.ID(districtID), h.Class("select select-sm text-base w-full"), h.Disabled(),
			g.Attr("data-tree-level", "3"),
			data.On("change", "el.value && @post('"+searchURL+"?district='+encodeURIComponent(el.value))"),
			h.Option(h.Value(""), g.Text("— Pilih Kecamatan —")),
		),
	)
	return h.Div(
		g.Attr("data-tree-group", group),
		g.Attr("data-tree-url", treeURL),
		h.Class("grid grid-cols-1 sm:grid-cols-3 gap-2"),
		fieldSelect("Provinsi", "1"),
		fieldSelect("Kabupaten/Kota", "2"),
		districtSelect,
	)
}

// RegionSearchResults merender fragmen hasil (dipakai state AWAL modal MAUPUN
// patch SSE dari handler — kontrak id SAMA agar PatchElements menimpa di
// tempat, dari tab "kode" MAUPUN tab "Wilayah"). rows kosong → pesan bantu:
// emptyHint bila diisi (handler sudah mencari tapi nihil, lihat
// RegionSearchHintNoResults), atau pesan default "belum cukup ketikan" bila
// emptyHint "".
func RegionSearchResults(rows []RegionSearchRow, emptyHint string) g.Node {
	if len(rows) == 0 {
		msg := emptyHint
		if msg == "" {
			msg = regionSearchHintTooShort
		}
		return h.Div(
			h.ID(regionSearchResultsID),
			h.Class("text-sm text-base-content/60 py-2"),
			g.Text(msg),
		)
	}
	return h.Div(
		h.ID(regionSearchResultsID),
		TableScroll(h.Table(
			h.Class("table table-sm"),
			h.THead(h.Tr(
				h.Th(g.Text("Kode")),
				h.Th(g.Text("Desa")),
				h.Th(g.Text("Kecamatan")),
				h.Th(g.Text("Kabupaten/Kota")),
				h.Th(g.Text("Provinsi")),
			)),
			h.TBody(g.Map(rows, regionSearchRowNode)),
		)),
	)
}

func regionSearchRowNode(row RegionSearchRow) g.Node {
	desa := row.Desa
	if desa == "" {
		desa = "-"
	}
	return h.Tr(
		h.Td(h.Class("font-mono text-xs"), g.Text(row.Code)),
		h.Td(g.Text(desa)),
		h.Td(g.Text(row.Kecamatan)),
		h.Td(g.Text(row.Kabupaten)),
		h.Td(g.Text(row.Provinsi)),
	)
}
