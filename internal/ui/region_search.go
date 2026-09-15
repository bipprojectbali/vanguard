package ui

// region_search.go — BL-163: modal global "Cari Kode Desa/Kecamatan". Dirender
// SEKALI di AppShell (pola sama ConfirmModal/ChangelogModal: signal Datastar
// boolean, inline display:none anti-FOUC), dipicu dari MANA PUN via
// RegionSearchTrigger (mis. form Akun, halaman impor CSV — lihat call site).
// Pencarian live: input ter-debounce men-@post fragmen hasil (SearchRegions{
// ByCode,ByName} di handler regions_search.go) — modal TETAP TERBUKA sepanjang
// round-trip (SSE PatchElements fragmen, bukan navigasi; lolos gotcha #16).

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

// RegionSearchMinChars = panjang minimum ketikan sebelum live-search jalan
// (keputusan desain BL-163). Dipakai DUA sisi: expr client (skip @post di
// bawah panjang ini — tak ada request terkirim sama sekali) dan guard server
// (regions_search.go, pertahanan jika client dilewati/dimanipulasi).
const RegionSearchMinChars = 3

// regionSearchResultsID = id kontainer hasil yang di-SSE-patch oleh handler.
const regionSearchResultsID = "region-search-results"

// RegionSearchHintNoResults = pesan kontainer hasil saat sudah cari (≥
// RegionSearchMinChars karakter) tapi nihil — DIBEDAKAN dari pesan default
// "belum cukup ketikan" (RegionSearchResults tanpa hint eksplisit) agar user
// yang sudah mengetik cukup tak dikira belum mengetik apa-apa.
const RegionSearchHintNoResults = "Tidak ada hasil ditemukan."

// regionSearchHintTooShort = pesan default kontainer hasil (belum ada
// pencarian / ketikan < RegionSearchMinChars). var (bukan const) karena
// dirakit dari RegionSearchMinChars via strconv — satu sumber kebenaran,
// tak ada angka "3" terduplikasi di string literal.
var regionSearchHintTooShort = "Ketik minimal " + strconv.Itoa(RegionSearchMinChars) + " karakter untuk mencari."

// RegionSearchRow = SATU baris hasil pencarian, bentuk TAMPILAN murni (handler
// memetakan dari db.SearchRegionsBy{Code,Name}Row) — Desa "" menandakan baris
// Kecamatan (level 3, tak punya desa), dirender "-".
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
// (signal + backdrop + stop-propagation kartu dalam), body berisi input
// live-search + kontainer hasil yang di-patch handler. wsBase = prefix
// workspace aktif ("/w/{slug}", dari ShellData.WSBase) — endpoint
// /regions/search ter-NEST di bawah rute workspace (routes.go), BUKAN path
// absolut; tanpa prefix ini @post 404 diam-diam (bug ditemukan user: ketikan
// tak memicu perubahan apa pun karena request tak pernah sampai ke handler).
func RegionSearchModal(wsBase string) g.Node {
	openExpr := "$" + regionSearchSignal
	searchURL := wsBase + "/regions/search"
	minChars := strconv.Itoa(RegionSearchMinChars)
	// Guard panjang di EKSPRESI sendiri (bukan cuma placeholder): && pendek-
	// sirkuit — di bawah minChars, @post tak pernah dievaluasi = tak ada
	// request terkirim (kasus uji "<3 karakter no-request").
	inputExpr := "el.value.trim().length >= " + minChars +
		" && @post('" + searchURL + "?q='+encodeURIComponent(el.value.trim()))"
	return h.Div(
		h.Class("fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"),
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
			// Body: input + hasil (scroll).
			h.Div(
				h.Class("px-5 py-4 flex flex-col gap-3 overflow-y-auto min-w-0"),
				h.Input(
					h.Type("text"), h.Name("q"),
					h.Class("input input-bordered w-full text-base"),
					h.Placeholder("Ketik kode (mis. 32.01.01) atau nama Kecamatan/Desa, minimal "+minChars+" karakter…"),
					g.Attr("autocomplete", "off"),
					data.On("input", inputExpr, data.ModifierDebounce, data.Duration(400*time.Millisecond)),
				),
				RegionSearchResults(nil, ""),
			),
		),
	)
}

// RegionSearchResults merender fragmen hasil (dipakai state AWAL modal MAUPUN
// patch SSE dari handler — kontrak id SAMA agar PatchElements menimpa di
// tempat). rows kosong → pesan bantu: emptyHint bila diisi (handler sudah
// mencari tapi nihil, lihat RegionSearchHintNoResults), atau pesan default
// "belum cukup ketikan" bila emptyHint "".
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
