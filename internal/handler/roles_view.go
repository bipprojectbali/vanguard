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
	read, write, approve, arr bool
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
		case "arr":
			c.arr = true
		}
	}
	return byRole
}
