package panel

import (
	"strings"

	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// role_edit.go — view HALAMAN detail/edit SATU peran CRM
// (/w/{slug}/roles/{name}). Dipisah dari daftar (roles.go): daftar soal "peran
// apa saja ada", halaman ini soal "izin & cakupan satu peran". Murni-data:
// handler sudah menghitung level tiap sel & menandai peran sistem.

// RoleModulePerm = satu baris matriks (satu modul CRM) dalam sebuah peran.
// Level ∈ {none, read, write} — read & write dilipat jadi satu tingkat karena
// write MENCAKUP read (business.conf). Approve berdiri sendiri dan hanya
// bermakna bila CanApprove (Deals/Quotes/Renewal Management).
type RoleModulePerm struct {
	Obj        string
	Label      string
	CanApprove bool
	CanARR     bool // true → sel "Lihat ARR" dirender (hanya Subscriptions, BL-58)
	Level      string
	Approve    bool
	ARR        bool // tersetel → kotak "Lihat ARR" tercentang
}

// RoleCard = satu peran CRM workspace beserta matriksnya. Name = identitas mesin
// (subject Casbin, tak berubah), DisplayName = label layar. IsSystem (admin) →
// Modules nil dan dirender TERKUNCI: admin diwakili glob crm:* yang tak
// terpetakan ke kolom per-modul, jadi menampilkannya sebagai matriks akan keliru
// "semua None".
type RoleCard struct {
	Name        string
	DisplayName string
	Description string
	DataScope   string
	IsSystem    bool
	Modules     []RoleModulePerm
}

// levelOptions = pilihan tingkat izin per modul. Value = string mesin yang
// dibaca readRoleMatrix di handler ("read"/"write"; selain itu → dilewati = none).
var levelOptions = []ScopeOption{
	{Value: "none", Label: "Tak ada"},
	{Value: "read", Label: "Lihat"},
	{Value: "write", Label: "Kelola"},
}

// RoleEdit merender halaman satu peran: judul + link kembali + alert, lalu
// (peran sistem) keterangan terkunci ATAU (peran kustom) SATU form sunting
// label/cakupan/matriks/Pengaturan Tambahan (+ Field Security bila fsec != nil)
// dalam SATU submit (BL-145 subtask 7), plus kartu "Zona Berbahaya" (Hapus
// Peran) TERPISAH — form hapus sengaja tak pernah ikut form sunting supaya
// submit yang tak disengaja tak bisa memicu hapus. base = prefix URL workspace
// (dioper handler). canEdit=false → form terkunci & tombol simpan/hapus
// disembunyikan. fsec nil → peninjau tak berwenang crm:field_security, section
// Field Security tak dirender sama sekali.
func RoleEdit(base string, rc RoleCard, scopes []ScopeOption, canEdit bool, errMsg, okMsg string, fsec *FieldSecurityRoleView) g.Node {
	header := []g.Node{
		h.A(h.Href(base+"/roles"), h.Class("link text-sm text-base-content/70"),
			g.Text("← Kembali ke daftar peran")),
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.H1(h.Class("text-xl font-semibold"), g.Text(rc.DisplayName)),
			h.Span(h.Class("badge badge-neutral font-mono text-xs"), g.Text(rc.Name)),
			ui.When(rc.IsSystem, h.Span(h.Class("badge badge-warning"), g.Text("Sistem"))),
		),
	}
	if errMsg != "" {
		header = append(header, ui.Toast(ui.VariantDestructive, "role-err", g.Text(errMsg)))
	}
	if okMsg != "" {
		header = append(header, ui.Toast(ui.VariantSuccess, "role-ok", g.Text(okMsg)))
	}

	if rc.IsSystem {
		body := append(header,
			ui.When(rc.Description != "", h.P(h.Class("text-sm text-base-content/70"),
				g.Text(rc.Description))),
			h.P(h.Class("text-sm text-base-content/70"),
				g.Text("Peran bawaan dengan akses penuh ke semua modul CRM. "+
					"Tak bisa disunting atau dihapus.")))
		if fsec != nil {
			// Admin diwakili glob crm:* — akses penuh, statis, tanpa guard reaktif
			// (tak ada matriks Contacts/Leads untuk direaksikan). Peran sistem
			// tak punya matriks utk dilebur dgn Field Security, jadi bentuknya
			// TETAP form berdiri sendiri (satu-satunya form di halaman ini).
			body = append(body, fieldSecurityRoleSection(*fsec))
		}
		// Indikator Nilai Kontrak/MRR TAK bergantung gate crm:field_security
		// (§3b) — admin selalu "Terlihat" (akses penuh crm:*), statis.
		body = append(body, contractValueIndicator(true, false))
		return roleEditShell(body)
	}

	settingsForm := h.FormEl(
		h.Method("post"), h.Action(base+"/roles/"+rc.Name),
		h.Class("grid gap-3 min-w-0"),
		h.Div(
			h.Class("grid gap-3 sm:grid-cols-2"),
			h.Div(
				h.Class("grid gap-2"),
				ui.Label("Nama tampilan"),
				ui.Input(h.Name("display_name"), h.Type("text"),
					h.Value(rc.DisplayName), h.Required(), disabledIf(!canEdit)),
			),
			h.Div(
				h.Class("grid gap-2"),
				ui.Label("Cakupan data"),
				scopeSelect("", "data_scope", rc.DataScope, scopes, !canEdit),
			),
		),
		h.Div(
			h.Class("grid gap-2"),
			ui.Label("Deskripsi (opsional)"),
			ui.Input(h.Name("description"), h.Type("text"),
				h.Value(rc.Description), h.MaxLength("200"),
				h.Placeholder("Ringkas peran ini dalam satu kalimat"),
				disabledIf(!canEdit)),
		),
		roleMatrix(rc, canEdit, flsReactive(fsec)),
		additionalSettings(rc, canEdit, fsec),
		ui.When(canEdit, h.Button(h.Type("submit"),
			h.Class("btn btn-primary min-h-11 justify-self-start"),
			g.Text("Simpan Perubahan"))),
	)

	cards := [][]g.Node{{settingsForm}}
	if canEdit {
		cards = append(cards, []g.Node{
			h.H2(h.Class("font-semibold text-error"), g.Text("Zona Berbahaya")),
			h.P(h.Class("text-sm text-base-content/70"),
				g.Text("Menghapus peran akan mencabutnya dari semua anggota yang "+
					"memegangnya (mereka tak terhapus, hanya kehilangan peran ini).")),
			roleDeleteForm(base, rc.Name, "Hapus Peran"),
		})
	}
	return roleEditShellCards(header, cards...)
}

// roleEditShell = kartu tunggal berlebar terbatas (max-w-3xl) agar form editor
// tak melebar penuh di layar besar. min-w-0 agar tabel matriks di dalam bisa
// menyusut (konvensi mobile-first).
func roleEditShell(inner []g.Node) g.Node {
	return h.Div(
		h.Class("grid gap-4 min-w-0 max-w-3xl"),
		h.Div(
			h.Class("card bg-base-100 border border-base-300 min-w-0"),
			h.Div(h.Class("card-body min-w-0 gap-3"), g.Group(inner)),
		),
	)
}

// roleEditShellCards = varian roleEditShell utk peran KUSTOM (BL-145 subtask
// 7): header (link kembali/judul/badge/toast) dirender DI LUAR kartu apa pun,
// lalu tiap slice di cards dibungkus KARTUNYA SENDIRI. Dipakai memisahkan
// SECARA VISUAL form Pengaturan (identitas+matriks+Pengaturan Tambahan+Field
// Security, SATU submit) dari Zona Berbahaya (Hapus Peran, form POST
// TERPISAH) — dua form tak pernah berbagi satu card, walau satu halaman.
func roleEditShellCards(header []g.Node, cards ...[]g.Node) g.Node {
	all := make([]g.Node, 0, len(header)+len(cards))
	all = append(all, header...)
	for _, inner := range cards {
		all = append(all, h.Div(
			h.Class("card bg-base-100 border border-base-300 min-w-0"),
			h.Div(h.Class("card-body min-w-0 gap-3"), g.Group(inner)),
		))
	}
	return h.Div(h.Class("grid gap-4 min-w-0 max-w-3xl"), g.Group(all))
}

// flsReactive menentukan apakah baris crm:contacts/crm:leads perlu klausa
// reset silang ke $fls_view/$fls_edit (BL-145 subtask 3) — hanya bila section
// Field Security DIRENDER reaktif di halaman yang sama (fsec != nil, peran
// kustom, boleh disunting); fsec nil (tak berwenang crm:field_security) atau
// checkbox-nya statis → tak ada signal fls_view/fls_edit untuk dirujuk.
func flsReactive(fsec *FieldSecurityRoleView) bool {
	return fsec != nil && fsec.Reactive && fsec.CanEdit
}

// roleMatrix = tabel izin per modul CRM. Dibungkus ui.TableScroll agar scroll
// terkurung, tak mendorong lebar halaman di mobile (konvensi mobile-first).
// fls → diteruskan ke roleMatrixRow (lihat flsReactive di atas). BL-169: 4
// modul Reports (Sales/CS/Support/Subscription) dirender FLAT sebagai baris
// biasa, sama seperti modul lain — tanpa header grup.
func roleMatrix(rc RoleCard, canEdit bool, fls bool) g.Node {
	rows := make([]g.Node, 0, len(rc.Modules))
	for _, m := range rc.Modules {
		rows = append(rows, roleMatrixRow(m, canEdit, fls))
	}
	return ui.TableScroll(h.Table(
		h.Class("w-full text-sm"),
		h.THead(h.Tr(
			h.Class("border-b border-base-300 text-left text-base-content/70"),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Modul")),
			h.Th(h.Class("py-2 font-medium"), g.Text("Akses")),
		)),
		h.TBody(g.Group(rows)),
	))
}

// roleMatrixRow merender satu baris matriks modul — HANYA Modul + Akses
// (BL-145 subtask 7: kolom "Setujui"/"Lihat ARR" pindah keluar tabel jadi
// checklist "Pengaturan Tambahan", additionalSettings di bawah, agar matriks
// tetap 2 kolom sempit di mobile). Checkbox approve/arr itu sendiri TETAP
// dirender additionalSettings, tapi signal Datastar-nya (lvl_/apv_/arr_ +
// nama objek tanpa prefix "crm:") tetap DIDEKLARASIKAN DI SINI (data.Signals
// pada <tr>) — Datastar signal adalah store GLOBAL, bukan terikat posisi DOM,
// jadi checklist yang dirender belakangan tetap bisa merujuknya asal
// dideklarasikan lebih dulu di dokumen (pola sama contractValueIndicator &
// fieldSecurityRoleSection, sudah berjalan sebelum subtask ini). Saat canEdit,
// level select SELALU dapat signal (lvl_<suffix>) — termasuk crm:contacts/
// crm:leads yang tak punya approve/arr, sebab section Field Security di
// halaman yang sama butuh bereaksi thd levelnya. Level "Tak ada" otomatis
// memaksa signal apv_/arr_ baris itu balik ke false (checkbox terkait di
// additionalSettings ikut nonaktif+tak tercentang, BL-145 subtask 4/5); level
// "Lihat" ATAU "Kelola" membebaskannya lagi (approve SENGAJA tak butuh
// "Kelola" — pola maker-checker, lihat memory bl-145-roles-redesign.md).
// fls=true & m.Obj ∈ {crm:contacts, crm:leads} → dua klausa TAMBAHAN memaksa
// $fls_view/$fls_edit false bila kombinasi lvl_contacts+lvl_leads tak lagi
// memenuhi syarat (signal itu dideklarasikan additionalSettings, bukan di
// sini). Backend (readRoleMatrix, guard hasLevel BL-145 subtask 0;
// writeFieldSecurity utk FLS) TETAP penjaga sesungguhnya — reaktivitas ini
// murni UX, bukan pengganti validasi server.
func roleMatrixRow(m RoleModulePerm, canEdit bool, fls bool) g.Node {
	levelCell := levelSelect(m.Obj, m.Level, !canEdit)
	var rowAttrs []g.Node

	if canEdit {
		suffix := moduleSignal(m.Obj)
		lvlSig := "lvl_" + suffix
		hasLevel := m.Level != "none"
		signals := map[string]any{lvlSig: m.Level}
		var resets []string
		if m.CanApprove {
			apvSig := "apv_" + suffix
			signals[apvSig] = m.Approve && hasLevel
			resets = append(resets, "$"+apvSig+"=false")
		}
		if m.CanARR {
			arrSig := "arr_" + suffix
			signals[arrSig] = m.ARR && hasLevel
			resets = append(resets, "$"+arrSig+"=false")
		}
		var stmts []string
		if len(resets) > 0 {
			stmts = append(stmts, "evt.target.value==='none'&&("+strings.Join(resets, ",")+")")
		}
		if fls && (m.Obj == "crm:contacts" || m.Obj == "crm:leads") {
			stmts = append(stmts,
				"($lvl_contacts==='none'&&$lvl_leads==='none')&&($fls_view=false)",
				"!($lvl_contacts==='write'||$lvl_leads==='write')&&($fls_edit=false)")
		}
		selectAttrs := []g.Node{
			h.Class("select select-sm"), h.Name("level." + m.Obj),
			data.Bind(lvlSig),
		}
		if len(stmts) > 0 {
			selectAttrs = append(selectAttrs, data.On("change", strings.Join(stmts, ";")))
		}
		levelCell = h.Select(append(selectAttrs, g.Group(levelOpts(m.Level)))...)
		rowAttrs = append(rowAttrs, data.Signals(signals))
	}

	return h.Tr(append(rowAttrs,
		h.Class("border-b border-base-300/50"),
		h.Td(h.Class("py-2 pr-4"), g.Text(m.Label)),
		h.Td(h.Class("py-2"), levelCell),
	)...)
}

// additionalSettings merender checklist "Pengaturan Tambahan", dikelompokkan
// dua sub-judul (perbaikan tampilan, 2026-09-17): "Keuangan" (kapabilitas
// approve/arr per modul — dulu kolom "Setujui"/"Lihat ARR" di tabel matriks,
// BL-145 subtask 7) dan "Kontak" (dua checkbox Field Security yang dulu form
// berdiri sendiri, fieldSecurityRoleSection, kini melebur ke SATU form dgn
// matriks). Baris ARR berlabel tetap "Lihat MRR dan ARR pelanggan" (dulu
// disisipi nama modul + catatan kecil Nilai Kontrak/MRR terpisah — dibuang,
// satu-satunya modul ber-CanARR hari ini adalah Subscriptions jadi label
// generik sudah cukup jelas tanpa perlu template per-modul). fsec != nil →
// sisipkan input hidden fsec_present=1: penanda bagi RoleUpdate (roles.go)
// bahwa form INI memang menyertakan bagian Field Security (beda dari submit
// lama/test yang tak menyinggung FLS sama sekali) — tanpanya, view/edit yang
// absen tak bisa dibedakan dari "form tak pernah render bagian ini". Kosong
// sama sekali (nol approve/arr & fsec nil) → g.Text("") (pola ui.When, BUKAN
// nil).
func additionalSettings(rc RoleCard, canEdit bool, fsec *FieldSecurityRoleView) g.Node {
	var financeRows []g.Node
	for _, m := range rc.Modules {
		suffix := moduleSignal(m.Obj)
		lvlSig := "lvl_" + suffix
		if m.CanApprove {
			apvSig := "apv_" + suffix
			cb := permCheckCell("approve."+m.Obj, true, m.Approve, false)
			if canEdit {
				cb = reactiveCheckbox("approve."+m.Obj, apvSig, lvlSig)
			}
			financeRows = append(financeRows, settingRow(cb, "Boleh menyetujui "+m.Label, g.Text("")))
		}
		if m.CanARR {
			arrSig := "arr_" + suffix
			cb := permCheckCell("arr."+m.Obj, true, m.ARR, false)
			if canEdit {
				cb = reactiveCheckbox("arr."+m.Obj, arrSig, lvlSig)
			}
			financeRows = append(financeRows, settingRow(cb, "Lihat MRR dan ARR pelanggan", g.Text("")))
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

	if len(financeRows) == 0 && len(fsecRows) == 0 {
		return g.Text("")
	}

	inner := []g.Node{h.H2(h.Class("font-semibold text-sm"), g.Text("Pengaturan Tambahan"))}
	if len(financeRows) > 0 {
		inner = append(inner, settingsGroup("Keuangan", keuanganHint, financeRows))
	}
	if len(fsecRows) > 0 {
		inner = append(inner, settingsGroup("Kontak", kontakHint, fsecRows))
	}
	divArgs := append([]g.Node{h.Class("grid gap-3 border-t border-base-300 pt-3 mt-1")}, fsecAttrs...)
	divArgs = append(divArgs, inner...)
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
	"\"Lihat MRR dan ARR pelanggan\" untuk mengatur akses ke nilai ARR/MRR.",
	"\"Boleh menyetujui\" mengatur wewenang approve Deals/Quotes/Renewal Management, " +
		"(tak perlu pilih \"Kelola\" untuk bisa menyetujui).",
}

// settingsGroup = sub-judul di dalam "Pengaturan Tambahan" yang mengelompokkan
// baris terkait tema yang sama (mis. "Keuangan" utk approve/ARR, "Kontak" utk
// Field Security) — perbaikan tampilan agar admin langsung tahu KONTEKS tiap
// checklist tanpa menerka dari label baris saja. hint kosong → judul polos.
func settingsGroup(title string, hint []string, rows []g.Node) g.Node {
	inner := append([]g.Node{sectionHeading(title, hint)}, rows...)
	return h.Div(append([]g.Node{h.Class("grid gap-2")}, inner...)...)
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

// moduleSignal mengubah objek Casbin ("crm:renewals") jadi sufiks aman-sinyal
// ("renewals") — objek modul CRM SELALU berawalan "crm:" (crmModules,
// business_defaults.go), sisanya huruf kecil+underscore saja, jadi aman jadi
// identifier Datastar tanpa escaping tambahan.
func moduleSignal(obj string) string {
	return strings.TrimPrefix(obj, "crm:")
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
// dinonaktifkan reaktif saat level baris (lvlSig) = "none", dan DIPAKSA false
// oleh handler on:change level select (roleMatrixRow) saat itu terjadi. name/
// value TETAP native agar form ter-submit apa adanya (checkbox nonaktif tak
// ikut terkirim, sama seperti checkbox biasa).
func reactiveCheckbox(name, sig, lvlSig string) g.Node {
	return h.Input(
		h.Type("checkbox"), h.Class("checkbox checkbox-sm"),
		h.Name(name), h.Value("1"),
		data.Bind(sig),
		data.Attr("disabled", "$"+lvlSig+" == 'none'"),
	)
}

// levelSelect = dropdown tingkat izin satu modul (name="level.<obj>"), versi
// STATIS (tanpa Datastar) — dipakai saat !canEdit atau baris tak reaktif.
func levelSelect(obj, current string, disabled bool) g.Node {
	attrs := []g.Node{h.Class("select select-sm"), h.Name("level." + obj)}
	if disabled {
		attrs = append(attrs, h.Disabled())
	}
	return h.Select(append(attrs, g.Group(levelOpts(current)))...)
}

// levelOpts = daftar <option> levelOptions, current terpilih. Dipakai
// levelSelect (statis) & roleMatrixRow (varian reaktif) agar daftar opsi tak
// dua kali diketik.
func levelOpts(current string) []g.Node {
	opts := make([]g.Node, 0, len(levelOptions))
	for _, o := range levelOptions {
		opts = append(opts, optionSel(o.Value, o.Label, current))
	}
	return opts
}

// roleDeleteForm = hapus peran (form POST terpisah agar submit tak tertukar
// dengan sunting). Native POST → 303 (gotcha #16). label dibedakan pemanggil
// ("Hapus" ringkas di tabel, "Hapus Peran" di halaman detail).
func roleDeleteForm(base, name, label string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/roles/"+name+"/delete"),
		h.Button(h.Type("submit"), h.Class("btn btn-sm btn-error btn-outline min-h-11"),
			g.Text(label)),
	)
}
