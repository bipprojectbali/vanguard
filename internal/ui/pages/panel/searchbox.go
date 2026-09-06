package panel

import (
	"net/url"
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// searchbox.go — kotak pencarian bebas (?q=) BERSAMA untuk daftar Sales (BL-6):
// leads, deals (tabel), kontak global, quotes. Satu bentuk form agar keempat
// daftar konsisten & tak menduplikasi markup.
//
// Navigasi native GET (bookmarkable + reload penuh, lolos CSP gotcha #16), BUKAN
// Datastar. Submit query baru MENGATUR ULANG ke halaman 1 — form sengaja TAK
// membawa ?after=, jadi cursor lama tak ikut. Nilai q di-echo balik lewat
// h.Value → auto-escape gomponents (aman gotcha #15).
//
// Mobile-first: input text-base (≥16px → iOS tak auto-zoom), tap target min-h-11
// (≥44px), flex-wrap + min-w-0 agar tak meluber di viewport 375px.

// hiddenField = satu <input type=hidden> yang DIPERTAHANKAN lintas submit search
// (mis. tab/view/stage aktif) supaya konteks daftar tak hilang saat mencari.
// Value "" dilewati (tak merender field kosong / URL kanonik).
type hiddenField struct{ Name, Value string }

// searchBox merender form GET pencarian. action = URL daftar (tanpa query); q =
// nilai saat ini; placeholder + ariaLabel menyesuaikan entitas; keep = field
// tersembunyi yang dipertahankan (tab/view/stage). Tanpa tombol "Reset": ikon X
// bawaan input type=search sudah mengosongkan kata kunci; tekan "Cari" saat
// kosong → daftar kembali tak tersaring.
func searchBox(action, q, placeholder, ariaLabel string, keep ...hiddenField) g.Node {
	fields := []g.Node{
		h.Input(
			h.Type("search"), h.Name("q"), h.Value(q),
			h.Placeholder(placeholder),
			h.Class("input input-bordered text-base w-full sm:max-w-xs min-h-11 min-w-0"),
			g.Attr("aria-label", ariaLabel),
		),
	}
	for _, f := range keep {
		if f.Value == "" {
			continue
		}
		fields = append(fields, h.Input(h.Type("hidden"), h.Name(f.Name), h.Value(f.Value)))
	}
	fields = append(fields, h.Button(
		h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text("Cari"),
	))
	// Tak ada tombol "Reset": input type=search sudah punya ikon X bawaan untuk
	// mengosongkan kata kunci; kosongkan lalu tekan "Cari" → daftar tak tersaring.
	return h.Form(
		h.Method("get"), h.Action(action),
		h.Class("flex flex-wrap items-center gap-2 min-w-0"),
		g.Group(fields),
	)
}

// tabSearchRow menyejajarkan bilah tab (kiri) dengan kotak cari yang terdorong ke
// pojok kanan (justify-between) dalam SATU baris (BL-68), meniru pola acuan
// accounts.go. Sebelumnya tiap daftar menaruh search di baris terpisah full-width
// di bawah tab → tampak tak seragam. flex-wrap → di 375px kotak cari TURUN ke
// baris bawah alih-alih meluber; min-w-0 cegah overflow horizontal.
func tabSearchRow(tablist, search g.Node) g.Node {
	return h.Div(
		h.Class("flex flex-wrap items-center justify-between gap-2 min-w-0"),
		tablist, search,
	)
}

// withQuery merakit "action" + query string dari keep (field tak kosong) + q
// (bila tak kosong), nilai di-escape lewat url.Values.Encode. Dipakai tautan
// Reset (q kosong, keep bertahan) & bisa dipakai ulang pager/empty-state agar
// q + konteks (tab/view/stage) bertahan antar halaman keyset. Tanpa param apa
// pun → action polos (URL kanonik).
func withQuery(action, q string, keep ...hiddenField) string {
	vals := url.Values{}
	for _, f := range keep {
		if f.Value != "" {
			vals.Set(f.Name, f.Value)
		}
	}
	if q != "" {
		vals.Set("q", q)
	}
	if len(vals) == 0 {
		return action
	}
	return action + "?" + vals.Encode()
}

// appendQuery menambahkan &q=<escaped> ke href yang SUDAH punya query string
// (mis. pager: "...?after=CURSOR&tab=my"). Kosong → href apa adanya. Dipakai
// pager/empty-state yang merakit href-nya sendiri (bukan lewat withQuery) agar
// q ikut bertahan ke halaman berikutnya / kembali-ke-awal.
func appendQuery(href, q string) string {
	if q == "" {
		return href
	}
	return href + "&q=" + url.QueryEscape(q)
}

// panelListHref merakit URL kanonik daftar untuk baseHref ui.KeysetPager (BL-7):
// path diikuti param filter non-kosong dengan pemisah ?/& yang benar, TANPA
// after/trail (ditambah helper pager sendiri). Nilai di-escape; berbeda dari
// appendQuery yang mengasumsikan href sudah punya "?". Bila path sudah berparam
// (mis. ".../reports?section=x"), param berikutnya otomatis pakai &.
func panelListHref(path string, params ...[2]string) string {
	out := path
	for _, p := range params {
		if p[1] == "" {
			continue
		}
		sep := "?"
		if strings.Contains(out, "?") {
			sep = "&"
		}
		out += sep + p[0] + "=" + url.QueryEscape(p[1])
	}
	return out
}
