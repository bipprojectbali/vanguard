package panel

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// enum_field.go — primitif form untuk enum yang MENJELASKAN dirinya: dropdown
// enum beserta legenda makna tiap opsi (enumField, BL-3) & checkbox beserta
// keterangan efeknya (checkboxHintField, BL-4). Enum/flag polos tak bercerita:
// pengguna baru tak tahu beda "Contacted" vs "Qualified", atau apa akibat
// mencentang "Jangan hubungi". Keterangan = teks statis di bawah kontrol —
// dipilih ketimbang tooltip/title karena hover mati di sentuh (mobile-first) &
// title tak konsisten lintas browser. Statis (bukan input user) → CSP-safe,
// tanpa JS.
//
// Nilai enum tetap dioper handler (sumber tunggal); legenda hanya memetakan
// nilai→makna sebagai copy UI. Pasangan {nilai, makna} yang dioper HARUS
// himpunan yang sama dengan opsi enum — dijaga uji render tiap form.

// enumField = dropdown enum + legenda makna opsi. Meniru struktur selectField
// (labelFor + Select) tapi dilebarkan penuh (sm:col-span-2) agar legenda punya
// ruang, dan menyisipkan enumLegend di bawah. Merakit <option> lewat enumOptions
// yang SAMA dengan selectField (tak ada dua sumber markup).
func enumField(label, name, current string, opts []string, required bool, legend [][2]string) g.Node {
	sel := []g.Node{
		h.ID("f-" + name), h.Name(name), h.Class("select text-base w-full"),
	}
	if required {
		sel = append(sel, h.Required())
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0 sm:col-span-2"),
		labelFor(label, "f-"+name, required),
		h.Select(append(sel, g.Group(enumOptions(current, opts, !required)))...),
		enumLegend(legend),
	)
}

// enumLegend merender daftar definisi ringkas {istilah: makna} — satu baris per
// opsi, istilah tebal + makna redup. text-xs agar tak mengalahkan field.
func enumLegend(items [][2]string) g.Node {
	rows := make([]g.Node, 0, len(items))
	for _, it := range items {
		rows = append(rows, h.Div(
			h.Class("flex gap-1"),
			h.Span(h.Class("font-medium shrink-0"), g.Text(it[0]+":")),
			h.Span(g.Text(it[1])),
		))
	}
	return h.Div(
		h.Class("mt-0.5 grid gap-0.5 text-xs text-base-content/70"),
		g.Group(rows),
	)
}

// checkboxHintField = satu boolean + keterangan efeknya (BL-4). Checkbox HTML tak
// mengirim apa pun saat tak dicentang, jadi absen = false secara alami
// (parseContactForm.optBool). Tap target ≥44px lewat padding label; hint sejajar
// di bawah teks label (ml-8 ≈ lebar checkbox + gap). Nilai "1" hanya penanda
// kehadiran. hint WAJIB diisi — checkbox yang butuh penjelasan efek adalah alasan
// helper ini ada; boolean tanpa efek non-obvius cukup pakai <label> biasa.
func checkboxHintField(label, name string, checked bool, hint string) g.Node {
	attrs := []g.Node{
		h.ID("f-" + name), h.Name(name), h.Type("checkbox"),
		h.Value("1"), h.Class("checkbox"),
	}
	if checked {
		attrs = append(attrs, h.Checked())
	}
	return h.Div(
		h.Class("grid gap-0.5 min-w-0 sm:col-span-2"),
		h.Label(
			h.For("f-"+name),
			h.Class("flex items-center gap-2 min-h-11 cursor-pointer"),
			h.Input(attrs...),
			h.Span(h.Class("text-sm"), g.Text(label)),
		),
		h.P(h.Class("ml-8 text-xs text-base-content/70"), g.Text(hint)),
	)
}
