package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// form_numeric_fields.go — field input NUMERIK khusus (uang & nomor telepon),
// pelengkap field() generik di accounts_form_fields.go (satu paket panel). Dipisah
// karena keduanya butuh atribut yang field() generik tak punya: inputmode +
// pattern + (utk uang) penanda data-numgroup yang dibaca static/numgroup.js.
//
// BL-2: HP/WhatsApp/Nilai Estimasi dibuat "berasa angka" di mobile —
//   - inputmode="numeric" memunculkan keypad angka iOS/Android (bukan keyboard
//     penuh) tanpa memakai type="number" (yang MENOLAK leading zero "0812…" &
//     prefix "+62", serta menaruh spinner tak relevan utk uang/telepon).
//   - pattern = jaring klien; backend (parseLeadForm → optPhone/optNumeric) tetap
//     penjaga sesungguhnya. text-base (≥16px) agar iOS tak auto-zoom saat fokus.
//
// Keduanya setengah-lebar (grid 2-kolom sm:) seperti field() tanpa hint, agar tata
// letak formCard tak berubah.

// moneyField = input rupiah BULAT (tanpa sen). type="text" + inputmode="numeric"
// + pattern yang menoleransi titik agar nilai TERFORMAT ("5.000.000") lolos
// constraint-validation saat submit (numgroup.js memformat tampilan lalu
// menormalkannya jadi digit polos sebelum kirim; tanpa JS pun aman — backend
// membuang pemisah ribuan). data-numgroup = kait yang dibaca numgroup.js.
func moneyField(label, name, val string) g.Node {
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelFor(label, "f-"+name, false),
		ui.Input(
			h.ID("f-"+name), h.Name(name), h.Type("text"),
			g.Attr("inputmode", "numeric"), g.Attr("pattern", "[0-9.]*"),
			g.Attr("data-numgroup", ""),
			h.Value(val), h.Placeholder("mis. 5.000.000"),
			h.Class("input text-base w-full"),
		),
	)
}

// moneyFieldRp = moneyField dengan afiks "Rp" di kiri (BL-60, APBDes). Pola
// daisyUI 5: kelas .input dipasang pada <label> pembungkus (jadi flex-row), teks
// "Rp" + <input> polos di dalamnya — input dalam TAK memakai kelas .input lagi
// agar tak berbingkai ganda. Atribut numerik & data-numgroup identik moneyField
// (keypad angka + pengelompokan ribuan + normalisasi digit saat submit); text-base
// (≥16px) di input dalam agar iOS tak auto-zoom. Backend cleanThousands tetap
// penjaga bila JS mati.
func moneyFieldRp(label, name, val string) g.Node {
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelFor(label, "f-"+name, false),
		h.Label(
			h.Class("input text-base w-full flex items-center gap-2"),
			h.Span(h.Class("text-base-content/60"), g.Text("Rp")),
			h.Input(
				h.ID("f-"+name), h.Name(name), h.Type("text"),
				g.Attr("inputmode", "numeric"), g.Attr("pattern", "[0-9.]*"),
				g.Attr("data-numgroup", ""),
				h.Value(val), h.Placeholder("mis. 5.000.000"),
				h.Class("grow text-base"),
			),
		),
	)
}

// phoneNumField = nomor telepon (HP/WhatsApp). type="tel" memberi semantik +
// keypad telepon; inputmode="numeric" menjaga keypad angka konsisten. pattern
// mengizinkan digit, '+' (prefix negara), spasi & tanda hubung sebagai pemisah.
// TIDAK memakai type="number": itu akan membuang leading zero & '+'. Nilai asli
// pengguna (termasuk "0812…"/"+62…") dipertahankan apa adanya oleh backend.
//
// BL-81: data-phonenum = kait yang dibaca static/phonenum.js — menyaring
// karakter tak-diizinkan SAAT DIKETIK (bukan hanya saat submit), sehingga field
// tak pernah berisi huruf/simbol mustahil. pattern hanya jaring pasif; backend
// (optPhone) tetap penolak sesungguhnya saat submit. Tanpa JS pun aman.
//
// BL-84: editable=false → field DIKUNCI (disabled, TANPA name) menampilkan nilai
// tersamar ("•••") + keterangan, persis phoneField (accounts_form_fields.go).
// Bug yang diperbaiki: dulu field bermask "•••" tetap punya name & aktif, jadi
// role tak-berhak-lihat (mis. Manager) mengirim "•••" saat submit → optPhone
// menolaknya (bukan [0-9+ -]) → seluruh sunting GAGAL validasi, role itu mustahil
// menyimpan perubahan apa pun. Dengan disabled+tanpa-name, mask tak ikut ter-submit;
// guard di LeadUpdate mempertahankan nomor asli.
func phoneNumField(label, name, val string, editable bool) g.Node {
	if !editable {
		return h.Div(
			h.Class("grid gap-1 min-w-0"),
			labelFor(label, "f-"+name+"_ro", false),
			ui.Input(
				h.ID("f-"+name+"_ro"), h.Type("tel"), h.Value(val),
				h.Disabled(), h.Class("input text-base w-full"),
			),
			h.P(h.Class("text-xs text-base-content/60"),
				g.Text("Nomor disamarkan & hanya bisa disunting oleh Sales.")),
		)
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelFor(label, "f-"+name, false),
		ui.Input(
			h.ID("f-"+name), h.Name(name), h.Type("tel"),
			g.Attr("inputmode", "numeric"), g.Attr("pattern", "[0-9+ -]*"),
			g.Attr("data-phonenum", ""),
			h.Value(val), h.Placeholder("mis. 0812… / +62812…"),
			h.Class("input text-base w-full"),
		),
	)
}
