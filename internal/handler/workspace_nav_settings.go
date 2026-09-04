package handler

import (
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"maragu.dev/gomponents/html"
)

// workspace_nav_settings.go — grup nav Settings (wireframe 9), dipisah dari
// workspace_nav.go (rule #8 file-health) begitu jumlah grup nested bertambah.

// workspaceSettingsGroup merakit grup Settings (wireframe 9). Muncul bila user
// punya MINIMAL satu izin pengaturan — tanpa itu grup tak menawarkan apa pun,
// jadi disembunyikan seutuhnya. Tiap anak enabled mengikuti izin gerbang
// halamannya (nol menu hantu); Automation & Integrations placeholder disabled.
// nil → user tak berhak melihat grup sama sekali.
func workspaceSettingsGroup(slug string, canMembers, canRoles, canSettings bool) *ui.NavItem {
	if !canMembers && !canRoles && !canSettings {
		return nil
	}
	children := make([]ui.NavItem, 0, 5)
	// Identitas workspace (ganti nama) & daur hidup (arsip/hapus) TAK lagi punya
	// item di sini — dipindah ke /dev/workspaces/{id} sebagai wewenang platform
	// (BL-53). Yang tersisa di grup Settings ruang kerja: Anggota, Peran, dan
	// Customization (format kode) — semuanya memang milik pengelola workspace.

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
