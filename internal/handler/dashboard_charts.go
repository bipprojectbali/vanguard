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
