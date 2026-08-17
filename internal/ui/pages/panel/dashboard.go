package panel

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// dashboard.go — Beranda ruang kerja (Modul 1, tasks.md M1-2): view MURNI-DATA,
// handler (internal/handler/dashboard.go) yang menghitung & memasking ARR (F4).
// Meniru pola kartu health_score_list.go (grid card) + chart activityChart
// dev/logs.go (JSON ditanam <script type=application/json>, CSP-safe).

// DashboardView — data siap-render Beranda. String KPI (ARRTotal/RenewalsDueARR)
// SUDAH diformat & di-mask di handler (bisa "Rp 1.000.000" atau "•••") — view
// tak pernah memutuskan tampil/sembunyi sendiri.
type DashboardView struct {
	ARRTotal       string
	PipelineChart  string // JSON option ECharts (bar per-stage), sudah di-marshal
	HealthChart    string // JSON option ECharts (donut health), sudah di-marshal
	HealthTotal    int64
	HealthScored   int64
	RenewalsDue    int64
	RenewalsDueARR string
}

// DashboardBody merender kartu KPI + dua chart. Mobile-first: KPI grid-cols-2
// (dasar) → md:grid-cols-4; chart grid-cols-1 (dasar, tumpuk) → md:grid-cols-2.
func DashboardBody(v DashboardView) g.Node {
	return h.Div(
		dashboardKPICards(v),
		dashboardCharts(v),
		// Runtime ECharts (vendored) + init — same-origin, CSP-safe (gotcha #12).
		h.Script(h.Src("/static/echarts.min.js")),
		h.Script(h.Src("/static/charts.js"), h.Defer()),
	)
}

func dashboardKPICards(v DashboardView) g.Node {
	scored := strconv.FormatInt(v.HealthScored, 10) + " / " + strconv.FormatInt(v.HealthTotal, 10)
	return h.Div(h.Class("grid grid-cols-2 md:grid-cols-4 gap-3 mb-4"),
		dashboardKPICard("ARR Total", v.ARRTotal, "text-primary"),
		dashboardKPICard("Renewal Jatuh Tempo (30 hari)", strconv.FormatInt(v.RenewalsDue, 10), "text-warning"),
		dashboardKPICard("ARR Berisiko Jatuh Tempo", v.RenewalsDueARR, "text-warning"),
		dashboardKPICard("Desa Dinilai Health", scored, "text-base-content"),
	)
}

func dashboardKPICard(label, value, valueClass string) g.Node {
	return h.Div(h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(h.Class("card-body p-4 min-w-0"),
			h.P(h.Class("text-sm text-base-content/60"), g.Text(label)),
			h.P(h.Class("text-2xl font-bold truncate "+valueClass), g.Text(value)),
		),
	)
}

func dashboardCharts(v DashboardView) g.Node {
	return h.Div(h.Class("grid grid-cols-1 md:grid-cols-2 gap-4"),
		dashboardChartCard("Pipeline per-Stage", "chart-pipeline", v.PipelineChart),
		dashboardChartCard("Distribusi Health", "chart-health", v.HealthChart),
	)
}

// dashboardChartCard — kontainer chart + data JSON, pola SAMA dgn activityChart
// (dev/logs.go): JSON di <script type="application/json"> tak dieksekusi
// browser (CSP-safe); g.Raw aman karena isinya json.Marshal, bukan input user.
func dashboardChartCard(title, chartID, chartJSON string) g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("font-semibold mb-2"), g.Text(title)),
			h.Div(h.ID(chartID), h.Style("height:280px")),
			h.Script(
				h.Type("application/json"),
				h.ID(chartID+"-data"),
				g.Raw(chartJSON),
			),
		),
	)
}
