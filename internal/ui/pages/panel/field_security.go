package panel

import (
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// field_security.go — section Field Security (F4, HP/WhatsApp) DAN indikator
// Nilai Kontrak/MRR, keduanya kini ditanam di halaman DETAIL satu peran
// (/roles/{name}, BL-145 subtask 3) — bukan lagi section/kartu tenant-wide di
// /roles. Dipindah agar signal Datastar-nya bisa bereaksi thd level
// Contacts/Leads (F2) peran yang SAMA (ADR 0012: sumbu F2/F4 tetap tak
// dicampur satu TRANSAKSI, tapi kini satu PAGE LOAD demi reaktivitas klien).
// Dipanggil dari role_edit.go, bukan dirender sebagai halaman sendiri.

// FieldSecurityRoleView = data siap-render form Field Security SATU peran.
// Name = identitas mesin peran (dipakai merakit action form). Reactive=true
// untuk peran kustom (ada matriks Contacts/Leads utk direaksikan, signal
// lvl_contacts/lvl_leads sudah dideklarasikan roleMatrixRow di halaman yang
// sama); false untuk peran sistem (admin diwakili glob crm:*, tak ada matriks
// — checkbox statis biasa, tanpa guard).
type FieldSecurityRoleView struct {
	Base         string
	Name         string
	CanEdit      bool
	CanViewPhone bool
	CanEditPhone bool
	Reactive     bool
}

// fieldSecurityRoleSection merender kartu kecil Field Security SATU peran:
// form TERPISAH dari form matriks Casbin (ADR 0012 — F2/F4 tak dicampur satu
// transaksi walau kini satu halaman), dua checkbox (name="view"/"edit", TANPA
// suffix ".role" — beda dari matriks lama tenant-wide karena kini satu peran
// per form). v.Reactive && v.CanEdit → checkbox dibind ke signal Datastar
// $fls_view/$fls_edit, dinonaktifkan+dipaksa-mati reaktif mengikuti level
// gabungan Contacts/Leads (signal lvl_contacts/lvl_leads, dideklarasikan
// roleMatrixRow — role_edit.go). Selain itu → checkbox statis (v.CanEdit saja
// menentukan disabled non-reaktif, pola fieldSecurityCheck lama).
func fieldSecurityRoleSection(v FieldSecurityRoleView) g.Node {
	var viewCell, editCell g.Node
	var formAttrs []g.Node
	if v.CanEdit && v.Reactive {
		formAttrs = append(formAttrs, data.Signals(map[string]any{
			"fls_view": v.CanViewPhone,
			"fls_edit": v.CanEditPhone,
		}))
		viewCell = reactiveFieldSecurityCheck("view", "fls_view",
			"($lvl_contacts==='none'&&$lvl_leads==='none')||$fls_edit")
		editCell = reactiveFieldSecurityCheck("edit", "fls_edit",
			"!($lvl_contacts==='write'||$lvl_leads==='write')",
			"evt.target.checked&&($fls_view=true)")
	} else {
		viewCell = fieldSecurityCheck("view", v.CanViewPhone, v.CanEdit)
		editCell = fieldSecurityCheck("edit", v.CanEditPhone, v.CanEdit)
	}

	rows := h.Div(
		h.Class("grid gap-2"),
		h.Label(h.Class("flex items-center gap-2 text-sm"),
			viewCell, g.Text("Lihat nomor HP/WhatsApp penuh")),
		h.Label(h.Class("flex items-center gap-2 text-sm"),
			editCell, g.Text("Boleh menyunting nomor")),
	)
	inner := []g.Node{
		h.H2(h.Class("font-semibold"), g.Text("Field Security")),
		h.P(h.Class("text-xs text-base-content/60 mb-1"),
			g.Text("Berlaku untuk nomor HP & WhatsApp di Kontak, Prospek (Lead), "+
				"dan formulir Konversi Lead. \"Boleh sunting\" otomatis mengikutkan "+
				"\"lihat nomor penuh\".")),
		rows,
	}
	if v.CanEdit {
		inner = append(inner, h.Div(h.Class("flex flex-wrap items-center gap-2 mt-1"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary btn-sm min-h-11"),
				g.Text("Simpan"))))
		return h.FormEl(append(
			append([]g.Node{
				h.Method("post"), h.Action(v.Base + "/roles/" + v.Name + "/field-security"),
				h.Class("card bg-base-100 border border-base-300 min-w-0 mt-2"),
			}, formAttrs...),
			h.Div(append([]g.Node{h.Class("card-body min-w-0 gap-2")}, inner...)...),
		)...)
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0 mt-2"),
		h.Div(append([]g.Node{h.Class("card-body min-w-0 gap-2")}, inner...)...),
	)
}

// fieldSecurityCheck = kotak-centang FLS versi STATIS (tanpa Datastar) — peran
// sistem (bebas centang, tanpa guard) atau workspace read-only (dikunci).
func fieldSecurityCheck(name string, checked, canEdit bool) g.Node {
	attrs := []g.Node{
		h.Type("checkbox"), h.Class("checkbox checkbox-sm"),
		h.Name(name), h.Value("1"),
	}
	if checked {
		attrs = append(attrs, h.Checked())
	}
	if !canEdit {
		attrs = append(attrs, h.Disabled())
	}
	return h.Input(attrs...)
}

// reactiveFieldSecurityCheck = kotak-centang FLS versi REAKTIF (BL-145
// subtask 3, pola reactiveCheckbox di role_edit.go tapi disabledExpr bebas —
// FLS bereaksi thd KOMBINASI dua signal lvl_contacts & lvl_leads, bukan satu
// signal baris seperti approve/arr). Signal sig DIPAKSA false oleh handler
// on:change kedua level select (roleMatrixRow) saat kondisi disabledExpr
// terpenuhi — disabled attribute di sini murni presentasi (Datastar re-eval
// otomatis tiap signal rujukan berubah), penegak sesungguhnya tetap backend
// (coercion server-side, role_field_security.go). onChange (opsional,
// variadic) = ekspresi tambahan dieksekusi saat checkbox berubah — dipakai
// checkbox "edit" memaksa $fls_view=true saat dicentang (2026-09-17: UI
// sebelumnya cuma menjanjikan via teks tap-info "Boleh sunting otomatis
// mengikutkan lihat nomor penuh" tanpa benar-benar reaktif; evt.target.checked
// dipakai, bukan $fls_edit, pola sama roleMatrixRow — hindari race dgn
// listener data.Bind pada event 'change' yang sama).
func reactiveFieldSecurityCheck(name, sig, disabledExpr string, onChange ...string) g.Node {
	attrs := []g.Node{
		h.Type("checkbox"), h.Class("checkbox checkbox-sm"),
		h.Name(name), h.Value("1"),
		data.Bind(sig),
		data.Attr("disabled", disabledExpr),
	}
	if len(onChange) > 0 && onChange[0] != "" {
		attrs = append(attrs, data.On("change", onChange[0]))
	}
	return h.Input(attrs...)
}

// contractValueIndicator merender indikator baca-saja "Nilai Kontrak/MRR"
// untuk satu peran: apakah peran ini melihat nilai kontrak/ARR pelanggan.
// TANPA form/POST (presentasional murni, pola kartu lama) — sejak BL-145
// subtask 3 axis ini SEPENUHNYA mengikuti F2 (checkbox "Lihat ARR" baris
// Subscriptions, signal arr_subscriptions yang sudah reaktif sejak subtask
// 4/5), bukan lagi F4 tersendiri. reactive=true (peran kustom, canEdit) →
// dua <span> toggle live via data.Show mengikuti signal yang SAMA, tanpa
// submit; else → statis dari nilai visible yang dihitung handler.
func contractValueIndicator(visible, reactive bool) g.Node {
	label := h.Span(h.Class("text-base-content/70"), g.Text("Nilai Kontrak/MRR: "))
	if reactive {
		return h.Div(h.Class("flex items-center gap-2 text-sm mt-2"),
			label,
			h.Span(h.Class("text-success font-semibold"), data.Show("$arr_subscriptions"),
				g.Text("Terlihat")),
			h.Span(h.Class("text-base-content/40"), data.Show("!$arr_subscriptions"),
				g.Text("Tersembunyi")),
		)
	}
	mark := h.Span(h.Class("text-base-content/40"), g.Text("Tersembunyi"))
	if visible {
		mark = h.Span(h.Class("text-success font-semibold"), g.Text("Terlihat"))
	}
	return h.Div(h.Class("flex items-center gap-2 text-sm mt-2"), label, mark)
}
