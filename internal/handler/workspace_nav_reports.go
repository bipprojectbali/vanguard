package handler

import (
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"maragu.dev/gomponents/html"
)

// workspace_nav_reports.go — grup nav Reports (wireframe 8, board 8.1-8.5),
// dipisah dari workspace_nav.go (rule #8 file-health). Meniru pola
// workspace_nav_subscriptions.go.

// workspaceReportsGroup merakit grup Reports (wireframe 8). Sama pola dengan
// Settings: nil bila user tak berhak ke satu pun anak (grup disembunyikan
// total). Sales Reports (8.1), Customer Success Reports (8.2), Support Reports
// (8.3), Subscription Reports (8.4) berbackend, enabled per-domain (BL-169:
// dulu satu gerbang canReports/crm:reports utk semuanya; kini 4 objek Casbin
// terpisah — admin bisa memberi Sales akses Sales Report saja). Custom Reports
// (8.5, Report Builder) SENGAJA TIDAK muncul di sini sama sekali — ditunda
// eksplisit v1.1 (skema.md §10); item nav-nya dihapus (bukan disabled) karena
// belum ada halaman untuk dituju sampai dibangun.
func workspaceReportsGroup(slug string, canSalesReports, canCSReports, canSupportReports, canSubscriptionReports bool) *ui.NavItem {
	if !canSalesReports && !canCSReports && !canSupportReports && !canSubscriptionReports {
		return nil
	}
	children := make([]ui.NavItem, 0, 4)
	if canSalesReports {
		children = append(children, ui.NavItem{
			Label: "Sales Reports", Href: wsPath(slug, "/reports/sales"),
			Icon: lucide.TrendingUp(html.Class("size-4")),
		})
	}
	if canCSReports {
		children = append(children, ui.NavItem{
			Label: "Customer Success Reports", Href: wsPath(slug, "/reports/customer-success"),
			Icon: lucide.HeartPulse(html.Class("size-4")),
		})
	}
	if canSupportReports {
		children = append(children, ui.NavItem{
			Label: "Support Reports", Href: wsPath(slug, "/reports/support"),
			Icon: lucide.Headset(html.Class("size-4")),
		})
	}
	if canSubscriptionReports {
		children = append(children, ui.NavItem{
			Label: "Subscription Reports", Href: wsPath(slug, "/reports/subscriptions"),
			Icon: lucide.RefreshCw(html.Class("size-4")),
		})
	}
	return &ui.NavItem{
		Label: "Reports", Icon: lucide.ChartColumn(html.Class("size-4")), Children: children,
	}
}
