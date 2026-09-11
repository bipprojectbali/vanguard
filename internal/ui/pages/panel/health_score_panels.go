package panel

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// health_score_panels.go — BL-96: dua panel dasbor "bagian ATAS" halaman Health
// Score (Sebaran Kesehatan + Komposisi Skor & Arah Pergerakan). View murni-data:
// semua persen/lebar bar & label sudah dihitung di handler (health_score_view.go)
// — di sini hanya render. Warna WAJIB token semantik daisyUI (success/warning/
// error/primary), bukan hijau/kuning/merah absolut (CLAUDE.md §11).

// HealthBarView — satu baris bar horizontal (distribusi/komposisi/tren).
// Pct = lebar 0..100; Value = teks kanan (mis. "12 · 40%" atau "78").
// Color = kelas latar token daisyUI (mis. "bg-success").
type HealthBarView struct {
	Label string
	Value string
	Pct   int
	Color string
}

// HealthDashPanels — data dua panel atas. Empty (Scored=0) → placeholder.
type HealthDashPanels struct {
	Scored bool // ada minimal satu desa terskor
	// DistributionChart — BL-139: JSON option ECharts pie "Sebaran Kesehatan"
	// (Sehat/Berisiko/Kritis), pola dashboardChartCard.
	DistributionChart string
	Composition       []HealthBarView // Komposisi: Adopsi/Engagement/Support/Sentimen
	// MovementChart — BL-138: JSON option ECharts pie "Arah Pergerakan"
	// (Membaik/Stabil/Menurun). Kosong bila MovementEmpty.
	MovementChart string
	MovementEmpty bool // trendBase==0 (tak ada desa dgn tren) → placeholder, bukan pie kosong
}

// healthDashPanels merender dua kartu berdampingan (1 kolom di mobile).
func healthDashPanels(p HealthDashPanels) g.Node {
	return h.Div(h.Class("grid grid-cols-1 lg:grid-cols-2 gap-3"),
		healthPanelCard("Sebaran Kesehatan", p.Scored,
			healthChartBlock("chart-distribution", p.DistributionChart), nil),
		healthPanelCard("Komposisi Skor", p.Scored, healthBars(p.Composition),
			healthMovementBlock(p.MovementChart, p.MovementEmpty)),
	)
}

// healthPanelCard — kartu berisi judul + body (bar atau chart); extra dirender
// di bawah body (dipakai panel Komposisi untuk blok Arah Pergerakan).
// Placeholder bila !scored.
func healthPanelCard(title string, scored bool, body g.Node, extra g.Node) g.Node {
	content := []g.Node{
		h.H3(h.Class("text-sm font-semibold text-base-content/70 mb-3"), g.Text(title)),
	}
	if !scored {
		content = append(content, h.P(h.Class("text-sm text-base-content/50 py-4"),
			g.Text("Belum ada skor kesehatan untuk dihitung.")))
	} else {
		content = append(content, body)
		if extra != nil {
			content = append(content, extra)
		}
	}
	return h.Div(h.Class("card bg-base-100 shadow-sm min-w-0"),
		h.Div(h.Class("card-body p-4"), g.Group(content)),
	)
}

// healthChartBlock — kontainer chart ECharts + data JSON, pola SAMA dgn
// dashboardChartCard (panel/dashboard.go): <div id=chartID> + <script
// type="application/json" id=chartID+"-data"> (CSP-safe, charts.js
// auto-discover). Tinggi tetap agar mobile-first nol-overflow.
func healthChartBlock(chartID, chartJSON string) g.Node {
	return h.Div(h.Class("min-w-0"),
		h.Div(h.ID(chartID), h.Class("min-w-0"), h.Style("height:240px")),
		h.Script(h.Type("application/json"), h.ID(chartID+"-data"), g.Raw(chartJSON)),
	)
}

// healthBars — daftar bar horizontal berlabel.
func healthBars(bars []HealthBarView) g.Node {
	nodes := make([]g.Node, 0, len(bars))
	for _, b := range bars {
		nodes = append(nodes, h.Div(h.Class("space-y-1"),
			h.Div(h.Class("flex items-center justify-between text-sm"),
				h.Span(h.Class("text-base-content/80"), g.Text(b.Label)),
				h.Span(h.Class("font-medium tabular-nums"), g.Text(b.Value)),
			),
			h.Div(h.Class("h-2 w-full rounded-full bg-base-200 overflow-hidden"),
				h.Div(h.Class("h-2 rounded-full "+b.Color),
					h.Style("width:"+strconv.Itoa(b.Pct)+"%"),
				),
			),
		))
	}
	return h.Div(h.Class("space-y-3"), g.Group(nodes))
}

// healthMovementBlock — sub-blok "Arah Pergerakan" di dalam panel Komposisi
// (BL-138: pie, bukan bar). empty=true (trendBase==0, tak ada desa dgn tren)
// → placeholder teks, JANGAN render pie kosong.
func healthMovementBlock(chartJSON string, empty bool) g.Node {
	var body g.Node
	if empty {
		body = h.P(h.Class("text-sm text-base-content/50 py-2"),
			g.Text("Belum ada data tren."))
	} else {
		body = healthChartBlock("chart-movement", chartJSON)
	}
	return h.Div(h.Class("mt-4 pt-4 border-t border-base-200"),
		h.H4(h.Class("text-xs font-semibold uppercase text-base-content/50 mb-3"),
			g.Text("Arah Pergerakan")),
		body,
	)
}
