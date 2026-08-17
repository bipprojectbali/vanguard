package handler

import (
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"maragu.dev/gomponents/html"
)

// workspace_nav.go — menu sidebar ruang kerja, diselaraskan dengan wireframe CRM
// (label Inggris, urutan modul persis wireframe). Dipisah dari render.go (yang
// sudah lewat ambang file-health): "menu apa yang tampil" adalah concern sendiri
// yang berubah tiap modul baru mendarat. Orkestrator ini SENDIRI dipecah lagi per
// grup (workspace_nav_sales.go/_subscriptions.go/_cs.go/_settings.go) begitu
// jumlah grup nested membuatnya lewat ambang file-health (rule #8) — jangan
// gabungkan grup baru ke sini, tambah file grup baru.
//
// Tiga aturan tampil, semuanya demi "nol menu hantu — pintu yang tampil tak
// pernah ditolak saat diketuk":
//   - Modul BERBACKEND (Accounts, Contacts, Settings) muncul mengikuti IZIN yang
//     SAMA dengan gerbang halamannya — bukan sekadar keberadaan route.
//   - Modul wireframe yang backend-nya BELUM ada (Activities, Reports,
//     Automation, Integrations) tetap ditampilkan terurut TAPI disabled
//     (teredam, tanpa label tambahan) — peta jalan terlihat, pintunya jujur
//     belum terbuka.
//   - Settings jadi GRUP bersarang (wireframe 9): satu header, anak-anaknya
//     User Management / Roles & Permissions / Customization + dua placeholder.

// workspaceNav membangun menu ruang kerja untuk slug + izin yang sudah dihitung
// handler. Semua href bergantung SLUG (0004) → menu tak bisa jadi var paket.
//
// Izin dioper terpisah (bukan satu `canManage`) karena keduanya TIDAK identik —
// di mode single admin boleh menyunting workspace tapi keanggotaan dinilai
// sendiri — jadi menyatukannya akan membuat salah satu menu berbohong.
// workspaceNav membangun menu ruang kerja. canSalesActivity = gate CRM bisnis
// (crm:sales_activity) — untuk "Sales Activities" dalam grup Sales.
// canAllActivities = gate halaman lintas-context (/activity-log) — LEBIH LUAS:
// CRM role ATAU platform role (super_admin/staff butuh visibilitas sistem).
func workspaceNav(slug string, canMembers, canSettings, canAccounts, canContacts, canLeads, canDeals, canSalesActivity, canAllActivities, canPlans, canSubs, canSLA, canPlaybooks, canKB, canRoles, canTickets, canHealthScore, canSuccessPlans, canEngagements, canRenewals bool) []ui.NavItem {
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
	// Sales jadi GRUP bersarang (wireframe 4): Leads/Deals berbackend (enabled per
	// izin), Quotes/Activities placeholder. Selalu tampil agar peta jalan terlihat.
	items = append(items, workspaceSalesGroup(slug, canLeads, canDeals, canSalesActivity))
	// Subscriptions jadi GRUP bersarang (wireframe 5): Plans & Pricing berbackend
	// (enabled per izin), sisanya (Active Subscriptions/Renewals/Churn) placeholder.
	// Selalu tampil agar peta jalan terlihat.
	items = append(items, workspaceSubscriptionsGroup(slug, canPlans, canSubs))
	// Customer Success jadi GRUP bersarang (wireframe 6): SLA Management
	// (slice A1), Playbooks (slice A2), Knowledge Base (slice A3) & Tickets/Cases
	// (slice B2) berbackend (enabled per izin), sisanya placeholder. Selalu tampil
	// agar peta jalan terlihat.
	items = append(items, workspaceCSGroup(slug, canSLA, canPlaybooks, canKB, canTickets, canHealthScore, canSuccessPlans, canEngagements, canRenewals))
	// Activities top-level = daftar lintas-context (sales+cs+general), M7.
	// Gate LEBIH LUAS dari Sales Activities: CRM role ATAU platform role
	// (super_admin/staff butuh visibilitas sistem tanpa harus diberi business_role).
	// TODO(activities): ganti ke canViewActivities (objek crm:activities) saat M9.
	if canAllActivities {
		items = append(items, ui.NavItem{
			Label: "Activities",
			Icon:  lucide.Activity(html.Class("size-4")),
			Href:  wsPath(slug, "/activity-log"),
		})
	} else {
		items = append(items,
			ui.NavItem{Label: "Activities", Icon: lucide.Activity(html.Class("size-4")), Disabled: true})
	}
	items = append(items,
		ui.NavItem{Label: "Reports", Icon: lucide.ChartColumn(html.Class("size-4")), Disabled: true},
	)
	if grp := workspaceSettingsGroup(slug, canMembers, canRoles, canSettings); grp != nil {
		items = append(items, *grp)
	}
	return items
}
