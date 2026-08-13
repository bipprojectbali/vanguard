package handler

import (
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"maragu.dev/gomponents/html"
)

// workspace_nav_sales.go — grup nav Sales (wireframe 4), dipisah dari
// workspace_nav.go (rule #8 file-health) begitu jumlah grup nested bertambah.

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
