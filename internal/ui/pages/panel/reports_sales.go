package panel

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// reports_sales.go — Sales Report (Modul 8, wireframe 8.1, BL-43): view
// MURNI-DATA; handler (internal/handler/reports_sales*.go) yang menghitung,
// memformat (formatRupiah), & memasking ARR (maskARR). Struktur data 5 panel di
// sini; fungsi render tiap panel di reports_sales_panels.go (satu file di bawah
// ambang). Kartu KPI meniru dashboardKPICard (dashboard.go, package sama); tiap
// <table> dibungkus ui.TableScroll (WAJIB, gotcha overflow).

// ── Struktur data 5 panel (semua string Rp SUDAH diformat & di-mask di handler) ──

// PipelineRow = satu baris tabel Pipeline Report (panel 1). Count/Value/Weighted
// & Probability SUDAH diformat di handler; BarPct 0–100 (relatif nilai stage
// terbesar) untuk lebar bar CSP-safe.
type PipelineRow struct {
	Stage       string
	Count       int64
	Value       string
	Probability string
	Weighted    string
	BarPct      int
}

// ForecastRow = satu bucket bulan Sales Forecast (panel 2). Period "Sep 2026".
type ForecastRow struct {
	Period   string
	Forecast string
	BarPct   int
}

// WinLossReasonRow = satu alasan kalah (panel 3). Porsi "% dari total kalah".
type WinLossReasonRow struct {
	Reason string
	Count  int64
	Porsi  string
	BarPct int
}

// FunnelStep = satu tahap corong Lead Conversion (panel 4). Pct = porsi dari
// total lead masuk.
type FunnelStep struct {
	Label  string
	Count  int64
	Pct    string
	BarPct int
}

// SalesActivityRow = satu baris Sales Activity Report (panel 5). PerDeal = rata
// aktivitas per deal menang ("—" bila belum ada deal menang). Dinamai spesifik
// (bukan ActivityRow) — nama itu sudah dipakai linimasa aktivitas
// (sales_activities.go) di package yang sama.
type SalesActivityRow struct {
	Owner   string
	Call    int64
	Email   int64
	Meeting int64
	Total   int64
	Won     int64
	PerDeal string
}

// SalesFilterOption = satu opsi dropdown filter (Periode/Tim). Selected menandai
// pilihan aktif (handler yang memutuskan, view tak menghitung).
type SalesFilterOption struct {
	Value    string
	Label    string
	Selected bool
}

// SalesReportFilterView = sub-view filter interaktif Periode + Tim(owner)
// (BL-49). ShowOwner=false → dropdown Tim tak dirender (pemakai non-scope_all;
// dropdown menyesatkan karena F3 mengunci ke dirinya). QueryString ditempel ke
// tautan Export CSV agar CSV tersaring identik HTML. CustomStart/End = nilai
// input date (echo, YYYY-MM-DD) saat Periode = Kustom.
type SalesReportFilterView struct {
	PeriodValue string
	Periods     []SalesFilterOption
	ShowOwner   bool
	OwnerValue  string
	Owners      []SalesFilterOption
	CustomStart string
	CustomEnd   string
	QueryString string
}

// ReportsSalesView — data siap-render /reports/sales (5 panel + KPI ringkas).
type ReportsSalesView struct {
	Base string

	// Filter interaktif Periode + Tim (BL-49).
	Filter SalesReportFilterView

	// KPI ringkas (dipertahankan dari M8-1).
	OpenCount     int64
	PipelineValue string
	WinRate       string

	// Panel 1 — Pipeline Report.
	Pipeline []PipelineRow

	// Panel 2 — Sales Forecast.
	Forecast []ForecastRow

	// Panel 3 — Win/Loss Analysis.
	WinPct      string
	LossPct     string
	LossReasons []WinLossReasonRow

	// Panel 4 — Lead Conversion.
	Funnel        []FunnelStep
	Conversion    string
	AvgLeadToDeal string
	AvgDealToWon  string

	// Panel 5 — Sales Activity Report.
	Activity []SalesActivityRow
}

// ReportsSalesBody merender header + KPI ringkas + 5 panel. Mobile-first: KPI
// grid-cols-1 (dasar) → md:grid-cols-3; panel ditumpuk grid-cols-1 →
// lg:grid-cols-2 (dua kolom di layar lebar).
func ReportsSalesBody(v ReportsSalesView) g.Node {
	return h.Div(h.Class("grid gap-4 min-w-0"),
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text("Sales Report")),
			h.P(h.Class("text-base-content/70"), g.Text("Pipeline, forecast, win/loss, konversi lead, & aktivitas — seluruh stage.")),
		),
		reportsSalesFilterForm(v),
		reportsSalesKPICards(v),
		h.Div(h.Class("grid grid-cols-1 lg:grid-cols-2 gap-4 min-w-0"),
			reportsPipelinePanel(v),
			reportsForecastPanel(v),
			reportsWinLossPanel(v),
			reportsFunnelPanel(v),
		),
		reportsActivityPanel(v),
	)
}

func reportsSalesKPICards(v ReportsSalesView) g.Node {
	return h.Div(h.Class("grid grid-cols-1 md:grid-cols-3 gap-3"),
		dashboardKPICard("Pipeline Terbuka", strconv.FormatInt(v.OpenCount, 10), "text-base-content"),
		dashboardKPICard("Nilai Pipeline", v.PipelineValue, "text-primary"),
		dashboardKPICard("Win Rate", v.WinRate, "text-success"),
	)
}

// reportsSalesFilterForm = baris filter Periode + Tim(owner) (BL-49). Form GET
// native (bookmarkable, lolos CSP gotcha #16) → submit re-render seluruh
// halaman tersaring; membuang tak-perlu preserve cursor (report bukan keyset).
// Dropdown Tim hanya dirender saat ShowOwner (pemakai scope_all). Input tanggal
// Kustom selalu tampil (tanpa JS toggle, CSP-safe) tapi hanya berdampak saat
// Periode=Kustom (handler). Mobile-first: flex-wrap (nol overflow 375px), tiap
// kontrol text-base (≥16px → iOS tak auto-zoom) + min-h-11 (tap ≥44px).
func reportsSalesFilterForm(v ReportsSalesView) g.Node {
	f := v.Filter
	controls := []g.Node{salesFilterSelect("period", "Periode", f.Periods)}
	if f.ShowOwner {
		controls = append(controls, salesFilterSelect("owner", "Tim", f.Owners))
	}
	controls = append(controls,
		salesFilterDate("start", "Dari (Kustom)", f.CustomStart),
		salesFilterDate("end", "Sampai (Kustom)", f.CustomEnd),
		h.Div(h.Class("flex flex-wrap items-end gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text("Terapkan")),
			g.If(f.QueryString != "", h.A(
				h.Href(v.Base+"/reports/sales"),
				h.Class("btn btn-ghost min-h-11"), g.Text("Reset"),
			)),
		),
	)
	return h.Form(
		h.Method("get"), h.Action(v.Base+"/reports/sales"),
		h.Class("flex flex-wrap items-end gap-3 min-w-0"),
		g.Group(controls),
	)
}

// salesFilterSelect = satu dropdown filter berlabel. Nilai terpilih ditandai di
// opsi (Selected) — view tak menghitung, cuma merender.
func salesFilterSelect(name, label string, opts []SalesFilterOption) g.Node {
	nodes := make([]g.Node, 0, len(opts))
	for _, o := range opts {
		nodes = append(nodes, h.Option(
			h.Value(o.Value), g.If(o.Selected, h.Selected()), g.Text(o.Label),
		))
	}
	return h.Label(h.Class("form-control min-w-0"),
		h.Span(h.Class("label-text text-xs text-base-content/60 mb-1"), g.Text(label)),
		h.Select(
			h.Name(name),
			h.Class("select select-bordered select-sm text-base min-h-11 min-w-0"),
			g.Attr("aria-label", label),
			g.Group(nodes),
		),
	)
}

// salesFilterDate = input tanggal (Kustom). type=date native; value di-echo
// mentah agar pilihan bertahan lintas submit.
func salesFilterDate(name, label, value string) g.Node {
	return h.Label(h.Class("form-control min-w-0"),
		h.Span(h.Class("label-text text-xs text-base-content/60 mb-1"), g.Text(label)),
		h.Input(
			h.Type("date"), h.Name(name), h.Value(value),
			h.Class("input input-bordered input-sm text-base min-h-11 min-w-0"),
			g.Attr("aria-label", label),
		),
	)
}

// reportPanelCard = pembungkus kartu panel seragam: judul + tautan Export CSV
// per-panel (panelKey → ?panel=…, ditempel filterQS BL-49 agar CSV tersaring
// identik) + isi. min-w-0 agar card menyusut (bukan meluber) di flex/grid mobile.
func reportPanelCard(base, filterQS, title, panelKey string, body ...g.Node) g.Node {
	href := base + "/reports/sales/export?panel=" + panelKey
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

// reportBar = bar horizontal CSP-safe (lebar precomputed di handler → inline
// style, style-src 'unsafe-inline'; preseden success_plans_list.go). BUKAN
// script inline / ECharts (gotcha #1/#12 — bar sederhana tak perlu runtime).
func reportBar(pct int) g.Node {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	return h.Div(h.Class("bg-base-300 rounded-full h-1.5 w-full min-w-[48px]"),
		h.Div(h.Class("bg-primary h-1.5 rounded-full"), h.Style("width:"+strconv.Itoa(pct)+"%")),
	)
}

// reportEmpty = placeholder seragam saat panel tak punya data (dalam cakupan
// ownership pemakai).
func reportEmpty(msg string) g.Node {
	return h.P(h.Class("text-sm text-base-content/70"), g.Text(msg))
}
