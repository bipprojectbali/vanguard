package ui

import (
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// When merender node hanya bila allowed; selainnya node kosong. Untuk render
// kondisional berbasis izin (mis. tombol admin) — flag di-PRECOMPUTE di handler
// (authz.Can), bukan dipanggil dari dalam fungsi gomponents. Ini kosmetik UI;
// pertahanan sebenarnya tetap di middleware/service.
func When(allowed bool, node g.Node) g.Node {
	if allowed {
		return node
	}
	return g.Text("")
}

// ConfirmModal = dialog konfirmasi reusable via Datastar signal. Tampil saat
// signal $<sig> true. Tombol "Ya" menjalankan confirmAction (ekspresi Datastar,
// mis. "@post('/logout')") lalu menutup; "Batal"/backdrop menutup.
//
// Pasangkan dengan tombol pemicu yang men-set signal true (lihat ConfirmTrigger).
// signal harus unik per halaman. Sertakan modal ini SEKALI di halaman.
// ConfirmModal merender dialog konfirmasi. Tombol konfirmasi = NATIVE form POST
// ke confirmPostURL (bukan Datastar @post): aksinya = NAVIGASI (mis. logout →
// redirect 303), dan redirect via SSE menyuntik <script> yang diblokir CSP proyek.
// Tombol Batal menutup modal via signal. sig = nama signal boolean pengendali.
func ConfirmModal(sig, title, message, confirmLabel, confirmPostURL string) g.Node {
	openExpr := "$" + sig
	return h.Div(
		// Overlay + backdrop. Inline display:none agar tak FOUC sebelum Datastar.
		h.Class("fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"),
		g.Attr("style", "display:none"),
		data.Show(openExpr),
		data.On("click", openExpr+" = false"), // klik backdrop → tutup
		// Kartu dialog. stop-propagation agar klik di dalam tak menutup.
		h.Div(
			h.Class("card bg-base-100 shadow-lg w-full max-w-sm"),
			data.On("click", "evt.stopPropagation()"),
			h.Div(
				h.Class("card-body gap-4"),
				h.H2(h.Class("text-lg font-semibold"), g.Text(title)),
				h.P(h.Class("text-sm text-base-content/70"), g.Text(message)),
				h.Div(
					h.Class("flex justify-end gap-2"),
					h.Button(
						h.Type("button"), h.Class("btn btn-outline btn-sm"),
						data.On("click", openExpr+" = false"),
						g.Text("Batal"),
					),
					// Konfirmasi = native form submit → HTTP 303 (navigasi penuh).
					h.FormEl(
						h.Method("post"), h.Action(confirmPostURL),
						h.Button(
							h.Type("submit"), h.Class("btn btn-error btn-sm"),
							g.Text(confirmLabel),
						),
					),
				),
			),
		),
	)
}

// ConfirmTrigger membungkus atribut untuk tombol yang MEMBUKA ConfirmModal:
// klik → set signal true. Sisipkan hasilnya sebagai atribut tombol.
func ConfirmTrigger(sig string) g.Node {
	return data.On("click", "$"+sig+" = true")
}

// Variant adalah tipe typed untuk varian komponen — typo jadi compile error,
// bukan string kosong senyap (§4.5).
type Variant int

const (
	VariantDefault Variant = iota
	VariantDestructive
	VariantOutline
	VariantGhost
	VariantWarning
)

// btnVariant memetakan varian ke class modifier daisyUI. daisyUI memakai class
// tambahan (btn-primary, btn-error, ...) di samping "btn" — BUKAN atribut
// data-variant Basecoat. Array indeks enum: menambah Variant tanpa entri =
// compile error. Default = btn-primary (daisyUI: "btn" polos = netral, jadi
// kita eksplisitkan primary agar setara "default" Basecoat lama).
var btnVariant = [...]string{
	VariantDefault:     "btn-primary",
	VariantDestructive: "btn-error",
	VariantOutline:     "btn-outline",
	VariantGhost:       "btn-ghost",
	VariantWarning:     "btn-warning",
}

// Button merender tombol daisyUI: class "btn" + modifier varian. Atribut
// tambahan (mis. data-on Datastar) dilewatkan lewat attrs.
func Button(variant Variant, attrs []g.Node, children ...g.Node) g.Node {
	base := []g.Node{h.Class("btn " + btnVariant[variant])}
	return h.Button(append(base, append(attrs, g.Group(children))...)...)
}

// Card membungkus konten dalam kartu daisyUI. `.card` = kontainer flex; padding
// & gap antar-anak datang dari `.card-body` (yang WAJIB membungkus isi). bg
// eksplisit (bg-base-100) + border agar kontras dari latar halaman.
func Card(children ...g.Node) g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(append([]g.Node{h.Class("card-body gap-6")}, children...)...),
	)
}

// TableScroll membungkus <table> agar scroll horizontalnya TERKURUNG, tak
// mendorong lebar halaman di mobile (konvensi mobile-first).
//
// Dua hal WAJIB bersamaan, dan keduanya mudah lupa (terukur: tanpa ini halaman
// members meluber 439px di viewport 375px):
//  1. overflow-x-auto pada pembungkus LANGSUNG tabel — bukan pada `.card-body`.
//     `.card-body` flex-col; overflow di sana tak menahan min-content tabel.
//  2. min-w-0 — kartu/flex-item default `min-width:auto` sehingga MENOLAK
//     menyusut di bawah min-content anaknya; overflow jadi mubazir.
//
// Pakai helper ini, jangan tulis `overflow-x-auto` manual di card-body.
func TableScroll(table g.Node) g.Node {
	return h.Div(h.Class("overflow-x-auto min-w-0 w-full"), table)
}

// Input adalah field teks daisyUI. `.input` daisyUI sudah punya border + tinggi;
// tambahkan w-full agar field mengisi lebar kontainer (default daisyUI = auto).
// attrs untuk data-bind, placeholder, dll.
func Input(attrs ...g.Node) g.Node {
	return h.Input(append([]g.Node{h.Class("input w-full"), h.Type("text")}, attrs...)...)
}

// Label untuk field form. daisyUI `.label` = inline-flex wrapper; untuk teks
// label mandiri cukup class-nya (font & warna diwarisi).
func Label(text string, attrs ...g.Node) g.Node {
	return h.Label(append([]g.Node{h.Class("label")}, append(attrs, g.Text(text))...)...)
}

// AlertSlot merender wadah error KOSONG (tanpa class .alert) sebagai target
// patch Datastar. Penting: `.alert` selalu punya padding/warna, jadi merender
// .alert saat kosong menghasilkan kotak hantu. Slot ini hanya <div id> kosong;
// isi diganti Alert() lengkap saat server mem-patch error.
func AlertSlot(id string) g.Node {
	return h.Div(h.ID(id))
}

// Alert menampilkan pesan (mis. error validasi, atau peringatan non-blocking
// spt kandidat desa duplikat). daisyUI: class "alert" + modifier warna
// (alert-error untuk destructive, alert-warning untuk peringatan lunak). Punya
// id (sama dengan slot) agar patch outer menggantikan slot kosong dgn alert
// berisi.
func Alert(variant Variant, id string, children ...g.Node) g.Node {
	cls := "alert"
	switch variant {
	case VariantDestructive:
		cls = "alert alert-error"
	case VariantWarning:
		cls = "alert alert-warning"
	}
	attrs := []g.Node{h.ID(id), h.Class(cls), h.Role("alert")}
	return h.Div(append(attrs, g.Group(children))...)
}
