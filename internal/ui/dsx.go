package ui

import (
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// dsx = helper Datastar bertipe. Tujuan: memindah footgun authoring "JS-sebagai-
// string di atribut" dari gagal-senyap-runtime jadi mustahil-ditulis. CHARTER
// (jaga tetap tipis, agent-friendly): tiap helper = SATU jaminan quoting/co-render,
// TANPA logika/DSL. Runtime tetap Datastar (unsafe-eval tetap perlu — itu di luar
// jangkauan generasi string server-side; lihat CLAUDE.md gotcha CSP). Helper ini
// menutup gotcha #5 (data-class hyphen) & #6 (@post butuh <form>) secara struktural.

// ClassOn men-toggle SATU class berdasar ekspresi Datastar. Nama class di-passing
// TELANJANG (tanpa kutip); helper yang mengutipnya. Ini membuat gotcha #5
// (`data-class` key ber-hyphen wajib di-quote) MUSTAHIL ditulis salah: tak ada
// jalur untuk memberi key tanpa kutip. Emit identik dengan
// data.Class("'<class>'", expr) → data-class="{'<class>': <expr>}".
func ClassOn(class, expr string) g.Node {
	return data.Class("'"+class+"'", expr)
}

// Classes = ClassOn untuk banyak class sekaligus. Sama jaminannya (tiap key
// dikutip helper), tanpa peluang lupa mengutip salah satu.
type ClassRule struct {
	Class string // nama class TELANJANG (tanpa kutip)
	Expr  string // ekspresi Datastar (mis. "$open")
}

func Classes(rules ...ClassRule) g.Node {
	pairs := make([]string, 0, len(rules)*2)
	for _, r := range rules {
		pairs = append(pairs, "'"+r.Class+"'", r.Expr)
	}
	return data.Class(pairs...)
}

// PostAction / DeleteAction membangun ekspresi aksi Datastar bertipe. Menghapus
// juggle-kutip manual di call site (auth.go/components.go). Catatan jujur:
// ini MEMPERBAIKI (bukan mustahilkan) site string — hasilnya tetap string, tapi
// bentuknya dijamin benar. Untuk form-valued post, pakai FormPostSelect.
func PostAction(url string) string   { return "@post('" + url + "')" }
func DeleteAction(url string) string { return "@delete('" + url + "')" }

// FormPostSelect merender <select> yang mem-@post nilainya BESERTA <form>
// pembungkusnya sebagai satu node. Ini membuat gotcha #6 (@post {contentType:'form'}
// butuh <form> terdekat) MUSTAHIL salah: aksi form-valued tak punya jalur lain
// selain lewat helper ini, jadi tak mungkin ada select-post tanpa form-nya. String
// contentType juga dibangun internal → typo ('from') mustahil. Emit identik dengan
// pola lama: <form><select class="select select-sm" name=... data-on:change="@post(...,
// {contentType: 'form'})">opts...</select></form>.
func FormPostSelect(postURL, name string, opts ...g.Node) g.Node {
	return FormPostSelectWith(postURL, name, nil, opts...)
}

// FormPostSelectWith = FormPostSelect + field TERSEMBUNYI ikut terkirim (mis.
// tenant_id saat role per-workspace). hidden: nama→nilai. Tetap satu <form> yang
// dirender helper → gotcha #6 tetap mustahil terjadi.
// OnChangePostQuery memicu @post saat event "change", mengirim NILAI ELEMEN itu
// sendiri (el.value) sebagai query string `?<param>=<value>` — untuk elemen yang
// SUDAH ada di dalam <form> lain (mis. hidden input picker target di form
// aktivitas, BL-164) TANPA membuat <form> baru.
//
// BUKAN {contentType:'form'} (kebalikan niat awal, ditemukan lewat bug laporan
// user: Target berubah, Kontak tak ikut berubah): Datastar men-checkValidity()
// SELURUH <form> terdekat sebelum kirim (bukan cuma elemen ybs) — di form
// aktivitas dengan field required lain (mis. Subjek) yang masih kosong saat
// Target baru dipilih, itu bikin @post GAGAL DIAM-DIAM (`reportValidity()` +
// return, tanpa error terlihat). contentType default 'json' (dipakai di sini)
// TAK memvalidasi apa pun — bodinya (signal JSON) diabaikan backend; nilai
// dikirim lewat query string yang kita rakit sendiri di URL.
func OnChangePostQuery(url, param string) g.Node {
	return data.On("change", "@post('"+url+"?"+param+"='+encodeURIComponent(el.value))")
}

func FormPostSelectWith(postURL, name string, hidden map[string]string, opts ...g.Node) g.Node {
	attrs := []g.Node{
		h.Class("select select-sm"),
		h.Name(name),
		data.On("change", "@post('"+postURL+"', {contentType: 'form'})"),
	}
	nodes := make([]g.Node, 0, len(hidden)+1)
	for k, v := range hidden {
		nodes = append(nodes, h.Input(h.Type("hidden"), h.Name(k), h.Value(v)))
	}
	nodes = append(nodes, h.Select(append(attrs, opts...)...))
	return h.FormEl(nodes...)
}
