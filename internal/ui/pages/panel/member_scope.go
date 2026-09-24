package panel

import (
	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// member_scope.go — section "Cakupan Jenis Anggota" (BL-171) di dalam
// "Pengaturan Tambahan" halaman detail peran: dua checkbox (internal/
// eksternal) yang menentukan anggota BERJENIS APA yang boleh dilihat/dikelola
// peran ini lewat akses baru "User Management" (crm:members,
// business_modules.go). BEDA dari business_roles.kind (BL-170, dropdown
// "Jenis Anggota" — peran ini UNTUK anggota internal/eksternal): setting ini
// soal DATA anggota siapa yang boleh dilihat, diberi label terpisah "Cakupan
// Jenis Anggota" agar tak rancu. Melebur ke form matriks yang SAMA
// (mscope_present, pola persis fsec_present) — beda dari Field Security yang
// punya gate terpisah (crm:field_security): crm:members gatenya SAMA dengan
// matriks (canManageRoles), jadi tak perlu form/endpoint sendiri.

// MemberScopeRoleView = nilai cakupan SATU peran, siap-render. Baris DB tak
// ada (belum dikonfigurasi) → handler (roles_page.go) sudah menerjemahkannya
// jadi true/true (default terbuka — beda dari FLS yang defaultnya tertutup,
// lihat komentar migrasi 00052) sebelum sampai ke view.
type MemberScopeRoleView struct {
	CanViewInternal bool
	CanViewExternal bool
}

// memberScopeRows merender dua baris checklist cakupan (dipanggil
// additionalSettings, digabung ke settingsGroup "Anggota") + penanda hidden
// mscope_present. additionalSettings HANYA dipanggil dari cabang peran
// KUSTOM di role_edit.go (peran sistem return lebih awal), jadi tak perlu
// flag "reactive" terpisah seperti FieldSecurityRoleView (yang JUGA dipakai
// berdiri sendiri utk peran sistem) — canEdit saja cukup, pola sama
// financeRows (additionalSettings). canEdit → checkbox dibind ke signal
// msp_internal/msp_external, dinonaktifkan saat $lvl_members==='none'
// (lvl_members dideklarasikan roleMatrixRow — signal GLOBAL, tak terikat
// posisi DOM, pola sama fls_view/fls_edit) dan DIPAKSA true/true oleh
// roleMatrixRow saat level BARU SAJA aktif dari "Tak ada" (auto-buka penuh —
// beda dari FLS yang mereset ke false: di sini mempersempit cakupan adalah
// keputusan SADAR admin, bukan default saat mengaktifkan modul). !canEdit →
// checkbox statis (fieldSecurityCheck, sama seperti FLS versi baca-saja).
func memberScopeRows(v MemberScopeRoleView, canEdit bool) (rows []g.Node, attrs []g.Node) {
	var internalCell, externalCell g.Node
	if canEdit {
		attrs = append(attrs, data.Signals(map[string]any{
			"msp_internal": v.CanViewInternal,
			"msp_external": v.CanViewExternal,
		}))
		internalCell = reactiveMemberScopeCheck("mscope_internal", "msp_internal", "msp_external")
		externalCell = reactiveMemberScopeCheck("mscope_external", "msp_external", "msp_internal")
	} else {
		internalCell = fieldSecurityCheck("mscope_internal", v.CanViewInternal, canEdit)
		externalCell = fieldSecurityCheck("mscope_external", v.CanViewExternal, canEdit)
	}
	rows = []g.Node{
		settingRow(internalCell, "Lihat anggota internal", g.Text("")),
		settingRow(externalCell, "Lihat anggota eksternal", g.Text("")),
		h.Input(h.Type("hidden"), h.Name("mscope_present"), h.Value("1")),
	}
	return rows, attrs
}

// reactiveMemberScopeCheck = satu checkbox cakupan, dibind ke signal sig,
// dinonaktifkan saat $lvl_members==='none'. onChange: bila DICENTANG-OFF
// (!evt.target.checked) padahal signal PASANGAN (otherSig) juga sudah false,
// batalkan (paksa checkbox tetap tercentang) — mencegah kombinasi
// false/false yang ditolak CHECK DB (msp_at_least_one_chk) tanpa roundtrip
// server dulu; server (RoleUpdate) tetap penjaga sesungguhnya (coercion +
// CHECK DB), ini murni UX.
func reactiveMemberScopeCheck(name, sig, otherSig string) g.Node {
	onChange := "!evt.target.checked&&!$" + otherSig + "&&(evt.target.checked=true,$" + sig + "=true)"
	return h.Input(
		h.Type("checkbox"), h.Class("checkbox checkbox-sm"),
		h.Name(name), h.Value("1"),
		data.Bind(sig),
		data.Attr("disabled", "$lvl_members==='none'"),
		data.On("change", onChange),
	)
}
