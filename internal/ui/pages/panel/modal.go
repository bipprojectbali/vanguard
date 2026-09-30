package panel

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// modal.go — modal CSP-safe pola daisyUI checkbox-toggle (BL-70): murni CSS/HTML,
// NOL JavaScript (tak bergantung Datastar/runtime) → paling tahan terhadap CSP
// (§gotcha 1/16, larangan sse.Redirect/inline). Pemicu = <label for=id>;
// visibilitas dikendalikan <input type=checkbox class=modal-toggle> tersembunyi;
// form di dalam modal-box tetap NATIVE POST → 303 (gotcha #16) → modal tertutup
// sendiri saat halaman reload. Class daisyUI (modal/modal-toggle/modal-box/
// modal-backdrop) di-tree-shake `make css` karena dipakai di .go ini (gotcha #4).

// modalTrigger = tombol pembuka modal berbentuk <label for=id> (BUKAN <button>:
// label menyalakan checkbox tersembunyi tanpa JS). class bebas untuk styling btn;
// sertakan min-h-11 agar tap target ≥44px (mobile-first).
func modalTrigger(id, label, class string) g.Node {
	return h.Label(h.For(id), h.Class(class), g.Text(label))
}

// modalDialog = checkbox pengendali + kotak modal daisyUI (keduanya SIBLING agar
// selektor CSS daisyUI mengikat). content = isi modal-box (mis. form native POST).
// Tombol ✕ & backdrop menutup via label ber-for yang sama. Sisipkan sekali di
// halaman; pasangkan dengan modalTrigger(id, …). modal-box responsif
// (w-11/12 max-w-lg) + min-w-0 → nol overflow di 375px.
func modalDialog(id, title string, content g.Node) g.Node {
	return g.Group([]g.Node{
		h.Input(h.Type("checkbox"), h.ID(id), h.Class("modal-toggle")),
		h.Div(
			h.Class("modal"), g.Attr("role", "dialog"),
			h.Div(
				h.Class("modal-box w-11/12 max-w-lg min-w-0"),
				h.Label(
					h.For(id), g.Attr("aria-label", "Tutup"),
					h.Class("btn btn-circle btn-ghost absolute right-2 top-2"),
					g.Text("✕"),
				),
				h.H3(h.Class("font-semibold text-lg mb-3 pr-8"), g.Text(title)),
				content,
			),
			h.Label(h.Class("modal-backdrop"), h.For(id), g.Text("Tutup")),
		),
	})
}

// popover — varian modal & menu berbasis Popover API native (HTML murni, NOL JS,
// CSP-safe). Dipakai di dalam <table>/TableScroll: popover tampil di TOP LAYER
// sehingga tak terpotong overflow-x-auto (menu absolut biasa akan terpotong).
// Membuka satu popover auto menutup popover auto lain yang bukan leluhurnya →
// dialog HARUS sibling menu, bukan anak. Esc & klik-luar menutup bawaan browser.
// JANGAN beri utilitas display (grid/flex) pada elemen popover itu sendiri —
// menimpa display:none saat tertutup; taruh di wrapper dalam.

// popoverTrigger = <button popovertarget=id>. extra = atribut tambahan.
func popoverTrigger(id, label, class string, extra ...g.Node) g.Node {
	return h.Button(append([]g.Node{
		h.Type("button"), h.Class(class), g.Attr("popovertarget", id), g.Text(label),
	}, extra...)...)
}

// popoverMenu = menu mengambang di bawah pemicunya (posisi: .row-menu di
// static/input.css; tanpa dukungan anchor positioning jatuh ke tengah layar).
func popoverMenu(id string, items ...g.Node) g.Node {
	return h.Div(
		h.ID(id), g.Attr("popover"),
		h.Class("row-menu w-48 rounded-box border border-base-300 bg-base-100 text-base-content overflow-hidden shadow-lg"),
		h.Div(h.Class("grid min-w-0"), g.Group(items)),
	)
}

// popoverDialog = dialog modal-like (tengah layar + backdrop) berisi form native
// POST → 303 (gotcha #16). Mobile-first: w-11/12 max-w-lg min-w-0, tinggi dibatasi.
func popoverDialog(id, title string, content g.Node) g.Node {
	return h.Div(
		h.ID(id), g.Attr("popover"), g.Attr("role", "dialog"),
		h.Class("m-auto w-11/12 max-w-lg min-w-0 max-h-[90dvh] rounded-box border border-base-300 bg-base-100 text-base-content p-6 shadow-xl backdrop:bg-black/50"),
		h.Button(h.Type("button"), g.Attr("popovertarget", id), g.Attr("popovertargetaction", "hide"),
			g.Attr("aria-label", "Tutup"),
			h.Class("btn btn-circle btn-ghost absolute right-2 top-2"), g.Text("✕")),
		h.H3(h.Class("font-semibold text-lg mb-3 pr-8"), g.Text(title)),
		content,
	)
}
