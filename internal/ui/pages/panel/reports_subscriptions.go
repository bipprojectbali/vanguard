package panel

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// reports_subscriptions.go — Subscription Report 8.4 (Modul 8, BL-47): view
// MURNI-DATA; handler (internal/handler/reports_subscriptions*.go) yang
// menghitung & memformat (Rp/persen/porsi + masking F4). Struktur data 5 panel
// di sini; fungsi render tiap panel di reports_subscriptions_panels.go (pola
// sama reports_support/reports_cs). Kartu KPI meniru dashboardKPICard (package
// sama); tiap <table> dibungkus ui.TableScroll lewat csTableScroll (WAJIB,
// gotcha overflow). Daftar baris renewal/churn per-desa TIDAK lagi di sini —
// itu drill-down di modul Subscriptions (/subscriptions/renewals · /churn);
// laporan ini agregasi murni ("Report bukan objek data").
//
// DILEWATKAN (keputusan sadar BL-47, "data belum ada DILEWATKAN dulu"): delta
// "vs bulan lalu" di kartu MRR & tren MRR bulanan historis (Apr–Agu) — tak ada
// snapshot MRR per bulan lampau (subscriptions.mrr = nilai SEKARANG). Filter
// interaktif Periode+Paket ditunda ke BL lanjutan (pola BL-49/50/51).

// ── Struktur data (semua string SUDAH diformat & di-mask F4 di handler) ──

// SubMRRComponentRow = satu komponen pergerakan MRR bulan ini (Panel 1): MRR
// baru / Ekspansi / Kontraksi / Churn. Value = Rp (di-mask F4). Porsi = bagian
// terhadap total magnitudo pergerakan.
type SubMRRComponentRow struct {
	Component string
	Value     string
	Count     int64
	Porsi     string
}

// SubRenewalMonthRow = satu baris bulanan Panel 2 (Renewal). Rate = diperpanjang/
// jatuh-tempo bulan itu (dihitung handler).
type SubRenewalMonthRow struct {
	Period  string
	Due     int64
	Renewed int64
	Rate    string
}

// SubChurnReasonRow = satu alasan churn Panel 3. Reason = label Indonesia
// (dipetakan dari picklist Inggris). LostValue = Rp (di-mask F4). Porsi =
// bagian desa terhadap total desa churn.
type SubChurnReasonRow struct {
	Reason    string
	Count     int64
	LostValue string
	Porsi     string
}

// SubPlanRevenueRow = satu paket Panel 4 (Revenue by Plan). MRR & AvgPer = Rp
// (di-mask F4). BarPct relatif paket MRR terbesar.
type SubPlanRevenueRow struct {
	Plan   string
	Count  int64
	MRR    string
	AvgPer string
	BarPct int
}

// SubAgingRow = satu kelompok umur Panel 5 (Subscription Aging). MRR = Rp
// (di-mask F4). Note = label interpretatif tetap per bucket (const handler).
type SubAgingRow struct {
	Bucket      string
	Villages    int64
	MRR         string
	AvgHealth   string
	RenewalRate string
	ChurnRate   string
	Note        string
}

// ReportsSubscriptionsView = data halaman /reports/subscriptions (5 panel
// agregasi + 4 KPI). Semua nilai SUDAH string terformat/ter-mask.
type ReportsSubscriptionsView struct {
	Base string

	// Filter interaktif Periode + Paket (BL-52).
	Filter SubscriptionReportFilterView

	// KPI (4 kartu)
	MRR         string
	ARR         string
	RenewalRate string
	ChurnRate   string

	// Panel 1 — MRR/ARR Report
	MRRComponents []SubMRRComponentRow

	// Panel 2 — Renewal Report
	RenewalRateCard string
	RenewedValue    string
	Due30           int64
	RenewalMonths   []SubRenewalMonthRow

	// Panel 3 — Churn Report
	ChurnVillages int64
	LostValue     string
	AvgAge        string
	ChurnReasons  []SubChurnReasonRow

	// Panel 4 — Revenue by Plan
	RevenueByPlan []SubPlanRevenueRow

	// Panel 5 — Subscription Aging (selebar penuh)
	AgingRows []SubAgingRow
}

// SubscriptionReportFilterView = sub-view filter interaktif Periode + Paket
// (BL-52). Reuse SalesFilterOption (bentuk opsi identik). Dropdown Paket selalu
// dirender (plan tak owner-spesifik; diturunkan dari DATA dalam cakupan pemakai).
// QueryString ditempel ke tautan Export CSV agar CSV tersaring identik HTML.
// CustomStart/End = echo input tanggal Kustom.
type SubscriptionReportFilterView struct {
	PeriodValue string
	Periods     []SalesFilterOption
	PlanValue   string
	Plans       []SalesFilterOption
	CustomStart string
	CustomEnd   string
	QueryString string
}

// ReportsSubscriptionsBody — header + 4 KPI + 4 panel (grid 2 kolom) + panel
// Aging selebar penuh. Grid mobile-first: 1 kolom di mobile, 2 di lg.
func ReportsSubscriptionsBody(v ReportsSubscriptionsView) g.Node {
	return h.Div(h.Class("grid gap-4 min-w-0"),
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text("Subscription Report")),
			h.P(h.Class("text-base-content/70"), g.Text("MRR/ARR, renewal, churn, pendapatan per paket, & umur langganan.")),
		),
		reportsSubscriptionsFilterForm(v),
		reportsSubscriptionsKPICards(v),
		h.Div(h.Class("grid grid-cols-1 lg:grid-cols-2 gap-4 min-w-0"),
			reportsSubMRRPanel(v),
			reportsSubRenewalPanel(v),
			reportsSubChurnPanel(v),
			reportsSubRevenuePanel(v),
		),
		reportsSubAgingPanel(v),
	)
}

// reportsSubscriptionsFilterForm = baris filter Periode + Paket (BL-52). Form
// GET native (bookmarkable, lolos CSP gotcha #16) → submit re-render seluruh
// halaman tersaring. Dropdown Paket selalu dirender (plan tak owner-spesifik).
// Input tanggal Kustom selalu tampil (tanpa JS toggle, CSP-safe) tapi hanya
// berdampak saat Periode=Kustom (handler). Catatan snapshot memperjelas Periode
// tak menyentuh MRR/ARR berjalan, Revenue by Plan, & Aging. Mobile-first:
// flex-wrap (nol overflow 375px), tiap kontrol text-base (≥16px → iOS tak
// auto-zoom) + min-h-11 (tap ≥44px).
func reportsSubscriptionsFilterForm(v ReportsSubscriptionsView) g.Node {
	f := v.Filter
	return h.Div(h.Class("grid gap-1 min-w-0"),
		h.Form(
			h.Method("get"), h.Action(v.Base+"/reports/subscriptions"),
			h.Class("flex flex-wrap items-end gap-3 min-w-0"),
			salesFilterSelect("period", "Periode", f.Periods),
			salesFilterSelect("plan", "Paket", f.Plans),
			salesFilterDate("start", "Dari (Kustom)", f.CustomStart),
			salesFilterDate("end", "Sampai (Kustom)", f.CustomEnd),
			h.Div(h.Class("flex flex-wrap items-end gap-2"),
				h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text("Terapkan")),
				g.If(f.QueryString != "", h.A(
					h.Href(v.Base+"/reports/subscriptions"),
					h.Class("btn btn-ghost min-h-11"), g.Text("Reset"),
				)),
			),
		),
		h.P(h.Class("text-xs text-base-content/60"),
			g.Text("Periode menyaring pergerakan MRR, renewal, & churn; MRR/ARR berjalan, Revenue by Plan, & Aging menampilkan kondisi terkini. Paket menyaring semua panel.")),
	)
}

func reportsSubscriptionsKPICards(v ReportsSubscriptionsView) g.Node {
	return h.Div(h.Class("grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3"),
		dashboardKPICard("MRR", v.MRR, "text-primary"),
		dashboardKPICard("ARR", v.ARR, "text-secondary"),
		dashboardKPICard("Renewal Rate", v.RenewalRate, "text-success"),
		dashboardKPICard("Churn Rate", v.ChurnRate, "text-error"),
	)
}

// reportSubPanelCard = pembungkus kartu panel seragam Subscription: judul +
// tautan Export CSV per-panel (panelKey → ?panel=…) + isi. Pola sama
// reportSupportPanelCard/reportCSPanelCard.
func reportSubPanelCard(base, filterQS, title, panelKey string, body ...g.Node) g.Node {
	href := base + "/reports/subscriptions/export?panel=" + panelKey
	if filterQS != "" {
		href += "&" + filterQS
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(h.Class("card-body min-w-0"),
			h.Div(h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
				h.H2(h.Class("font-semibold"), g.Text(title)),
				h.A(
					h.Href(href),
					h.Class("btn btn-outline btn-sm min-h-11"),
					g.Text("Export CSV"),
				),
			),
			g.Group(body),
		),
	)
}
