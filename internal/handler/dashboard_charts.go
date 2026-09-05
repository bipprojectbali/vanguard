package handler

import "go_starter/internal/db"

// dashboard_charts.go — dua option ECharts Beranda: bar "pipeline per-stage" &
// donut "distribusi health". Dibangun di Go (map generik, pola sama dgn
// activity/charts.go) agar data menyatu dgn render server & bisa di-test;
// di-marshal jadi JSON oleh dashboardData lalu ditanam <script
// type="application/json"> (CSP-safe) — charts.js yang merender.

// pipelineChartOption membangun option bar jumlah deal per-stage TERBUKA.
// Urutan stage sudah benar dari query (ORDER BY CASE) — tak perlu re-sort.
func pipelineChartOption(rows []db.DashboardPipelineByStageRow) map[string]any {
	stages := make([]string, len(rows))
	counts := make([]int64, len(rows))
	for i, r := range rows {
		stages[i] = r.Stage
		counts[i] = r.DealCount
	}
	return map[string]any{
		"tooltip": map[string]any{"trigger": "axis"},
		"grid":    map[string]any{"left": "3%", "right": "4%", "bottom": "3%", "containLabel": true},
		"xAxis":   map[string]any{"type": "category", "data": stages},
		"yAxis":   map[string]any{"type": "value", "minInterval": 1},
		"series": []any{
			map[string]any{"name": "Deal", "type": "bar", "data": counts},
		},
	}
}

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

// leadsBySourceChartOption membangun option bar jumlah lead per sumber (BL-59a,
// section Sales). Urutan sudah dari query (count DESC) — tak perlu re-sort.
func leadsBySourceChartOption(rows []db.DashboardLeadsBySourceRow) map[string]any {
	sources := make([]string, len(rows))
	counts := make([]int64, len(rows))
	for i, r := range rows {
		sources[i] = r.Source
		counts[i] = r.LeadCount
	}
	return map[string]any{
		"tooltip": map[string]any{"trigger": "axis"},
		"grid":    map[string]any{"left": "3%", "right": "4%", "bottom": "3%", "containLabel": true},
		"xAxis":   map[string]any{"type": "category", "data": sources},
		"yAxis":   map[string]any{"type": "value", "minInterval": 1},
		"series": []any{
			map[string]any{"name": "Lead", "type": "bar", "data": counts},
		},
	}
}

// mrrMovementChartOption — bar MRR baru vs churn (BL-59b, section Langganan).
// Dua batang nilai Rp dari komponen ReportSubMRR (jendela "bulan ini"). Hanya
// dirakit saat pemakai berhak lihat nilai (canSeeARR) — chart tak memasking.
func mrrMovementChartOption(m db.ReportSubMRRRow) map[string]any {
	return map[string]any{
		"tooltip": map[string]any{"trigger": "axis"},
		"grid":    map[string]any{"left": "3%", "right": "4%", "bottom": "3%", "containLabel": true},
		"xAxis":   map[string]any{"type": "category", "data": []string{"MRR Baru", "Churn"}},
		"yAxis":   map[string]any{"type": "value"},
		"series": []any{
			map[string]any{"name": "MRR", "type": "bar", "data": []float64{numericFloat(m.NewMrr), numericFloat(m.ChurnMrr)}},
		},
	}
}

// revenueByPlanChartOption — bar MRR aktif per paket (BL-59b). Nama paket dari
// data tenant (bukan hardcode); urutan MRR terbesar sudah dari query.
func revenueByPlanChartOption(rows []db.ReportRevenueByPlanRow) map[string]any {
	names := make([]string, len(rows))
	values := make([]float64, len(rows))
	for i, r := range rows {
		names[i] = r.PlanName
		values[i] = numericFloat(r.Mrr)
	}
	return map[string]any{
		"tooltip": map[string]any{"trigger": "axis"},
		"grid":    map[string]any{"left": "3%", "right": "4%", "bottom": "3%", "containLabel": true},
		"xAxis":   map[string]any{"type": "category", "data": names},
		"yAxis":   map[string]any{"type": "value"},
		"series": []any{
			map[string]any{"name": "MRR", "type": "bar", "data": values},
		},
	}
}
