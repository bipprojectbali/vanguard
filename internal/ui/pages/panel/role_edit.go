package panel

import (
	"strings"

	"go_starter/internal/ui"

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
	// ARREligible: true → level modul INI ikut menentukan buka/tutup checkbox
	// "Lihat Nilai Kontrak" (authz.ARRGateObjects, disalin handler dari sana) —
	// checkbox itu sendiri TETAP satu, dirender hanya di baris CanARR.
	ARREligible bool
	// WriteEnforced: true → backend punya titik enforcement "write" nyata utk
	// modul ini, opsi "Kelola" dirender (levelOpts, role_edit_levels.go).
	// false → hanya "Tak ada"/"Lihat" (authz.ModuleWriteEnforced, disalin
	// handler dari sana; lihat rasional lengkap di role_edit_levels.go).
	WriteEnforced bool
	Level         string
	Approve       bool
	ARR           bool // tersetel → kotak "Lihat ARR" tercentang
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

	actx := buildARRCrossModuleCtx(rc)
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
		roleMatrix(rc, canEdit, flsReactive(fsec), actx),
		additionalSettings(rc, canEdit, fsec, actx),
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
func roleMatrix(rc RoleCard, canEdit bool, fls bool, actx arrCrossModuleCtx) g.Node {
	rows := make([]g.Node, 0, len(rc.Modules))
	for _, m := range rc.Modules {
		rows = append(rows, roleMatrixRow(m, canEdit, fls, actx))
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
func roleMatrixRow(m RoleModulePerm, canEdit bool, fls bool, actx arrCrossModuleCtx) g.Node {
	levelCell := levelSelect(m.Obj, m.Level, !canEdit, m.WriteEnforced)
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
			// Bukan hasLevel (baris ini sendiri) — actx.unlockedNow lintas
			// authz.ARRGateObjects (lihat arrCrossModuleCtx).
			arrSig := "arr_" + suffix
			signals[arrSig] = m.ARR && actx.unlockedNow
		}
		var stmts []string
		if len(resets) > 0 {
			stmts = append(stmts, "evt.target.value==='none'&&("+strings.Join(resets, ",")+")")
		}
		if m.ARREligible && actx.resetStmt != "" {
			// Fires di SETIAP baris gate (bukan cuma baris CanARR sendiri) —
			// begitu SEMUA modul gate balik "none", paksa checkbox ARR ke false.
			stmts = append(stmts, actx.resetStmt)
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
		levelCell = h.Select(append(selectAttrs, g.Group(levelOpts(m.Level, m.WriteEnforced)))...)
		rowAttrs = append(rowAttrs, data.Signals(signals))
	}

	return h.Tr(append(rowAttrs,
		h.Class("border-b border-base-300/50"),
		h.Td(h.Class("py-2 pr-4"), g.Text(m.Label)),
		h.Td(h.Class("py-2"), levelCell),
	)...)
}

// arrCrossModuleCtx = konteks reaktivitas checkbox "Lihat Nilai Kontrak" LINTAS
// MODUL (dibangun SEKALI di RoleEdit lewat buildARRCrossModuleCtx, dipakai
// roleMatrix/roleMatrixRow & additionalSettings) — checkbox itu TETAP SATU
// (dirender di baris CanARR, Subscriptions), tapi level baris ITU SENDIRI
// bukan lagi satu-satunya syarat aktif: authz.ARRGateObjects (business_defaults.go)
// melebarkan gate ke modul lain di backend (readRoleMatrix, roles_rest.go), dan
// ini menyalin logika OR yang SAMA persis di sisi UI — tanpanya checkbox bisa
// tampak bisa dicentang di editor tapi diam-diam dibuang backend saat submit
// (atau sebaliknya: tampak terkunci padahal backend sudah mengizinkan).
type arrCrossModuleCtx struct {
	disabledExpr string // "$lvl_x=='none'&&$lvl_y=='none'&&..." semua modul gate — kosong bila tak ada baris CanARR
	resetStmt    string // "(disabledExpr)&&($arr_<sig>=false)" — dipasang di on:change TIAP baris gate
	unlockedNow  bool   // true → SALAH SATU modul gate level-nya (saat render) bukan "none"
}

// buildARRCrossModuleCtx menyisir rc.Modules SEKALI: baris ber-ARREligible
// menyusun disabledExpr/unlockedNow, baris ber-CanARR menentukan signal target
// (arr_<suffix>) yang direset. Urut ikut urutan rc.Modules (= authz.CRMModules,
// stabil) agar keluarannya deterministik.
func buildARRCrossModuleCtx(rc RoleCard) arrCrossModuleCtx {
	var gateObjs []string
	var arrSig string
	unlocked := false
	for _, m := range rc.Modules {
		if m.ARREligible {
			gateObjs = append(gateObjs, m.Obj)
			if m.Level != "none" {
				unlocked = true
			}
		}
		if m.CanARR {
			arrSig = "arr_" + moduleSignal(m.Obj)
		}
	}
	if len(gateObjs) == 0 || arrSig == "" {
		return arrCrossModuleCtx{}
	}
	parts := make([]string, len(gateObjs))
	for i, obj := range gateObjs {
		parts[i] = "$lvl_" + moduleSignal(obj) + "=='none'"
	}
	disabled := strings.Join(parts, "&&")
	return arrCrossModuleCtx{
		disabledExpr: disabled,
		resetStmt:    "(" + disabled + ")&&($" + arrSig + "=false)",
		unlockedNow:  unlocked,
	}
}

// moduleSignal mengubah objek Casbin ("crm:renewals") jadi sufiks aman-sinyal
// ("renewals") — objek modul CRM SELALU berawalan "crm:" (crmModules,
// business_defaults.go), sisanya huruf kecil+underscore saja, jadi aman jadi
// identifier Datastar tanpa escaping tambahan.
func moduleSignal(obj string) string {
	return strings.TrimPrefix(obj, "crm:")
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
