package panel

import (
	"go_starter/internal/ui"

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

// fieldHint = baris penjelasan opsional di bawah input, dipanggil field/
// selectField saat hint diisi. Variadic pada pemanggil agar dropdown pemanggil
// lama (mayoritas field self-explanatory) tak perlu ikut berubah — kirim ""
// atau tak sama sekali = tanpa hint, sama seperti pola phoneField sebelumnya.
func fieldHint(hint []string) g.Node {
	if len(hint) == 0 || hint[0] == "" {
		return g.Text("")
	}
	return h.P(h.Class("text-xs text-base-content/60"), g.Text(hint[0]))
}

// fieldWrapClass menentukan lebar grid satu field di formCard (sm:grid-cols-2).
// Field TANPA hint tetap setengah lebar (dua per baris, seperti semula). Field
// BER-hint dilebarkan penuh (sm:col-span-2) — sebelum ini, field ber-hint yang
// kebetulan berpasangan dgn field tanpa hint pada baris grid yang sama (mis.
// "Kode Pos" vs "Teritori", "Nama Desa" vs "Tipe Akun") membuat baris itu
// tampak timpang: sel bertetangga jauh lebih pendek, menyisakan ruang kosong
// di bawahnya karena tinggi baris grid mengikuti sel tertinggi. Melebarkan
// penuh field ber-hint memutus pasangannya dgn field tak terkait sehingga tiap
// baris tetap rata — pola yang sama dgn textareaField yang sudah lebih dulu
// full-width.
func fieldWrapClass(hint []string) string {
	if len(hint) > 0 && hint[0] != "" {
		return "grid gap-1 min-w-0 sm:col-span-2"
	}
	return "grid gap-1 min-w-0"
}

// field = satu input teks/angka. required menandai wajib (jaring klien; backend
// tetap memvalidasi). text-base (≥16px) agar iOS tak auto-zoom saat fokus. hint
// (opsional, variadic) = penjelasan singkat utk field yang maknanya tak jelas
// hanya dari label (mis. beda Teritori vs Kabupaten/Kota administratif). Field
// ber-hint dilebarkan penuh (fieldWrapClass) — lihat komentarnya soal alasan.
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
		h.Class(fieldWrapClass(hint)),
		labelFor(label, "f-"+name, required),
		ui.Input(attrs...),
		fieldHint(hint),
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

// phoneField = HP kontak. Bila tak boleh disunting (bukan Sales), field dikunci
// menampilkan nilai tersamar + keterangan, dan TANPA name agar tak terkirim —
// handler juga mempertahankan nomor asli, jadi mask tak pernah menimpa data.
func phoneField(editable bool, val string) g.Node {
	if editable {
		return field("HP Kontak", "contact_phone", val, false, "tel")
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelFor("HP Kontak", "f-contact_phone_ro", false),
		ui.Input(
			h.ID("f-contact_phone_ro"), h.Type("tel"), h.Value(val),
			h.Disabled(), h.Class("input text-base w-full"),
		),
		h.P(h.Class("text-xs text-base-content/60"),
			g.Text("Nomor disamarkan & hanya bisa disunting oleh Sales.")),
	)
}

// textareaField = teks bebas panjang (deskripsi). Lebar penuh (rentang 2 kolom).
func textareaField(label, name, val string) g.Node {
	return h.Div(
		h.Class("grid gap-1 min-w-0 sm:col-span-2"),
		labelFor(label, "f-"+name, false),
		h.Textarea(
			h.ID("f-"+name), h.Name(name), h.Class("textarea text-base w-full"),
			h.Rows("3"), g.Text(val),
		),
	)
}

// selectField = dropdown enum. Opsi kosong "—" hanya bila tak wajib (nilai
// opsional boleh dikosongkan = NULL). hint (opsional, variadic) = penjelasan
// arti tiap opsi utk enum yang nilainya bukan kata umum (mis. IDM) — lihat
// fieldHint. Backward-compatible: pemanggil existing yang tak lewat hint tak
// perlu ikut berubah.
func selectField(label, name, current string, opts []string, required bool, hint ...string) g.Node {
	sel := []g.Node{
		h.ID("f-" + name), h.Name(name), h.Class("select text-base w-full"),
	}
	if required {
		sel = append(sel, h.Required())
	}
	return h.Div(
		h.Class(fieldWrapClass(hint)),
		labelFor(label, "f-"+name, required),
		h.Select(append(sel, g.Group(enumOptions(current, opts, !required)))...),
		fieldHint(hint),
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
