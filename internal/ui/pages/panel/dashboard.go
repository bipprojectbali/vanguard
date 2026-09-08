package panel

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// dashboard.go — Beranda ruang kerja (Modul 1, tasks.md M1-2 + BL-59 + BL-98):
// view MURNI-DATA, handler (internal/handler/dashboard.go) yang menghitung,
// memasking ARR (F4), & mengomposisi section per-domain per-izin. Meniru pola
// kartu health_score_list.go (grid card) + chart activityChart dev/logs.go (JSON
// ditanam <script type=application/json>, CSP-safe).
//
// BL-98: section domain DIRAMPINGKAN untuk role multi-domain (admin) — maksimum
// 2 KPI ringkas per-domain (tanpa chart domain; chart hanya donut health GLOBAL)
// + tautan "Lihat Laporan →" ke halaman Report domain. Baris KPI GLOBAL + donut
// health DIPERTAHANKAN utuh.

// DashboardView — data siap-render Beranda. String KPI (ARRTotal/RenewalsDueARR)
// SUDAH diformat & di-mask di handler (bisa "Rp 1.000.000" atau "•••") — view
// tak pernah memutuskan tampil/sembunyi sendiri. Domains = section per-domain
// (Sales/…) yang sudah difilter per-kapabilitas di handler (BL-59).
type DashboardView struct {
	ARRTotal       string
	HealthChart    string // JSON option ECharts (donut health), sudah di-marshal
	HealthTotal    int64
	HealthScored   int64
	RenewalsDue    int64
	RenewalsDueARR string
	Domains        []DashDomain
}

// DashDomain — satu section domain (mis. "Sales"). Hanya dirakit handler bila
// role punya ≥1 kapabilitas modulnya; heading dirender hanya bila ada isi.
// BL-98: ReportPath = tautan "Lihat Laporan →". Handler HANYA menyetelnya bila
// role ber-crm:reports (canViewReports) → view tak pernah menautkan halaman yang
// akan 403; "" = tak ada tautan.
type DashDomain struct {
	Title      string
	ReportPath string
	KPIs       []DashKPI
}

// DashKPI — kartu angka ringkas dalam section domain. Value sudah diformat di
// handler (bisa mask F4).
type DashKPI struct {
	Label      string
	Value      string
	ValueClass string
}

// DashboardBody merender baris KPI GLOBAL + distribusi health, lalu tiap section
// domain. Mobile-first: KPI grid-cols-2 (dasar) → md:grid-cols-4; chart global
// grid-cols-1 (dasar, tumpuk) → md:grid-cols-2.
func DashboardBody(v DashboardView) g.Node {
	domains := make([]g.Node, len(v.Domains))
	for i, d := range v.Domains {
		domains[i] = dashboardDomain(d)
	}
	return h.Div(
		dashboardKPICards(v),
		dashboardGlobalCharts(v),
		g.Group(domains),
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

// dashboardGlobalCharts — chart GLOBAL yang bertahan di atas section domain.
// BL-59: distribusi health tetap di sini. BL-98: ini SATU-SATUNYA chart Beranda
// (chart per-domain dibuang). Kolom dipilih dashPanelCols agar chart tunggal
// mengisi penuh-lebar (bukan setengah kosong).
func dashboardGlobalCharts(v DashboardView) g.Node {
	return h.Div(h.Class(dashPanelCols(1)),
		dashboardChartCard("Distribusi Health", "chart-health", v.HealthChart),
	)
}

// dashKPICols memilih kelas grid strip KPI agar baris terisi penuh — tak ada sel
// kosong menggantung saat jumlah kartu < kolom (mis. 3 KPI di grid 4-kolom).
// Kolom = jumlah kartu, dibatasi 4; 6 → 3 (2 baris rapi). Mobile tetap 2 kolom
// (1 kartu → 1). Kelas WAJIB literal penuh agar tak ter-tree-shake Tailwind
// (gotcha #4) — jangan rakit string "md:grid-cols-"+n.
func dashKPICols(n int) string {
	switch n {
	case 0, 1:
		return "grid grid-cols-1 gap-3 mb-4"
	case 2:
		return "grid grid-cols-2 gap-3 mb-4"
	case 3, 6:
		return "grid grid-cols-2 md:grid-cols-3 gap-3 mb-4"
	default: // 4,5,7+ → 4 kolom (baris penuh utk kelipatan 4)
		return "grid grid-cols-2 md:grid-cols-4 gap-3 mb-4"
	}
}

// dashPanelCols — 1 panel → penuh-lebar (tak setengah kosong); ≥2 → 2 kolom
// desktop. Kelas literal penuh (gotcha #4). BL-98: kini hanya dipakai donut
// health global (n=1).
func dashPanelCols(n int) string {
	if n <= 1 {
		return "grid grid-cols-1 gap-4"
	}
	return "grid grid-cols-1 md:grid-cols-2 gap-4"
}

// dashboardDomain — heading section (judul + tautan "Lihat Laporan →" opsional) +
// strip KPI. Dipanggil hanya untuk domain yang punya isi (handler menyaring),
// jadi heading tak pernah berdiri kosong. BL-98: baris judul flex-wrap agar
// judul & tautan sebaris di desktop, turun rapi di mobile 375px.
func dashboardDomain(d DashDomain) g.Node {
	var kpiStrip g.Node
	if len(d.KPIs) > 0 {
		cards := make([]g.Node, len(d.KPIs))
		for i, k := range d.KPIs {
			cards[i] = dashboardKPICard(k.Label, k.Value, k.ValueClass)
		}
		kpiStrip = h.Div(h.Class(dashKPICols(len(d.KPIs))), g.Group(cards))
	}
	return h.Section(h.Class("mt-8"),
		h.Div(h.Class("flex flex-wrap items-center justify-between gap-2 mb-3"),
			h.H2(h.Class("text-lg font-semibold"), g.Text(d.Title)),
			g.If(d.ReportPath != "", dashboardReportLink(d.ReportPath)),
		),
		g.If(kpiStrip != nil, kpiStrip),
	)
}

// dashboardReportLink — tautan "Lihat Laporan →" ke halaman Report domain.
// Native <a> (navigasi biasa, lolos CSP gotcha #16, bukan Datastar). Tap target
// ≥44px (min-h-11). Dirender hanya bila handler menyetel ReportPath (role
// ber-crm:reports) → tak pernah menautkan halaman yang akan 403.
func dashboardReportLink(path string) g.Node {
	return h.A(
		h.Href(path),
		h.Class("inline-flex items-center min-h-11 text-sm font-medium text-primary hover:underline"),
		g.Text("Lihat Laporan →"),
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
