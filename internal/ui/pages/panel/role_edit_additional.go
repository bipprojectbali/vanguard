package panel

import (
	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// role_edit_additional.go — checklist "Pengaturan Tambahan" (approve/arr per
// modul + Field Security) dari role_edit.go, dipisah agar keduanya di bawah
// batas global 500 baris/20.000 karakter (bukan hanya matriks utama yang
// tumbuh — checklist ini juga). Satu paket dgn role_edit.go; moduleSignal
// tetap di sana (dipakai roleMatrixRow juga, bukan spesifik "additional").

// additionalSettings merender checklist "Pengaturan Tambahan", dikelompokkan
// dua sub-judul (perbaikan tampilan, 2026-09-17): "Keuangan" (kapabilitas
// approve/arr per modul — dulu kolom "Setujui"/"Lihat ARR" di tabel matriks,
// BL-145 subtask 7) dan "Kontak" (dua checkbox Field Security yang dulu form
// berdiri sendiri, fieldSecurityRoleSection, kini melebur ke SATU form dgn
// matriks). Baris ARR berlabel "Lihat Nilai Kontrak (MRR/ARR/Deal Amount)
// pelanggan" TANPA catatan <p> statis di bawahnya — cakupan lintas modul
// (Subscriptions/Deals/Leads/Accounts/Quotes/Reports) pindah ke tap-info ⓘ
// grup "Keuangan" (keuanganHint), pola sama kontakHint (sudah dipindah
// 2026-09-17), bukan lagi teks selalu-tampil. fsec != nil →
// sisipkan input hidden fsec_present=1: penanda bagi RoleUpdate (roles.go)
// bahwa form INI memang menyertakan bagian Field Security (beda dari submit
// lama/test yang tak menyinggung FLS sama sekali) — tanpanya, view/edit yang
// absen tak bisa dibedakan dari "form tak pernah render bagian ini". Kosong
// sama sekali (nol approve/arr & fsec nil) → g.Text("") (pola ui.When, BUKAN
// nil). mscope (BL-171, MemberScopeRoleView) selalu dioper NON-pointer (beda
// dari fsec) — additionalSettings hanya dipanggil utk peran kustom (role_edit.go),
// dan crm:members SELALU ada di rc.Modules (CRMModules() daftar statis), jadi
// tak ada kondisi "bagian ini absen" spt fsec yang bergantung gate terpisah.
func additionalSettings(rc RoleCard, canEdit bool, fsec *FieldSecurityRoleView, mscope MemberScopeRoleView, actx arrCrossModuleCtx) g.Node {
	var financeRows []g.Node
	for _, m := range rc.Modules {
		suffix := moduleSignal(m.Obj)
		lvlSig := "lvl_" + suffix
		if m.CanApprove {
			apvSig := "apv_" + suffix
			cb := permCheckCell("approve."+m.Obj, true, m.Approve, false)
			if canEdit {
				cb = reactiveCheckbox("approve."+m.Obj, apvSig, "$"+lvlSig+" == 'none'")
			}
			financeRows = append(financeRows, settingRow(cb, "Boleh menyetujui "+m.Label, g.Text("")))
		}
		if m.CanARR {
			arrSig := "arr_" + suffix
			cb := permCheckCell("arr."+m.Obj, true, m.ARR, false)
			if canEdit {
				cb = reactiveCheckbox("arr."+m.Obj, arrSig, actx.disabledExpr)
			}
			financeRows = append(financeRows, settingRow(cb,
				"Lihat Nilai Kontrak (MRR/ARR/Deal Amount) pelanggan", g.Text("")))
		}
	}

	var fsecAttrs []g.Node
	var fsecRows []g.Node
	if fsec != nil {
		var viewCell, editCell g.Node
		if fsec.CanEdit && fsec.Reactive {
			fsecAttrs = append(fsecAttrs, data.Signals(map[string]any{
				"fls_view": fsec.CanViewPhone,
				"fls_edit": fsec.CanEditPhone,
			}))
			viewCell = reactiveFieldSecurityCheck("view", "fls_view",
				"($lvl_contacts==='none'&&$lvl_leads==='none')||$fls_edit")
			editCell = reactiveFieldSecurityCheck("edit", "fls_edit",
				"!($lvl_contacts==='write'||$lvl_leads==='write')",
				"evt.target.checked&&($fls_view=true)")
		} else {
			viewCell = fieldSecurityCheck("view", fsec.CanViewPhone, fsec.CanEdit)
			editCell = fieldSecurityCheck("edit", fsec.CanEditPhone, fsec.CanEdit)
		}
		fsecRows = []g.Node{
			settingRow(viewCell, "Lihat nomor HP/WhatsApp penuh", g.Text("")),
			settingRow(editCell, "Boleh menyunting nomor", g.Text("")),
			h.Input(h.Type("hidden"), h.Name("fsec_present"), h.Value("1")),
		}
	}

	// hasModule guard (bukan unconditional): fixture test lama (role_edit_test.go)
	// merakit RoleCard.Modules manual TANPA baris crm:members — render tanpa
	// syarat akan memunculkan "Pengaturan Tambahan" di skenario yang justru
	// menguji KETIADAANnya. Di produksi rc.Modules SELALU memuat crm:members
	// (authz.CRMModules(), roleModuleRows), jadi guard ini transparan.
	var mscopeRows, mscopeAttrs []g.Node
	if hasModule(rc.Modules, "crm:members") {
		mscopeRows, mscopeAttrs = memberScopeRows(mscope, canEdit)
	}

	if len(financeRows) == 0 && len(fsecRows) == 0 && len(mscopeRows) == 0 {
		return g.Text("")
	}

	inner := []g.Node{h.H2(h.Class("font-semibold text-sm"), g.Text("Pengaturan Tambahan"))}
	if len(financeRows) > 0 {
		inner = append(inner, settingsGroup("Keuangan", keuanganHint, financeRows))
	}
	if len(fsecRows) > 0 {
		inner = append(inner, settingsGroup("Kontak", kontakHint, fsecRows, fsecAttrs...))
	}
	if len(mscopeRows) > 0 {
		inner = append(inner, settingsGroup("Anggota", anggotaHint, mscopeRows, mscopeAttrs...))
	}
	divArgs := append([]g.Node{h.Class("grid gap-3 border-t border-base-300 pt-3 mt-1")}, inner...)
	return h.Div(divArgs...)
}

// kontakHint/keuanganHint = keterangan di balik ikon ⓘ tap-friendly judul
// sub-grup settingsGroup (2026-09-17: dipindah dari <p> statis di bawah judul
// "Kontak" ke tap-i di sebelah judul, pola sama labelWithHint BL-65/BL-129).
// Slice, bukan satu string — tiap butir dirender baris terpisah (hintList).
var kontakHint = []string{
	"Berlaku untuk nomor HP & WhatsApp di Kontak, Prospek (Lead), dan formulir Konversi Lead.",
	"\"Boleh sunting\" otomatis mengikutkan \"lihat nomor penuh\".",
}

var keuanganHint = []string{
	"\"Lihat Nilai Kontrak (MRR/ARR/Deal Amount) pelanggan\" mengatur akses ke nilai kontrak.",
	"Berlaku lintas modul: Subscriptions, Deals, Leads, Accounts, Quotes, Renewals, " +
		"Churn/Cancellations, Reports.",
	"\"Boleh menyetujui\" mengatur wewenang approve Deals/Quotes/Renewal Management, " +
		"(tak perlu pilih \"Kelola\" untuk bisa menyetujui).",
}

// anggotaHint = keterangan tap-info grup "Anggota" (BL-171, cakupan halaman
// Anggota via module "User Management"/crm:members) — jelaskan beda dgn
// dropdown "Jenis Anggota" (business_roles.kind, di atas form) agar admin tak
// rancu: yang satu soal peran ini UNTUK siapa, yang ini soal peran ini boleh
// MELIHAT data anggota jenis apa.
var anggotaHint = []string{
	"Menentukan anggota berjenis apa yang boleh dilihat/dikelola peran ini di " +
		"halaman Anggota, lewat akses \"User Management\".",
	"Beda dari \"Jenis Anggota\" di atas (peran ini UNTUK anggota internal/eksternal) " +
		"— ini soal DATA anggota siapa yang terlihat.",
	"Aktif otomatis (kedua jenis tercentang) saat \"User Management\" diubah ke " +
		"\"Lihat\"/\"Kelola\"; minimal satu jenis harus tetap tercentang.",
}

// settingsGroup = sub-judul di dalam "Pengaturan Tambahan" yang mengelompokkan
// baris terkait tema yang sama (mis. "Keuangan" utk approve/ARR, "Kontak" utk
// Field Security) — perbaikan tampilan agar admin langsung tahu KONTEKS tiap
// checklist tanpa menerka dari label baris saja. hint kosong → judul polos.
// attrs (BL-171 fix) diletakkan pada DIV GRUP INI, bukan dititipkan ke div
// pembungkus "Pengaturan Tambahan" bersama — sebelumnya fsecAttrs & mscopeAttrs
// sama-sama menumpuk di satu div luar, menghasilkan DUA atribut data-signals
// pada elemen yang sama; parser HTML membuang duplikat kedua (msp_* Field
// Officer selalu ke-drop), jadi checkbox "Lihat anggota internal/eksternal"
// selalu unchecked walau data DB benar. Tiap grup kini bawa data-signals
// sendiri, tak ada lagi tabrakan.
func settingsGroup(title string, hint []string, rows []g.Node, attrs ...g.Node) g.Node {
	inner := append([]g.Node{sectionHeading(title, hint)}, rows...)
	divArgs := append([]g.Node{h.Class("grid gap-2")}, attrs...)
	divArgs = append(divArgs, inner...)
	return h.Div(divArgs...)
}

// sectionHeading = judul settingsGroup + ikon ⓘ tap-friendly opsional, pola
// sama labelWithHint (accounts_form_fields.go, BL-65/BL-129) tapi membungkus
// h3 judul sub-grup, bukan label field. hint kosong → h3 biasa (tanpa
// <details>), agar sub-grup tanpa keterangan tak ikut jadi tap target kosong.
func sectionHeading(title string, hint []string) g.Node {
	titleNode := h.H3(h.Class("text-xs font-semibold text-base-content/60 uppercase tracking-wide"),
		g.Text(title))
	if len(hint) == 0 {
		return titleNode
	}
	return h.Details(
		h.Class("hint-reveal min-w-0"),
		h.Summary(
			h.Class("hint-summary flex items-center gap-1 min-w-0 cursor-pointer"),
			titleNode,
			h.Span(
				h.Class("inline-flex items-center justify-center min-h-11 min-w-11 -my-2 shrink-0 text-base-content/50"),
				g.Attr("aria-hidden", "true"),
				lucide.Info(h.Class("size-4")),
			),
		),
		hintPop(hintList(hint)),
	)
}

// hintList merender keterangan tap-info ⓘ sebagai baris terpisah — satu <p>
// per butir (bukan satu paragraf mengalir) agar tiap poin gampang dipindai.
func hintList(items []string) g.Node {
	rows := make([]g.Node, 0, len(items))
	for _, it := range items {
		rows = append(rows, h.P(h.Class("text-xs font-normal text-base-content/70 break-words"),
			g.Text(it)))
	}
	return h.Div(h.Class("grid gap-1"), g.Group(rows))
}

// settingRow = satu baris checklist "Pengaturan Tambahan": checkbox + label,
// dgn catatan opsional (note) di bawahnya. note kosong → pakai g.Text("")
// (pola ui.When), bukan nil.
func settingRow(cb g.Node, label string, note g.Node) g.Node {
	return h.Div(h.Class("grid gap-0.5"),
		h.Label(h.Class("flex items-center gap-2 text-sm"), cb, g.Text(label)),
		note,
	)
}

// permCheckCell = satu sel kotak-centang matriks (approve/arr) versi STATIS
// (tanpa reaktivitas — dipakai saat !canEdit, atau saat modul tak punya
// approve/arr sama sekali). show=false → "—" (modul tak mendukung kapabilitas
// ini); show=true → checkbox name=name, value=1, tercentang bila checked,
// terkunci bila !canEdit. Menyatukan pola approve & arr (BL-58) agar tak ada
// dua salinan builder yang bisa menyimpang.
func permCheckCell(name string, show, checked, canEdit bool) g.Node {
	if !show {
		return h.Span(h.Class("text-base-content/40"), g.Text("—"))
	}
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

// reactiveCheckbox = checkbox approve/arr versi REAKTIF (BL-145 subtask 4/5):
// checked-nya di-bind dua-arah ke signal sig (bukan h.Checked() statis) —
// dinonaktifkan reaktif saat disabledExpr benar, dan DIPAKSA false oleh
// handler on:change level select (roleMatrixRow) saat itu terjadi. disabledExpr
// dibangun pemanggil: approve = level baris sendiri ("$lvl_x == 'none'"), arr =
// OR lintas modul (arrCrossModuleCtx.disabledExpr, sejak gate ARR melebar ke
// modul lain). name/value TETAP native agar form ter-submit apa adanya
// (checkbox nonaktif tak ikut terkirim, sama seperti checkbox biasa).
func reactiveCheckbox(name, sig, disabledExpr string) g.Node {
	return h.Input(
		h.Type("checkbox"), h.Class("checkbox checkbox-sm"),
		h.Name(name), h.Value("1"),
		data.Bind(sig),
		data.Attr("disabled", disabledExpr),
	)
}
