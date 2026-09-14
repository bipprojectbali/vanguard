package handler

import (
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// health_score_kpi.go — KPI header & dasbor (Sebaran/Komposisi/Arah) Customer
// Health Score (BL-96, BL-138/139), dipisah dari health_score_view.go untuk
// file health.

// healthKPIsToView memetakan hasil agregat DB → KPI header (angka + sub-teks)
// dan dua panel dasbor BL-96 (Sebaran + Komposisi/Arah). Semua persen & lebar
// bar dihitung DI SINI (view murni-data). Panel dikosongkan (placeholder) bila
// belum ada desa terskor (Scored=0) agar bar 0% dari COALESCE tak menyesatkan.
func (h *Handler) healthKPIsToView(k db.CountHealthScoreKPIsRow) (panel.HealthScoreKPIs, panel.HealthDashPanels) {
	kpi := panel.HealthScoreKPIs{
		Total:      k.Total,
		Healthy:    k.Healthy,
		AtRisk:     k.AtRisk,
		Critical:   k.Critical,
		HealthySub: strconv.Itoa(pctOf(k.Healthy, k.Total)) + "% dari binaan",
	}
	if k.Scored == 0 {
		kpi.AvgScoreSub = "belum ada skor"
	} else {
		kpi.AvgScoreSub = "rata skor " + strconv.Itoa(roundToInt(k.AvgScore))
	}

	panels := panel.HealthDashPanels{Scored: k.Scored > 0}
	if panels.Scored {
		// BL-139: pie "Sebaran Kesehatan" — reuse option donut Beranda apa
		// adanya (healthChartOption, dashboard_charts.go) agar satu sumber.
		panels.DistributionChart = h.marshalChart(healthChartOption(k))
		panels.Composition = []panel.HealthBarView{
			healthCompBar("Adopsi", k.AvgAdoption),
			healthCompBar("Engagement", k.AvgEngagement),
			healthCompBar("Support", k.AvgSupport),
			healthCompBar("Sentimen", k.AvgSentiment),
		}
		// BL-138: pie "Arah Pergerakan" — porsi relatif terhadap desa yang
		// PUNYA tren (bukan total); desa tanpa tren (snapshot pertama, BL-25)
		// tak ikut basis. trendBase==0 → jangan render pie kosong.
		trendBase := k.TrendImproving + k.TrendStable + k.TrendDeclining
		panels.MovementEmpty = trendBase == 0
		if !panels.MovementEmpty {
			panels.MovementChart = h.marshalChart(healthMovementChartOption(k))
		}
	}
	return kpi, panels
}

// healthCompBar — bar komposisi: lebar & nilai = rata komponen (skala 0–100),
// dijepit 0..100 agar lebar bar tak melampaui trek.
func healthCompBar(label string, avg float64) panel.HealthBarView {
	v := roundToInt(avg)
	if v > 100 {
		v = 100
	}
	return panel.HealthBarView{
		Label: label,
		Value: strconv.Itoa(v),
		Pct:   v,
		Color: "bg-primary",
	}
}

// pctOf = persen bulat count/total; total ≤ 0 → 0 (hindari bagi nol).
func pctOf(count, total int64) int {
	if total <= 0 {
		return 0
	}
	return int(float64(count)/float64(total)*100 + 0.5)
}

// roundToInt membulatkan float non-negatif ke int terdekat (avg skor ≥ 0).
func roundToInt(f float64) int {
	if f < 0 {
		return 0
	}
	return int(f + 0.5)
}

// healthTableSubtitle — subteks tabel BL-96. Default (sortCol=="") tetap
// created_at DESC (keputusan B, keyset BL-7 tak diubah) → "dari terbaru".
// BL-157i: saat user memilih sort kolom lain, klaim "dari terbaru" jadi salah
// → dibuang, sisakan "N desa binaan" saja (mirror pola subtitle Renewals).
func healthTableSubtitle(total int64, sortCol string) string {
	if sortCol != "" {
		return strconv.FormatInt(total, 10) + " desa binaan"
	}
	return "Diurutkan dari terbaru · " + strconv.FormatInt(total, 10) + " desa binaan"
}
