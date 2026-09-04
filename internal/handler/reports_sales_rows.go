package handler

import (
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// reports_sales_rows.go — builder tiap panel Sales Report 8.1 (BL-43), dipisah
// dari reports_sales_panels.go (ukuran file). Mengubah baris agregat SQL menjadi
// sub-view siap-render (panel.*Row); masking F4 & persen/bar tetap di sini.
// Perilaku identik; hanya organisasi file yang berubah.

// buildPipelinePanel — panel 1: baris per-stage + bar relatif nilai stage
// terbesar. Value & Weighted (keduanya Rp) di-mask.
func buildPipelinePanel(stages []db.ReportPipelineByStageRow, br string) []panel.PipelineRow {
	var max float64
	for _, s := range stages {
		if v := numericFloat(s.StageValue); v > max {
			max = v
		}
	}
	rows := make([]panel.PipelineRow, 0, len(stages))
	for _, s := range stages {
		rows = append(rows, panel.PipelineRow{
			Stage:       s.Stage,
			Count:       s.DealCount,
			Value:       maskARR(formatRupiah(s.StageValue), br),
			Probability: strconv.FormatInt(s.AvgProbability, 10) + "%",
			Weighted:    maskARR(formatRupiah(s.WeightedValue), br),
			BarPct:      barPct(numericFloat(s.StageValue), max),
		})
	}
	return rows
}

// buildForecastPanel — panel 2: bucket bulan + bar relatif bulan terbesar.
// Forecast (Rp tertimbang) di-mask.
func buildForecastPanel(buckets []db.ReportSalesForecastRow, br string) []panel.ForecastRow {
	var max float64
	for _, b := range buckets {
		if v := numericFloat(b.WeightedValue); v > max {
			max = v
		}
	}
	rows := make([]panel.ForecastRow, 0, len(buckets))
	for _, b := range buckets {
		rows = append(rows, panel.ForecastRow{
			Period:   forecastPeriodLabel(b.Period),
			Forecast: maskARR(formatRupiah(b.WeightedValue), br),
			BarPct:   barPct(numericFloat(b.WeightedValue), max),
		})
	}
	return rows
}

// buildWinLossReasons — panel 3 tabel: porsi tiap alasan dari total kalah + bar
// relatif alasan terbanyak. Tanpa Rp → tanpa masking.
func buildWinLossReasons(reasons []db.ReportWinLossReasonsRow) []panel.WinLossReasonRow {
	var total, max int64
	for _, r := range reasons {
		total += r.DealCount
		if r.DealCount > max {
			max = r.DealCount
		}
	}
	rows := make([]panel.WinLossReasonRow, 0, len(reasons))
	for _, r := range reasons {
		rows = append(rows, panel.WinLossReasonRow{
			Reason: r.Reason,
			Count:  r.DealCount,
			Porsi:  pctStr(r.DealCount, total),
			BarPct: barPct(float64(r.DealCount), float64(max)),
		})
	}
	return rows
}

// buildFunnelSteps — panel 4 corong: 4 tahap Lead→Terkualifikasi→Deal→Menang,
// porsi & bar relatif total lead masuk (tahap pertama = 100%).
func buildFunnelSteps(f db.ReportLeadFunnelRow, wonCount int64) []panel.FunnelStep {
	total := f.TotalLeads
	steps := []struct {
		label string
		count int64
	}{
		{"Lead Masuk", f.TotalLeads},
		{"Terkualifikasi", f.QualifiedCount},
		{"Jadi Deal", f.DealCount},
		{"Menang", wonCount},
	}
	out := make([]panel.FunnelStep, 0, len(steps))
	for _, s := range steps {
		out = append(out, panel.FunnelStep{
			Label:  s.label,
			Count:  s.count,
			Pct:    pctStr(s.count, total),
			BarPct: barPct(float64(s.count), float64(total)),
		})
	}
	return out
}

// buildActivityRows — panel 5: gabung aktivitas per-owner dengan jumlah deal
// menang per-owner (key owner_id) → kolom Deal Menang & Aktivitas/Deal.
// Aktivitas/Deal = total_count / won_count (1 desimal); won=0 → "—".
func buildActivityRows(acts []db.ReportSalesActivityByOwnerRow, won []db.ReportWonDealsByOwnerRow) []panel.SalesActivityRow {
	wonBy := make(map[int64]int64, len(won))
	for _, w := range won {
		if w.OwnerID != nil {
			wonBy[*w.OwnerID] = w.WonCount
		}
	}
	rows := make([]panel.SalesActivityRow, 0, len(acts))
	for _, a := range acts {
		var wc int64
		if a.OwnerID != nil {
			wc = wonBy[*a.OwnerID]
		}
		perDeal := "—"
		if wc > 0 {
			perDeal = strconv.FormatFloat(float64(a.TotalCount)/float64(wc), 'f', 1, 64)
		}
		rows = append(rows, panel.SalesActivityRow{
			Owner:   activityOwnerLabel(a.OwnerName, a.OwnerEmail),
			Call:    a.CallCount,
			Email:   a.EmailCount,
			Meeting: a.MeetingCount,
			Total:   a.TotalCount,
			Won:     wc,
			PerDeal: perDeal,
		})
	}
	return rows
}
