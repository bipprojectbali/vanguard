package handler

import (
	"go_starter/internal/authz"
	"go_starter/internal/ui/pages/panel"
)

// roles_card.go — pembangun kartu peran (buildRoleCard/roleModuleRows), label
// cakupan, dan level modul. Dipisah dari roles_view.go (gate canManageRoles +
// prep data baris/matriks izin) agar keduanya di bawah ambang tipe
// Route/Handler (150). Satu paket.
// buildRoleCard merakit kartu SATU peran (definisi + matriks) untuk halaman
// detail/edit.
//
// Peran is_system (admin) DIRENDER TERKUNCI: Modules dibiarkan nil, view
// menampilkan keterangan "akses penuh" alih-alih matriks. Alasannya bukan sekadar
// kosmetik — admin diwakili glob crm:* di enforcer, yang tak terpetakan ke kolom
// per-modul; mencoba merendernya sebagai matriks akan menampilkan semua sel
// "None" yang KELIRU (seakan admin tak punya akses).
//
// permissionSetRows/crudMark/buildPermissionSetView (blok B), businessScopeOptions,
// dan rolesMsg dipindah ke roles_permset.go — dipisah krn ambang File Health
// yang sama.
func buildRoleCard(name, display, description, scope string, isSystem bool, byRole map[string]map[string]*permCell) panel.RoleCard {
	card := panel.RoleCard{
		Name:        name,
		DisplayName: display,
		Description: description,
		DataScope:   scope,
		IsSystem:    isSystem,
	}
	if !isSystem {
		card.Modules = roleModuleRows(byRole[name])
	}
	return card
}

// scopeLabel menerjemahkan nilai cakupan data F3 → label Indonesia untuk tabel.
// Sumber sama dengan dropdown (businessScopeOptions) agar tabel & form tak pernah
// menyebut cakupan yang sama dengan istilah berbeda. Nilai tak dikenal →
// dikembalikan apa adanya (jujur; lebih baik daripada kosong yang menyesatkan).
func scopeLabel(value string) string {
	for _, o := range businessScopeOptions() {
		if o.Value == value {
			return o.Label
		}
	}
	return value
}

// roleModuleRows menurunkan satu baris matriks per modul CRM (urut menu §4) dari
// sel izin peran. cells boleh nil (peran baru tanpa izin) → semua modul "none".
//
// Level: write bila ada write DAN modul WriteEnforced, else read bila ada write
// ATAU read, else none. write⊇read (business.conf) jadi "write" tak perlu
// menyimpan read terpisah. Modul !WriteEnforced (dashboard, subscriptions,
// activities, 4 Reports) DIRENDAHKAN ke "read" walau sel DB-nya "write" (mis.
// grant default Manager crm:subscriptions write, business_defaults.go) — bukan
// kehilangan akses (write tetap mencakup read di enforcer), sekadar memastikan
// Level yang dioper ke view SELALU punya <option> yang cocok setelah editor
// berhenti menawarkan "Kelola" untuk modul ini (role_edit_levels.go); tanpa ini
// select tampak default ke opsi pertama ("Tak ada") dan submit berikutnya diam-
// diam menghapus akses read yang sebenarnya masih berlaku. approve hanya
// dibaca untuk modul yang mendukungnya (ModuleCanApprove) — sel approve modul
// lain tak bermakna dan tak dirender. arr (BL-58, visibilitas ARR) sama: hanya
// dibaca untuk modul ber-CanARR (Subscriptions).
func roleModuleRows(cells map[string]*permCell) []panel.RoleModulePerm {
	mods := authz.CRMModules()
	rows := make([]panel.RoleModulePerm, 0, len(mods))
	for _, m := range mods {
		level := moduleLevel(cells[m.Obj], m.WriteEnforced)
		approve := false
		arr := false
		if c := cells[m.Obj]; c != nil {
			approve = c.approve && m.CanApprove
			arr = c.arr && m.CanARR
		}
		rows = append(rows, panel.RoleModulePerm{
			Obj:           m.Obj,
			Label:         m.Label,
			CanApprove:    m.CanApprove,
			CanARR:        m.CanARR,
			ARREligible:   authz.ModuleARRGate(m.Obj),
			WriteEnforced: m.WriteEnforced,
			Level:         level,
			Approve:       approve,
			ARR:           arr,
		})
	}
	return rows
}

// moduleLevel menurunkan Level ("none"/"read"/"write") satu sel dari c
// (nil → "none"), direndahkan ke "read" bila !writeEnforced walau c.write true
// — satu fungsi dipakai roleModuleRows & permissionSetRows agar keduanya tak
// bisa menyimpang (lihat rasional lengkap di roleModuleRows).
func moduleLevel(c *permCell, writeEnforced bool) string {
	if c == nil {
		return "none"
	}
	switch {
	case c.write && writeEnforced:
		return "write"
	case c.write || c.read:
		return "read"
	default:
		return "none"
	}
}
