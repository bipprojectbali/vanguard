package handler

import (
	"go_starter/internal/authz"
	"go_starter/internal/ui/pages/panel"
)

// roles_card.go — pembangun kartu peran (buildRoleCard/roleModuleRows), label
// cakupan, opsi cakupan bisnis, dan pemetaan pesan. Dipisah dari roles_view.go
// (gate canManageRoles + prep data baris/matriks izin) agar keduanya di bawah
// ambang tipe Route/Handler (150). Satu paket.
// buildRoleCard merakit kartu SATU peran (definisi + matriks) untuk halaman
// detail/edit.
//
// Peran is_system (admin) DIRENDER TERKUNCI: Modules dibiarkan nil, view
// menampilkan keterangan "akses penuh" alih-alih matriks. Alasannya bukan sekadar
// kosmetik — admin diwakili glob crm:* di enforcer, yang tak terpetakan ke kolom
// per-modul; mencoba merendernya sebagai matriks akan menampilkan semua sel
// "None" yang KELIRU (seakan admin tak punya akses).
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
// Level: write bila ada write, else read bila ada read, else none. write⊇read
// (business.conf) jadi "write" tak perlu menyimpan read terpisah. approve hanya
// dibaca untuk modul yang mendukungnya (ModuleCanApprove) — sel approve modul
// lain tak bermakna dan tak dirender. arr (BL-58, visibilitas ARR) sama: hanya
// dibaca untuk modul ber-CanARR (Subscriptions).
func roleModuleRows(cells map[string]*permCell) []panel.RoleModulePerm {
	mods := authz.CRMModules()
	rows := make([]panel.RoleModulePerm, 0, len(mods))
	for _, m := range mods {
		level := "none"
		approve := false
		arr := false
		if c := cells[m.Obj]; c != nil {
			switch {
			case c.write:
				level = "write"
			case c.read:
				level = "read"
			}
			approve = c.approve && m.CanApprove
			arr = c.arr && m.CanARR
		}
		rows = append(rows, panel.RoleModulePerm{
			Obj:        m.Obj,
			Label:      m.Label,
			CanApprove: m.CanApprove,
			CanARR:     m.CanARR,
			Level:      level,
			Approve:    approve,
			ARR:        arr,
		})
	}
	return rows
}

// businessScopeOptions = tiga tingkat cakupan data F3 + labelnya (Bahasa
// Indonesia). Nilai dari konstanta authz (sumber sah), label = aturan tampilan
// yang tinggal di handler, bukan view. Dipakai dropdown create & edit peran.
func businessScopeOptions() []panel.ScopeOption {
	return []panel.ScopeOption{
		{Value: authz.DataScopeAll, Label: "Semua desa di workspace"},
		{Value: authz.DataScopeOwn, Label: "Hanya desa yang ditugaskan"},
		{Value: authz.DataScopeNone, Label: "Tak melihat desa lewat kepemilikan"},
	}
}

// rolesMsg memetakan ?ok= → pesan sukses panel peran. Dipisah dari wsErrMsg agar
// alert sukses & galat tak pernah tertukar variannya (pola accountsMsg).
func rolesMsg(code string) string {
	switch code {
	case "created":
		return "Peran ditambahkan."
	case "saved":
		return "Perubahan peran disimpan."
	case "deleted":
		return "Peran dihapus."
	case "fsec_saved":
		// Section Field Security (sumbu F4) berbagi region alert halaman /roles;
		// kode terpisah agar pesannya spesifik, tak tertukar dengan "Perubahan peran".
		return "Kebijakan Field Security disimpan dan berlaku seketika."
	default:
		return ""
	}
}
