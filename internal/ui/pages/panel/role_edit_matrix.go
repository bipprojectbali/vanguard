package panel

import (
	"strings"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// role_edit_matrix.go — tabel matriks izin per modul (roleMatrix/
// roleMatrixRow) + konteks reaktivitas checkbox "Lihat Nilai Kontrak" lintas
// modul (arrCrossModuleCtx dkk), dipisah dari role_edit.go (sudah di ambang
// batas 300 baris/tipe View) agar penambahan modul/gate ke depannya tak
// mendorongnya lewat batas.

// roleMatrix = tabel izin per modul CRM. Dibungkus ui.TableScroll agar scroll
// terkurung, tak mendorong lebar halaman di mobile (konvensi mobile-first).
// fls → diteruskan ke roleMatrixRow (lihat flsReactive di role_edit.go). BL-169: 4
// modul Reports (Sales/CS/Support/Subscription) dirender FLAT sebagai baris
// biasa, sama seperti modul lain — tanpa header grup.
//
// table-fixed + lebar tetap kolom "Akses" (w-28, dipasang di <th> — algoritma
// table-fixed browser mengambil lebar kolom dari baris PERTAMA/thead bila tak
// ada <colgroup>): TANPA ini, table-layout DEFAULT (auto) menghitung lebar tiap
// kolom dari preferred-width kontennya, dan kolom "Modul" melebar besar saat
// moduleHint (paragraf hint) muncul (baris ≥ Lihat & Active Subscriptions=Tak
// ada) — karena total lebar tabel tetap w-full, kolom "Akses" TERDESAK menyusut
// ikut mengecilkan <select>-nya secara visual (dilaporkan user: ukuran select
// tak konsisten saat hint tampil/hilang). Dengan table-fixed, lebar "Akses"
// dikunci independen dari isi kolom lain — select selalu sama besar, hint
// membungkus baris dalam lebar "Modul" yang tersisa tanpa memengaruhi kolom
// sebelahnya.
func roleMatrix(rc RoleCard, canEdit bool, fls bool, actx arrCrossModuleCtx) g.Node {
	subsLevel := moduleLevelOf(rc.Modules, "crm:subscriptions")
	rows := make([]g.Node, 0, len(rc.Modules))
	for _, m := range rc.Modules {
		rows = append(rows, roleMatrixRow(m, canEdit, fls, actx, subsLevel))
	}
	return ui.TableScroll(h.Table(
		h.Class("w-full text-sm table-fixed"),
		h.THead(h.Tr(
			h.Class("border-b border-base-300 text-left text-base-content/70"),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Modul")),
			h.Th(h.Class("py-2 font-medium w-28"), g.Text("Akses")),
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
func roleMatrixRow(m RoleModulePerm, canEdit bool, fls bool, actx arrCrossModuleCtx, subsLevel string) g.Node {
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
		h.Td(h.Class("py-2 pr-4 break-words"), g.Text(m.Label), moduleHint(m, canEdit, subsLevel)),
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
