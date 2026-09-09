package panel

import (
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// accounts_form_fields.go — primitif field form generik (formCard, field,
// phoneField, textareaField, selectField, hint/wrap), dipisah
// dari accounts_form.go agar tiap file di bawah ambang tipe View/Component (300).
// Dipakai bersama oleh view lain (sales_quotes_detail dsb) — satu paket.

// formCard = satu kelompok field dalam kartu. Grid 1-kolom di mobile → 2 di sm
// ke atas (mobile-first).
func formCard(title string, fields ...g.Node) g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-2"), g.Text(title)),
			h.Div(h.Class("grid gap-3 sm:grid-cols-2 min-w-0"), g.Group(fields)),
		),
	)
}

// labelWithHint = label field + (opsional) ikon ⓘ tap-friendly (BL-65).
// Tanpa hint → label biasa (labelFor). Dengan hint → seluruh baris label
// dibungkus <details class="hint-reveal"> (CSP-safe, tanpa JS inline): summary
// memuat label + ikon ⓘ, dan keterangan muncul INLINE penuh-lebar di bawahnya
// saat di-TAP/klik/Enter — BUKAN hover (hover mati di sentuh; ini menjawab
// keberatan enum_field.go BL-3/BL-4 terhadap tooltip/title). Karena keterangan
// mengalir (bukan absolut) ia membungkus teks & tak pernah meluber di 375px.
// Menggantikan (a) baris fieldHint lama di bawah input dan (b) pelebaran
// sm:col-span-2 field ber-hint — sekarang semua field tetap single-column
// seragam. Tap-target ikon ≥44px (min-h-11 min-w-11).
func labelWithHint(text, forID string, required bool, hint []string) g.Node {
	if len(hint) == 0 || hint[0] == "" {
		return labelFor(text, forID, required)
	}
	return h.Details(
		h.Class("hint-reveal min-w-0"),
		h.Summary(
			// flex → display:flex (bukan list-item) sekaligus menghapus segitiga
			// disclosure bawaan; .hint-reveal (input.css) menutup sisa marker.
			h.Class("hint-summary flex items-center gap-1 min-w-0 cursor-pointer"),
			labelFor(text, forID, required),
			h.Span(
				// min-h/w-11 = tap-target 44px; -my-2 tarik kembali agar baris
				// label tak menggemuk (area klik tetap 44px, meluap ke margin).
				h.Class("inline-flex items-center justify-center min-h-11 min-w-11 -my-2 shrink-0 text-base-content/50"),
				g.Attr("aria-hidden", "true"),
				lucide.Info(h.Class("size-4")),
			),
		),
		h.P(
			h.Class("mt-1 text-xs font-normal text-base-content/70 break-words"),
			g.Text(hint[0]),
		),
	)
}

// field = satu input teks/angka. required menandai wajib (jaring klien; backend
// tetap memvalidasi). text-base (≥16px) agar iOS tak auto-zoom saat fokus. hint
// (opsional, variadic) = penjelasan singkat utk field yang maknanya tak jelas
// hanya dari label (mis. beda Teritori vs Kabupaten/Kota administratif); kini
// tampil sebagai ikon ⓘ tap-friendly di label (labelWithHint), bukan baris teks.
func field(label, name, val string, required bool, typ string, hint ...string) g.Node {
	attrs := []g.Node{
		h.ID("f-" + name), h.Name(name), h.Type(typ),
		h.Value(val), h.Class("input text-base w-full"),
	}
	if required {
		attrs = append(attrs, h.Required())
	}
	if typ == "number" {
		attrs = append(attrs, g.Attr("min", "0"))
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelWithHint(label, "f-"+name, required, hint),
		ui.Input(attrs...),
	)
}

// dateFieldMin = input tanggal opsional dgn atribut `min` (batas bawah klien).
// field() generik tak menyetel `min` untuk type=date (hanya auto `min=0` utk
// number) → helper kecil ini menutup celah tanpa membebani semua pemanggil field.
// Dipakai BL-17: min = hari ini agar picker tak menawarkan tanggal lampau (jaring
// klien; parseQuoteForm tetap penjaga backend). min kosong = tanpa batas.
func dateFieldMin(label, name, val, min string) g.Node {
	attrs := []g.Node{
		h.ID("f-" + name), h.Name(name), h.Type("date"),
		h.Value(val), h.Class("input text-base w-full"),
	}
	if min != "" {
		attrs = append(attrs, g.Attr("min", min))
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelFor(label, "f-"+name, false),
		ui.Input(attrs...),
	)
}

// phoneField = HP Kontak desa. BL-106: field biasa — HP Kontak account tak lagi
// ber-FLS (semua yang boleh melihat desa boleh lihat & sunting nomor penuh).
// Mask/kunci per-peran tinggal di modul Kontak & Lead, bukan di sini.
//
// Input ANGKA (pola sama HP/WhatsApp Lead): inputmode=numeric (keypad angka
// mobile) + data-phonenum (phonenum.js membuang huruf/simbol SAAT diketik) +
// pattern jaring klien; backend optPhone penjaga sesungguhnya. editable=true
// karena account tak pernah mengunci HP (BL-106).
func phoneField(val string) g.Node {
	return phoneNumField("HP Kontak", "contact_phone", val, true)
}

// textareaField = teks bebas panjang (deskripsi). Lebar penuh (rentang 2 kolom).
// extra = atribut tambahan opsional pada <textarea> (mis. data.Attr("required",…)
// utk wajib-kondisional); pemanggil 3-arg lama tak berubah.
func textareaField(label, name, val string, extra ...g.Node) g.Node {
	attrs := []g.Node{
		h.ID("f-" + name), h.Name(name), h.Class("textarea text-base w-full"),
		h.Rows("3"),
	}
	attrs = append(attrs, extra...)
	attrs = append(attrs, g.Text(val))
	return h.Div(
		h.Class("grid gap-1 min-w-0 sm:col-span-2"),
		labelFor(label, "f-"+name, false),
		h.Textarea(attrs...),
	)
}

// selectField = dropdown enum. Opsi kosong "—" hanya bila tak wajib (nilai
// opsional boleh dikosongkan = NULL). hint (opsional, variadic) = penjelasan
// arti tiap opsi utk enum yang nilainya bukan kata umum (mis. IDM) — kini
// tampil sebagai ikon ⓘ tap-friendly di label (labelWithHint), bukan baris teks
// di bawah select yang dulu memaksa field melebar sm:col-span-2. Field tetap
// single-column seragam. Backward-compatible: pemanggil tanpa hint tak berubah.
func selectField(label, name, current string, opts []string, required bool, hint ...string) g.Node {
	sel := []g.Node{
		h.ID("f-" + name), h.Name(name), h.Class("select text-base w-full"),
	}
	if required {
		sel = append(sel, h.Required())
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelWithHint(label, "f-"+name, required, hint),
		h.Select(append(sel, g.Group(enumOptions(current, opts, !required)))...),
	)
}

// enumOptions membangun daftar <option> untuk dropdown enum. blank=true (field
// opsional) menambah opsi "—" bernilai kosong di depan; opsi yang == current
// ditandai Selected. Diekstrak agar selectField & enumField (legenda BL-3/BL-4)
// merakit markup opsi dari SATU tempat — tak bercabang jadi dua kebenaran.
func enumOptions(current string, opts []string, blank bool) []g.Node {
	nodes := make([]g.Node, 0, len(opts)+1)
	if blank {
		nodes = append(nodes, h.Option(h.Value(""), g.Text("—")))
	}
	for _, o := range opts {
		attrs := []g.Node{h.Value(o)}
		if o == current {
			attrs = append(attrs, h.Selected())
		}
		nodes = append(nodes, h.Option(append(attrs, g.Text(o))...))
	}
	return nodes
}
