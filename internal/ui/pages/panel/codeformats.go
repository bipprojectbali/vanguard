package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// codeformats.go — halaman pengaturan FORMAT kode-unik entitas per workspace
// (mis. DESA-001, DEAL-042). Satu kartu per entitas: prefix, pemisah, lebar
// angka, plus contoh kode berikutnya.
//
// Kenapa satu FORM per entitas (bukan satu form besar): tiap entitas disimpan
// terpisah (UpsertCodeFormat per baris), dan kegagalan validasi satu entitas tak
// boleh membatalkan penyimpanan yang lain. Form native POST → 303 (gotcha #16:
// redirect via SSE menyuntik <script> yang diblokir CSP). Pola sama dengan
// kartu kuota/retensi di /dev/settings.

// CodeFormatItem = data satu baris entitas siap-render. Preview dihitung di
// handler (codes.Format.Render) supaya aturan perakitan tetap satu sumber.
type CodeFormatItem struct {
	Entity    string // nilai enum ("account", "lead", …) — dikirim di form
	Label     string // nama tampilan ("Desa (Akun)")
	Prefix    string
	Separator string
	Padding   int
	Preview   string // contoh kode berikutnya, mis. "DESA-001"
	IsDefault bool   // true = masih pakai bawaan (belum disimpan tenant)
}

// CodeFormatsView = data siap-render halaman format kode.
type CodeFormatsView struct {
	Base         string // prefix URL workspace ("/w/acme") — dari handler, bukan dirakit view
	CanEdit      bool   // hanya pengelola boleh ubah; selain itu hanya lihat
	Items        []CodeFormatItem
	MaxPrefix    int // batas panjang prefix (untuk atribut & keterangan)
	MaxSeparator int
	MinPadding   int
	MaxPadding   int
	Msg          string // pesan sukses (PRG ?ok=)
	Err          string // pesan galat (PRG ?err=)
}

// CodeFormats merender halaman: penjelasan + satu kartu form per entitas.
func CodeFormats(v CodeFormatsView) g.Node {
	body := []g.Node{
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text("Format Kode")),
		h.P(h.Class("text-base-content/70 mb-4"),
			g.Text("Kode-unik ini dipakai untuk mencari dan menyebut data sehari-hari "+
				"(mis. \"cek DESA-014\"). Nomornya berjalan otomatis per workspace, "+
				"mulai dari 1. Mengubah format hanya memengaruhi kode BARU — kode yang "+
				"sudah terbit tetap seperti apa adanya.")),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "codes-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "codes-msg", g.Text(v.Msg)))
	}
	for _, it := range v.Items {
		body = append(body, codeFormatCard(v, it))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// codeFormatCard = satu entitas: judul + contoh + form prefix/pemisah/lebar.
// canEdit=false → field disabled, tanpa tombol Simpan (hanya lihat).
func codeFormatCard(v CodeFormatsView, it CodeFormatItem) g.Node {
	// Header: label entitas + contoh kode berikutnya (badge), agar dampak setiap
	// perubahan terlihat tanpa harus menyimpan dulu untuk sekadar melihat bentuknya.
	header := h.Div(
		h.Class("flex flex-wrap items-center justify-between gap-2 mb-1 min-w-0"),
		h.H2(h.Class("font-semibold"), g.Text(it.Label)),
		h.Span(h.Class("badge badge-neutral font-mono"), g.Text(it.Preview)),
	)

	// Tiga field sejajar di desktop, turun 1 kolom di mobile (mobile-first).
	fields := h.Div(
		h.Class("grid grid-cols-1 sm:grid-cols-3 gap-3 min-w-0"),
		codeField("Prefix", it.Entity+"-prefix", "prefix", it.Prefix,
			g.Attr("maxlength", strconv.Itoa(v.MaxPrefix)), h.Required(), !v.CanEdit),
		codeField("Pemisah", it.Entity+"-separator", "separator", it.Separator,
			g.Attr("maxlength", strconv.Itoa(v.MaxSeparator)), nil, !v.CanEdit),
		codeNumberField("Lebar angka", it.Entity+"-padding", "padding", it.Padding,
			v.MinPadding, v.MaxPadding, !v.CanEdit),
	)

	inner := []g.Node{header, fields}
	if v.CanEdit {
		inner = append(inner,
			h.Div(
				h.Class("flex flex-wrap items-center gap-2 mt-3 min-w-0"),
				h.Button(h.Type("submit"), h.Class("btn btn-primary btn-sm min-h-11"),
					g.Text("Simpan")),
				g.If(it.IsDefault, h.Span(h.Class("text-xs text-base-content/60"),
					g.Text("Masih memakai format bawaan."))),
			),
		)
		// Entity dikirim sebagai hidden field: server memilih baris mana yang
		// di-upsert dari sini, bukan dari URL, jadi satu endpoint melayani semuanya.
		return h.FormEl(
			h.Method("post"), h.Action(v.Base+"/codes"),
			h.Class("card bg-base-100 border border-base-300 min-w-0"),
			h.Input(h.Type("hidden"), h.Name("entity"), h.Value(it.Entity)),
			h.Div(append([]g.Node{h.Class("card-body min-w-0")}, inner...)...),
		)
	}
	// Hanya-lihat: tak ada form/tombol.
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(append([]g.Node{h.Class("card-body min-w-0")}, inner...)...),
	)
}

// codeField = satu field teks berlabel. disabled=true saat hanya-lihat.
func codeField(label, id, name, value string, extra g.Node, req g.Node, disabled bool) g.Node {
	attrs := []g.Node{
		h.ID(id), h.Name(name), h.Type("text"), h.Value(value),
		h.Class("input w-full"), h.AutoComplete("off"),
	}
	if extra != nil {
		attrs = append(attrs, extra)
	}
	if req != nil {
		attrs = append(attrs, req)
	}
	if disabled {
		attrs = append(attrs, h.Disabled())
	}
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		ui.Label(label, h.For(id)),
		h.Input(attrs...),
	)
}

// codeNumberField = field angka lebar (padding), dengan min/max sebagai jaring
// klien; backend tetap memvalidasi ulang (nilai user-controlled).
func codeNumberField(label, id, name string, value, min, max int, disabled bool) g.Node {
	attrs := []g.Node{
		h.ID(id), h.Name(name), h.Type("number"), h.Value(strconv.Itoa(value)),
		g.Attr("min", strconv.Itoa(min)), g.Attr("max", strconv.Itoa(max)),
		h.Required(), h.Class("input w-full"),
	}
	if disabled {
		attrs = append(attrs, h.Disabled())
	}
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		ui.Label(label, h.For(id)),
		h.Input(attrs...),
	)
}
