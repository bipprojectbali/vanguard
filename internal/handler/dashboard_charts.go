package handler

import "go_starter/internal/db"

// dashboard_charts.go — option ECharts Beranda. BL-98: setelah section domain
// dirampingkan (chart per-domain dibuang, hanya 2 KPI + tautan Report per
// domain), SATU-SATUNYA chart Beranda adalah donut "distribusi health" GLOBAL.
// Dibangun di Go (map generik, pola sama dgn activity/charts.go) agar data
// menyatu dgn render server & bisa di-test; di-marshal jadi JSON oleh
// dashboardData lalu ditanam <script type="application/json"> (CSP-safe) —
// charts.js yang merender.

// healthChartOption membangun option donut distribusi health (sehat/berisiko/
// kritis) dari CountHealthScoreKPIs — sengaja TAK termasuk "belum di-score"
// (kolom Scored dipakai terpisah sbg keterangan, bukan irisan pie) agar
// proporsi tiga status tetap terbaca jelas.
func healthChartOption(k db.CountHealthScoreKPIsRow) map[string]any {
	return map[string]any{
		"tooltip": map[string]any{"trigger": "item"},
		"legend":  map[string]any{"bottom": 0},
		"series": []any{
			map[string]any{
				"name": "Health", "type": "pie", "radius": []string{"45%", "70%"},
				"data": []any{
					map[string]any{"name": "Sehat", "value": k.Healthy},
					map[string]any{"name": "Berisiko", "value": k.AtRisk},
					map[string]any{"name": "Kritis", "value": k.Critical},
				},
			},
		},
	}
}

// healthMovementChartOption — BL-138: pie "Arah Pergerakan" (Membaik/Stabil/
// Menurun) di halaman Health Score, menggantikan 3 bar horizontal. Basis persen
// = trendBase (jumlah tiga nilai ini), dihitung ECharts sendiri via {d}% —
// BUKAN k.Total (desa tanpa tren, snapshot pertama BL-25, tak ikut basis).
// Warna eksplisit (bukan token daisyUI — kanvas ECharts butuh literal, gotcha
// #11 tak berlaku di sini) mencerminkan intent bar lama: hijau/abu/merah.
func healthMovementChartOption(k db.CountHealthScoreKPIsRow) map[string]any {
	return map[string]any{
		"tooltip": map[string]any{"trigger": "item"},
		"legend":  map[string]any{"bottom": 0},
		"series": []any{
			map[string]any{
				"name": "Arah Pergerakan", "type": "pie", "radius": []string{"45%", "70%"},
				"label": map[string]any{"formatter": "{b}: {c} · {d}%"},
				"data": []any{
					map[string]any{"name": "Membaik", "value": k.TrendImproving,
						"itemStyle": map[string]any{"color": "#16a34a"}},
					map[string]any{"name": "Stabil", "value": k.TrendStable,
						"itemStyle": map[string]any{"color": "#9ca3af"}},
					map[string]any{"name": "Menurun", "value": k.TrendDeclining,
						"itemStyle": map[string]any{"color": "#dc2626"}},
				},
			},
		},
	}
}

// pieOption — helper GENERIK donut (BL-140, membalik BL-98 utk chart Beranda
// per-domain). labels/values SEJAJAR indeks (panjang sama — kewajiban
// caller); dipakai lintas-domain (Sales/Subscription/CS) agar tak ada
// fungsi *ChartOption satu-satu per grafik seperti era pre-BL-98.
func pieOption(name string, labels []string, values []int64) map[string]any {
	data := make([]any, len(labels))
	for i, l := range labels {
		data[i] = map[string]any{"name": l, "value": values[i]}
	}
	return map[string]any{
		"tooltip": map[string]any{"trigger": "item"},
		"legend":  map[string]any{"bottom": 0},
		"series": []any{
			map[string]any{
				"name": name, "type": "pie", "radius": []string{"45%", "70%"},
				"data": data,
			},
		},
	}
}

// barOption — helper GENERIK bar vertikal (BL-140). categories/values sejajar
// indeks. grid left/right/bottom/containLabel (pola era pre-BL-98) menjaga
// label sumbu tak terpotong di kanvas sempit (mobile-first).
func barOption(categories []string, values []int64) map[string]any {
	return map[string]any{
		"tooltip": map[string]any{"trigger": "axis"},
		"grid":    map[string]any{"left": "3%", "right": "4%", "bottom": "3%", "containLabel": true},
		"xAxis":   map[string]any{"type": "category", "data": categories},
		"yAxis":   map[string]any{"type": "value", "minInterval": 1},
		"series": []any{
			map[string]any{"type": "bar", "data": values},
		},
	}
}
