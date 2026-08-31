package panel

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// lead_enum_field.go — dropdown enum Lead (Status/Rating) BESERTA legenda makna
// tiap opsi (BL-3). Enum tampil polos tak menjelaskan dirinya: pengguna baru tak
// tahu beda "Contacted" vs "Qualified" atau "Hot" vs "Cold". Legenda = daftar
// definisi ringkas di BAWAH dropdown — dipilih ketimbang tooltip/title per
// <option> karena hover mati di sentuh (mobile-first) & title tak konsisten
// lintas browser. Statis (bukan input user) → CSP-safe, tanpa JS.
//
// Nilai enum tetap dioper handler (v.Statuses/v.Ratings, sumber tunggal); legenda
// hanya memetakan nilai→makna sebagai copy UI. Pasangan {nilai, makna} di bawah
// HARUS himpunan yang sama dengan opsi enum — dijaga uji render BL-3.

// leadStatusLegend = makna tiap status lead. Urut = alur kualifikasi (New →
// Contacted → Qualified, atau bercabang ke Unqualified). 'Converted' tak di sini:
// itu status sistem hasil konversi, tak bisa dipilih manual (lihat parseLeadForm).
var leadStatusLegend = [][2]string{
	{"New", "Baru masuk, belum dihubungi."},
	{"Contacted", "Sudah dihubungi, belum dikualifikasi."},
	{"Qualified", "Cocok dan siap dikonversi jadi Deal."},
	{"Unqualified", "Tak cocok atau tak berminat (isi alasannya)."},
}

// leadRatingLegend = makna tiap rating minat. Urut dari paling panas.
var leadRatingLegend = [][2]string{
	{"Hot", "Minat tinggi, siap closing."},
	{"Warm", "Tertarik, perlu tindak lanjut."},
	{"Cold", "Belum tertarik atau prioritas rendah."},
}

// leadEnumField = selectField + legenda makna opsi (BL-3). Meniru struktur
// selectField (labelFor + Select) tapi dilebarkan penuh (sm:col-span-2) agar
// legenda punya ruang, dan menyisipkan enumLegend di bawah. Merakit <option>
// lewat enumOptions yang SAMA dengan selectField (tak ada dua sumber markup).
func leadEnumField(label, name, current string, opts []string, required bool, legend [][2]string) g.Node {
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
