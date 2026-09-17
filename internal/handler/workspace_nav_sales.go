package handler

import (
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"maragu.dev/gomponents/html"
)

// workspace_nav_sales.go — grup nav Sales (wireframe 4), dipisah dari
// workspace_nav.go (rule #8 file-health) begitu jumlah grup nested bertambah.

// workspaceSalesGroup merakit grup Sales (wireframe 4). Sama pola dengan
// Settings (workspace_nav_settings.go): nil bila user tak berhak ke SATU PUN
// anak (grup kosong tak menawarkan apa pun, disembunyikan total) — anak yang
// tersisa enabled mengikuti izin gerbang halamannya masing-masing (sumber
// izin SAMA dengan canViewLeads/canViewDeals — nol menu hantu). Anak tanpa
// izin tak ditampilkan sama sekali (bukan disabled).
func workspaceSalesGroup(slug string, canLeads, canDeals, canSalesActivity bool) *ui.NavItem {
	if !canLeads && !canDeals && !canSalesActivity {
		return nil
	}
	children := make([]ui.NavItem, 0, 4)
	// Leads → /leads (canViewLeads, objek crm:leads).
	if canLeads {
		children = append(children, ui.NavItem{
			Label: "Leads", Href: wsPath(slug, "/leads"),
			Icon: lucide.UserPlus(html.Class("size-4")),
		})
	}
	// Deals → /deals (canViewDeals, objek crm:deals).
	if canDeals {
		children = append(children, ui.NavItem{
			Label: "Deals", Href: wsPath(slug, "/deals"),
			Icon: lucide.Handshake(html.Class("size-4")),
		})
	}
	// Quotes → /quotes (daftar lintas-deal). Izin baca SAMA dengan deal (quote
	// mewarisi ownership F3 dari deal induk) → gate canDeals.
	if canDeals {
		children = append(children, ui.NavItem{
			Label: "Quotes", Href: wsPath(slug, "/quotes"),
			Icon: lucide.FileText(html.Class("size-4")),
		})
	}
	// Sales Activities → /activities (canViewSalesActivity, objek
	// crm:sales_activity). Berbeda dari "Activities" top-level (objek global
	// crm:activities): ini VIEW TERFILTER Sales (activity_context='sales'), maka
	// csm/support (tanpa crm:sales_activity) tetap tak masuk sini — BL-39.
	if canSalesActivity {
		children = append(children, ui.NavItem{
			Label: "Sales Activities", Href: wsPath(slug, "/activities"),
			Icon: lucide.Activity(html.Class("size-4")),
		})
	}
	return &ui.NavItem{
		Label: "Sales", Icon: lucide.TrendingUp(html.Class("size-4")), Children: children,
	}
}
