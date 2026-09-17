package handler

import (
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"maragu.dev/gomponents/html"
)

// workspace_nav_subscriptions.go — grup nav Subscriptions (wireframe 5),
// dipisah dari workspace_nav.go (rule #8 file-health) begitu jumlah grup
// nested bertambah.

// workspaceSubscriptionsGroup merakit grup Subscriptions (wireframe 5). Sama
// pola dengan Settings: nil bila user tak berhak ke SATU PUN anak (grup
// disembunyikan total). Subscription Lists → /subscriptions & Plans & Pricing
// → /plans enabled per izin (sumber izin SAMA dengan canViewSubscriptions/
// canViewPlans — nol menu hantu). Renewals & Churn = dasbor read-only Menu 5.2
// (M5-4), enabled mengikuti crm:subscriptions read.
//
// Urutan tampil: Subscription Lists → Renewals → Churn / Cancellations →
// Plans & Pricing (BL-91, permintaan user 7 Sep). Churn sengaja DI ATAS
// Plans & Pricing di sidebar — reorder ini murni sidebar, TAK menyentuh
// dokumentasi/urutan §4 (crmModules).
func workspaceSubscriptionsGroup(slug string, canPlans, canSubs bool) *ui.NavItem {
	if !canPlans && !canSubs {
		return nil
	}
	children := make([]ui.NavItem, 0, 4)
	// Subscription Lists → /subscriptions (canViewSubscriptions, objek
	// crm:subscriptions). Berbackend sejak M5-3b; tanpa izin → tak ditampilkan.
	if canSubs {
		children = append(children, ui.NavItem{
			Label: "Subscription Lists", Href: wsPath(slug, "/subscriptions"),
			Icon: lucide.RefreshCw(html.Class("size-4")),
		})
	}
	// Renewals → /subscriptions/renewals (dasbor read-only Menu 5.2). Izin SAMA
	// dengan Active Subscriptions (objek crm:subscriptions read); tanpa izin →
	// tak ditampilkan.
	if canSubs {
		children = append(children, ui.NavItem{
			Label: "Renewals", Href: wsPath(slug, "/subscriptions/renewals"),
			Icon: lucide.CalendarClock(html.Class("size-4")),
		})
	}
	// BL-91: Churn / Cancellations SEBELUM Plans & Pricing (churn, plans). Izin
	// SAMA dengan Active Subscriptions (objek crm:subscriptions read); tanpa izin
	// → tak ditampilkan.
	if canSubs {
		children = append(children, ui.NavItem{
			Label: "Churn / Cancellations", Href: wsPath(slug, "/subscriptions/churn"),
			Icon: lucide.UserMinus(html.Class("size-4")),
		})
	}
	// Plans & Pricing → /plans (canViewPlans, objek crm:plans). Berbackend sejak
	// M5-3a; tanpa izin → tak ditampilkan.
	if canPlans {
		children = append(children, ui.NavItem{
			Label: "Plans & Pricing", Href: wsPath(slug, "/plans"),
			Icon: lucide.Tag(html.Class("size-4")),
		})
	}
	return &ui.NavItem{
		Label: "Subscriptions", Icon: lucide.RefreshCw(html.Class("size-4")), Children: children,
	}
}
