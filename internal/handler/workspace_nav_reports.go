package handler

import (
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"maragu.dev/gomponents/html"
)

// workspace_nav_reports.go — grup nav Reports (wireframe 8, board 8.1-8.5),
// dipisah dari workspace_nav.go (rule #8 file-health). Meniru pola
// workspace_nav_subscriptions.go.

// workspaceReportsGroup merakit grup Reports (wireframe 8). SELALU tampil (peta
// jalan). Sales Reports (8.1) & Subscription Reports (8.4) berbackend sejak
// M8-1, enabled mengikuti canReports (objek crm:reports read, SATU gerbang utk
// kedua preset). Customer Success Reports (8.2), Support Reports (8.3), Custom
// Reports (8.5) tetap disabled — 8.2/8.3 bergantung slice Modul 6 lain, 8.5
// (Report Builder) ditunda eksplisit v1.1 (skema.md §10).
func workspaceReportsGroup(slug string, canReports bool) ui.NavItem {
	sales := ui.NavItem{Label: "Sales Reports", Icon: lucide.TrendingUp(html.Class("size-4"))}
	if canReports {
		sales.Href = wsPath(slug, "/reports/sales")
	} else {
		sales.Disabled = true
	}
	subs := ui.NavItem{Label: "Subscription Reports", Icon: lucide.RefreshCw(html.Class("size-4"))}
	if canReports {
		subs.Href = wsPath(slug, "/reports/subscriptions")
	} else {
		subs.Disabled = true
	}
	children := []ui.NavItem{
		sales,
		{Label: "Customer Success Reports", Icon: lucide.HeartPulse(html.Class("size-4")), Disabled: true},
		{Label: "Support Reports", Icon: lucide.Headset(html.Class("size-4")), Disabled: true},
		subs,
		{Label: "Custom Reports", Icon: lucide.Settings2(html.Class("size-4")), Disabled: true},
	}
	return ui.NavItem{
		Label: "Reports", Icon: lucide.ChartColumn(html.Class("size-4")), Children: children,
	}
}
