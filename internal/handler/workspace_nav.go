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
func workspaceNav(slug string, canMembers, canSettings, canAccounts, canContacts, canLeads, canDeals, canSalesActivity, canPlans, canRoles bool) []ui.NavItem {
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
	items = append(items, workspaceSubscriptionsGroup(slug, canPlans))
	// Modul berwireframe tapi belum berbackend — urutan persis wireframe, disabled.
	// Ditampilkan agar peta jalan produk terlihat utuh di sidebar sejak awal.
	items = append(items,
		ui.NavItem{Label: "Customer Success", Icon: lucide.Heart(html.Class("size-4")), Disabled: true},
		ui.NavItem{Label: "Activities", Icon: lucide.Activity(html.Class("size-4")), Disabled: true},
		ui.NavItem{Label: "Reports", Icon: lucide.ChartColumn(html.Class("size-4")), Disabled: true},
	)
	if grp := workspaceSettingsGroup(slug, canMembers, canRoles, canSettings); grp != nil {
		items = append(items, *grp)
	}
	return items
}

// workspaceSalesGroup merakit grup Sales (wireframe 4). Berbeda dari Settings,
// grup ini SELALU tampil (peta jalan produk terlihat): anak Leads/Deals/Quotes
// enabled mengikuti izin gerbang halamannya (sumber izin SAMA dengan
// canViewLeads/canViewDeals — nol menu hantu), Activities placeholder disabled.
func workspaceSalesGroup(slug string, canLeads, canDeals, canSalesActivity bool) ui.NavItem {
	children := make([]ui.NavItem, 0, 4)
	// Leads → /leads (canViewLeads, objek crm:leads). Disabled bila tak berhak,
	// tetap tampil agar posisi modul di peta jalan terlihat.
	leads := ui.NavItem{Label: "Leads", Icon: lucide.UserPlus(html.Class("size-4"))}
	if canLeads {
		leads.Href = wsPath(slug, "/leads")
	} else {
		leads.Disabled = true
	}
	// Deals → /deals (canViewDeals, objek crm:deals).
	deals := ui.NavItem{Label: "Deals", Icon: lucide.Handshake(html.Class("size-4"))}
	if canDeals {
		deals.Href = wsPath(slug, "/deals")
	} else {
		deals.Disabled = true
	}
	// Quotes → /quotes (daftar lintas-deal). Izin baca SAMA dengan deal (quote
	// mewarisi ownership F3 dari deal induk) → gate canDeals.
	quotes := ui.NavItem{Label: "Quotes", Icon: lucide.FileText(html.Class("size-4"))}
	if canDeals {
		quotes.Href = wsPath(slug, "/quotes")
	} else {
		quotes.Disabled = true
	}
	// Sales Activities → /activities (canViewSalesActivity, objek
	// crm:sales_activity). Berbeda dari "Activities" top-level (objek M7 global,
	// tetap disabled): ini VIEW TERFILTER Sales (activity_context='sales').
	salesAct := ui.NavItem{Label: "Sales Activities", Icon: lucide.Activity(html.Class("size-4"))}
	if canSalesActivity {
		salesAct.Href = wsPath(slug, "/activities")
	} else {
		salesAct.Disabled = true
	}
	children = append(children, leads, deals, quotes, salesAct)
	return ui.NavItem{
		Label: "Sales", Icon: lucide.TrendingUp(html.Class("size-4")), Children: children,
	}
}

// workspaceSubscriptionsGroup merakit grup Subscriptions (wireframe 5). Seperti
// Sales, SELALU tampil (peta jalan). Plans & Pricing → /plans enabled per izin
// (sumber izin SAMA dengan canViewPlans, objek crm:plans — nol menu hantu); Active
// Subscriptions/Renewals/Churn placeholder disabled sampai backend-nya mendarat
// (M5-3b dst). Urutan mengikuti nomor menu §4 (crmModules).
func workspaceSubscriptionsGroup(slug string, canPlans bool) ui.NavItem {
	children := make([]ui.NavItem, 0, 4)
	children = append(children,
		ui.NavItem{Label: "Active Subscriptions", Icon: lucide.RefreshCw(html.Class("size-4")), Disabled: true},
		ui.NavItem{Label: "Renewals", Icon: lucide.CalendarClock(html.Class("size-4")), Disabled: true},
	)
	// Plans & Pricing → /plans (canViewPlans, objek crm:plans). Berbackend sejak
	// M5-3a; disabled bila tak berhak, tetap tampil agar posisi modul terlihat.
	plans := ui.NavItem{Label: "Plans & Pricing", Icon: lucide.Tag(html.Class("size-4"))}
	if canPlans {
		plans.Href = wsPath(slug, "/plans")
	} else {
		plans.Disabled = true
	}
	children = append(children, plans,
		ui.NavItem{Label: "Churn / Cancellations", Icon: lucide.UserMinus(html.Class("size-4")), Disabled: true},
	)
	return ui.NavItem{
		Label: "Subscriptions", Icon: lucide.RefreshCw(html.Class("size-4")), Children: children,
	}
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
