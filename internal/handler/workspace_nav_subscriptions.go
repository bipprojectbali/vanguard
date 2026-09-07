package handler

import (
	"go_starter/internal/ui"

	lucide "github.com/eduardolat/gomponents-lucide"
	"maragu.dev/gomponents/html"
)

// workspace_nav_subscriptions.go — grup nav Subscriptions (wireframe 5),
// dipisah dari workspace_nav.go (rule #8 file-health) begitu jumlah grup
// nested bertambah.

// workspaceSubscriptionsGroup merakit grup Subscriptions (wireframe 5). Seperti
// Sales, SELALU tampil (peta jalan). Subscription Lists → /subscriptions &
// Plans & Pricing → /plans enabled per izin (sumber izin SAMA dengan
// canViewSubscriptions/canViewPlans — nol menu hantu). Renewals & Churn = dasbor
// read-only Menu 5.2 (M5-4), enabled mengikuti crm:subscriptions read.
//
// Urutan tampil: Subscription Lists → Renewals → Churn / Cancellations →
// Plans & Pricing (BL-91, permintaan user 7 Sep). Churn sengaja DI ATAS
// Plans & Pricing di sidebar — reorder ini murni sidebar, TAK menyentuh
// dokumentasi/urutan §4 (crmModules).
func workspaceSubscriptionsGroup(slug string, canPlans, canSubs bool) ui.NavItem {
	children := make([]ui.NavItem, 0, 4)
	// Subscription Lists → /subscriptions (canViewSubscriptions, objek
	// crm:subscriptions). Berbackend sejak M5-3b; disabled bila tak berhak, tetap
	// tampil agar posisi modul di peta jalan terlihat.
	active := ui.NavItem{Label: "Subscription Lists", Icon: lucide.RefreshCw(html.Class("size-4"))}
	if canSubs {
		active.Href = wsPath(slug, "/subscriptions")
	} else {
		active.Disabled = true
	}
	// Renewals → /subscriptions/renewals (dasbor read-only Menu 5.2). Izin SAMA
	// dengan Active Subscriptions (objek crm:subscriptions read); disabled bila tak
	// berhak, tetap tampil agar posisi modul di peta jalan terlihat.
	renewals := ui.NavItem{Label: "Renewals", Icon: lucide.CalendarClock(html.Class("size-4"))}
	if canSubs {
		renewals.Href = wsPath(slug, "/subscriptions/renewals")
	} else {
		renewals.Disabled = true
	}
	children = append(children, active, renewals)
	// Plans & Pricing → /plans (canViewPlans, objek crm:plans). Berbackend sejak
	// M5-3a; disabled bila tak berhak, tetap tampil agar posisi modul terlihat.
	plans := ui.NavItem{Label: "Plans & Pricing", Icon: lucide.Tag(html.Class("size-4"))}
	if canPlans {
		plans.Href = wsPath(slug, "/plans")
	} else {
		plans.Disabled = true
	}
	// Churn / Cancellations → /subscriptions/churn (dasbor read-only Menu 5.2/5.4).
	// Izin SAMA dengan Active Subscriptions (objek crm:subscriptions read); disabled
	// bila tak berhak, tetap tampil agar posisi modul di peta jalan terlihat.
	churn := ui.NavItem{Label: "Churn / Cancellations", Icon: lucide.UserMinus(html.Class("size-4"))}
	if canSubs {
		churn.Href = wsPath(slug, "/subscriptions/churn")
	} else {
		churn.Disabled = true
	}
	// BL-91: Churn / Cancellations SEBELUM Plans & Pricing (churn, plans).
	children = append(children, churn, plans)
	return ui.NavItem{
		Label: "Subscriptions", Icon: lucide.RefreshCw(html.Class("size-4")), Children: children,
	}
}
