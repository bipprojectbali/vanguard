package handler

import (
	"context"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// roles_view.go — gerbang + pemetaan model DB → data siap-render untuk panel
// manajemen peran CRM (/w/{slug}/roles). Murni-data: view menerima matriks yang
// sudah dihitung, tak memanggil authz sendiri (konvensi view murni-data).

// canManageRoles = gerbang panel manajemen peran (sumbu BISNIS, objek
// "crm:roles" aksi write). SATU sumber untuk item menu "Peran" DAN gate halaman
// RolesPage/aksinya — keduanya WAJIB menyebut objek/aksi yang sama, agar menu tak
// pernah menawarkan pintu yang lalu ditolak 403 (pola canViewAccounts).
//
// crm:roles dimiliki admin lewat glob crm:* (business_defaults). Peran lain tak
// memilikinya kecuali operator sengaja memberikannya — dan crm:roles SENGAJA tak
// jadi kolom matriks editor (business_defaults), jadi pemberian itu hanya bisa
// lewat seed/DB, bukan lewat panel ini. Deny-default: peran tanpa izin → false.
func canManageRoles(ctx context.Context) bool {
	return authz.CanBusiness(ctx, "crm:roles", "write")
}

// permKey menyatukan tiga aksi satu sel matriks (obj × role) jadi satu status
// tampilan. read & write dilipat jadi satu tingkat "Level" karena matcher bisnis
// membuat write MENCAKUP read (business.conf): menyimpan keduanya sebagai dua
// kotak centang akan membingungkan (write tanpa read tak bermakna). approve
// berdiri sendiri — tegak lurus level (bisa approve tanpa write, walau jarang).
type permCell struct {
	read, write, approve bool
}

// roleRows memetakan definisi peran → baris tabel daftar (/roles). Ringkas:
// hanya identitas + label cakupan; matriks izin TAK dibaca di sini (ada di
// halaman detail RoleEditPage). ScopeLabel = aturan tampilan, dihitung di handler
// bukan view (konvensi view murni-data).
func roleRows(roles []db.ListBusinessRolesRow) []panel.RoleRow {
	rows := make([]panel.RoleRow, 0, len(roles))
	for _, r := range roles {
		rows = append(rows, panel.RoleRow{
			Name:        r.Name,
			DisplayName: r.DisplayName,
			Description: r.Description,
			ScopeLabel:  scopeLabel(r.DataScope),
			MemberCount: r.MemberCount,
			IsSystem:    r.IsSystem,
		})
	}
	return rows
}

// permsByRole mengelompokkan baris izin tenant menjadi role → obj → sel. Dibangun
// sekali, dibaca per-modul saat merakit kartu satu peran (buildRoleCard).
func permsByRole(perms []db.ListBusinessRolePermissionsByTenantRow) map[string]map[string]*permCell {
	byRole := make(map[string]map[string]*permCell)
	for _, p := range perms {
		m := byRole[p.Role]
		if m == nil {
			m = make(map[string]*permCell)
			byRole[p.Role] = m
		}
		c := m[p.Obj]
		if c == nil {
			c = &permCell{}
			m[p.Obj] = c
		}
		switch p.Act {
		case "read":
			c.read = true
		case "write":
			c.write = true
		case "approve":
			c.approve = true
		}
	}
	return byRole
}

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
// lain tak bermakna dan tak dirender.
func roleModuleRows(cells map[string]*permCell) []panel.RoleModulePerm {
	mods := authz.CRMModules()
	rows := make([]panel.RoleModulePerm, 0, len(mods))
	for _, m := range mods {
		level := "none"
		approve := false
		if c := cells[m.Obj]; c != nil {
			switch {
			case c.write:
				level = "write"
			case c.read:
				level = "read"
			}
			approve = c.approve && m.CanApprove
		}
		rows = append(rows, panel.RoleModulePerm{
			Obj:        m.Obj,
			Label:      m.Label,
			CanApprove: m.CanApprove,
			Level:      level,
			Approve:    approve,
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
	default:
		return ""
	}
}
