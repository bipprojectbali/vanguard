package handler

import (
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"maragu.dev/gomponents/html"
)

// workspace_nav.go — menu sidebar ruang kerja, diselaraskan dengan wireframe CRM
// (label Inggris, urutan modul persis wireframe). Dipisah dari render.go (yang
// sudah lewat ambang file-health): "menu apa yang tampil" adalah concern sendiri
// yang berubah tiap modul baru mendarat.
//
// Tiga aturan tampil, semuanya demi "nol menu hantu — pintu yang tampil tak
// pernah ditolak saat diketuk":
//   - Modul BERBACKEND (Accounts, Contacts, Settings) muncul mengikuti IZIN yang
//     SAMA dengan gerbang halamannya — bukan sekadar keberadaan route.
//   - Modul wireframe yang backend-nya BELUM ada (Sales … Reports, Automation,
//     Integrations) tetap ditampilkan terurut TAPI disabled (teredam, tanpa label
//     tambahan) — peta jalan terlihat, pintunya jujur belum terbuka.
//   - Settings jadi GRUP bersarang (wireframe 9): satu header, anak-anaknya
//     User Management / Roles & Permissions / Customization + dua placeholder.

// workspaceNav membangun menu ruang kerja untuk slug + izin yang sudah dihitung
// handler. Semua href bergantung SLUG (0004) → menu tak bisa jadi var paket.
//
// Izin dioper terpisah (bukan satu `canManage`) karena keduanya TIDAK identik —
// di mode single admin boleh menyunting workspace tapi keanggotaan dinilai
// sendiri — jadi menyatukannya akan membuat salah satu menu berbohong.
func workspaceNav(slug string, canMembers, canSettings, canAccounts, canContacts, canRoles bool) []ui.NavItem {
	items := []ui.NavItem{
		{Label: "Dashboard", Href: wsPath(slug, ""), Icon: lucide.House(html.Class("size-4"))},
	}
	// Accounts (hub desa) — pemegang peran CRM (F2 read crm:accounts). Sumber izin
	// SAMA dengan gerbang AccountsList.
	if canAccounts {
		items = append(items, ui.NavItem{
			Label: "Accounts", Href: wsPath(slug, "/accounts"),
			Icon: lucide.MapPin(html.Class("size-4")),
		})
	}
	// Contacts (orang lintas-desa) — objek Casbin terpisah (crm:contacts), izinnya
	// dinilai sendiri. Sumber izin SAMA dengan gerbang ContactsAll.
	if canContacts {
		items = append(items, ui.NavItem{
			Label: "Contacts", Href: wsPath(slug, "/contacts"),
			Icon: lucide.Contact(html.Class("size-4")),
		})
	}
	// Modul berwireframe tapi belum berbackend — urutan persis wireframe, disabled.
	// Ditampilkan agar peta jalan produk terlihat utuh di sidebar sejak awal.
	items = append(items,
		ui.NavItem{Label: "Sales", Icon: lucide.TrendingUp(html.Class("size-4")), Disabled: true},
		ui.NavItem{Label: "Subscriptions", Icon: lucide.RefreshCw(html.Class("size-4")), Disabled: true},
		ui.NavItem{Label: "Customer Success", Icon: lucide.Heart(html.Class("size-4")), Disabled: true},
		ui.NavItem{Label: "Activities", Icon: lucide.Activity(html.Class("size-4")), Disabled: true},
		ui.NavItem{Label: "Reports", Icon: lucide.ChartColumn(html.Class("size-4")), Disabled: true},
	)
	if grp := workspaceSettingsGroup(slug, canMembers, canRoles, canSettings); grp != nil {
		items = append(items, *grp)
	}
	return items
}

// workspaceSettingsGroup merakit grup Settings (wireframe 9). Muncul bila user
// punya MINIMAL satu izin pengaturan — tanpa itu grup tak menawarkan apa pun,
// jadi disembunyikan seutuhnya. Tiap anak enabled mengikuti izin gerbang
// halamannya (nol menu hantu); Automation & Integrations placeholder disabled.
// nil → user tak berhak melihat grup sama sekali.
func workspaceSettingsGroup(slug string, canMembers, canRoles, canSettings bool) *ui.NavItem {
	if !canMembers && !canRoles && !canSettings {
		return nil
	}
	children := make([]ui.NavItem, 0, 6)
	// Workspace Settings → /settings (identitas workspace/app + daur hidup:
	// arsip/suspend). TAK ada di submenu wireframe, tapi halamannya nyata & penting;
	// ditaruh paling atas grup, gated canEditWorkspace (sumber izin sama dengan
	// gerbang WorkspaceSettings). Tanpa ini rename/arsip tak punya jalan menu.
	if canSettings {
		children = append(children, ui.NavItem{
			Label: "Workspace Settings", Href: wsPath(slug, "/settings"),
			Icon: lucide.Building2(html.Class("size-4")),
		})
	}
	// User Management → /members (canManageMembers, sama dengan gerbang MembersPage).
	if canMembers {
		children = append(children, ui.NavItem{
			Label: "User Management", Href: wsPath(slug, "/members"),
			Icon: lucide.Users(html.Class("size-4")),
		})
	}
	// Roles & Permissions → /roles (canManageRoles, objek crm:roles tersendiri).
	if canRoles {
		children = append(children, ui.NavItem{
			Label: "Roles & Permissions", Href: wsPath(slug, "/roles"),
			Icon: lucide.ShieldCheck(html.Class("size-4")),
		})
	}
	// Customization → /codes (format kode entity per-workspace; canEditWorkspace).
	if canSettings {
		children = append(children, ui.NavItem{
			Label: "Customization", Href: wsPath(slug, "/codes"),
			Icon: lucide.Wrench(html.Class("size-4")),
		})
	}
	children = append(children,
		ui.NavItem{Label: "Automation / Workflow", Icon: lucide.Workflow(html.Class("size-4")), Disabled: true},
		ui.NavItem{Label: "Integrations", Icon: lucide.Plug(html.Class("size-4")), Disabled: true},
	)
	return &ui.NavItem{
		Label: "Settings", Icon: lucide.Settings(html.Class("size-4")), Children: children,
	}
}
