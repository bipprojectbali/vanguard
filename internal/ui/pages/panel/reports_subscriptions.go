package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// reports_subscriptions.go — Subscription Report (Modul 8 M8-1, wireframe
// 8.4): view MURNI-DATA, dua tab bookmarkable (Renewal Forecast/Churn, gotcha
// #16 — LINK <a>, bukan Datastar). Tabel REUSE LANGSUNG renewalsTable/
// churnTable (subscriptions_renewals.go/subscriptions_churn.go, package
// sama) — kolom & ui.TableScroll tak ditulis ulang, hanya Base+Items dipakai.

// reportSectionTabs = dua opsi tab TETAP (bukan dioper handler seperti
// Window/Type — hanya dua section). Key SAMA dgn konstanta handler
// reportSectionRenewal/reportSectionChurn (reports_subscriptions.go).
var reportSectionTabs = []struct{ Key, Label string }{
	{"renewal", "Renewal Forecast"},
	{"churn", "Churn"},
}

// ReportsSubscriptionsView = data halaman /reports/subscriptions. Section
// aktif menentukan slice mana yang dirender — hanya salah satu Items terisi
// per request (handler hanya mengambil satu sumber per section).
type ReportsSubscriptionsView struct {
	Base         string
	Section      string
	RenewalItems []RenewalRow
	ChurnItems   []ChurnRow
	NextCursor   string
	After        string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail        string // BL-7: jejak cursor halaman sebelumnya (?trail=)
}

// ReportsSubscriptionsBody merender header + tab section + tabel aktif +
// pager + link Export CSV (ikut section aktif di query string).
func ReportsSubscriptionsBody(v ReportsSubscriptionsView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Subscription Report")),
				h.P(h.Class("text-base-content/70"), g.Text("Renewal jatuh tempo & langganan berhenti.")),
			),
			h.A(h.Href(v.Base+"/reports/subscriptions/export?section="+v.Section),
				h.Class("btn btn-outline min-h-11"), g.Text("Export CSV")),
		),
		reportSectionTabsView(v),
	}
	if v.Section == "churn" {
		if len(v.ChurnItems) == 0 {
			body = append(body, emptyChurn())
		} else {
			body = append(body, churnTable(ChurnView{Base: v.Base, Items: v.ChurnItems}))
		}
	} else {
		if len(v.RenewalItems) == 0 {
			body = append(body, emptyRenewals())
		} else {
			body = append(body, renewalsTable(RenewalsView{Base: v.Base, Items: v.RenewalItems}))
		}
	}
	body = append(body, reportsSubscriptionsPager(v))
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// reportSectionTabsView = baris tab LINK section. Navigasi bookmarkable
// (gotcha #16); flex-wrap agar tak mendorong lebar di mobile.
func reportSectionTabsView(v ReportsSubscriptionsView) g.Node {
	tabs := make([]g.Node, 0, len(reportSectionTabs))
	for _, t := range reportSectionTabs {
		cls := "tab min-h-11"
		if v.Section == t.Key {
			cls += " tab-active font-medium"
		}
		tabs = append(tabs, h.A(
			h.Href(v.Base+"/reports/subscriptions?section="+t.Key),
			h.Class(cls), g.Text(t.Label)))
	}
	return h.Div(h.Role("tablist"), h.Class("tabs tabs-bordered flex-wrap"), g.Group(tabs))
}

func reportsSubscriptionsPager(v ReportsSubscriptionsView) g.Node {
	base := v.Base + "/reports/subscriptions?section=" + v.Section
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}
