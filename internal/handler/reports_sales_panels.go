package handler

import (
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// reports_sales_panels.go — builder tiap panel Sales Report 8.1 (BL-43):
// mengubah baris agregat SQL (internal/db) menjadi sub-view siap-render
// (panel.*Row). Semua nilai Rp lewat maskARR (F4); persen/porsi/bar-pct dihitung
// di sini agar view MURNI-DATA. Bar CSP-safe: lebar = nilai/maks*100 (int),
// dipakai h.Style di view (bukan script). Data F3 sudah tersaring di query.

// numericFloat membaca pgtype.Numeric ke float64 (0 bila NULL/invalid).
// Dipakai untuk lebar bar & rata-rata hari — bukan untuk tampilan Rp (itu
// formatRupiah/numericStr).
func numericFloat(n pgtype.Numeric) float64 {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return 0
	}
	return f.Float64
}

// barPct = porsi val terhadap max, dibulatkan 0–100. max<=0 → 0 (hindari bagi
// nol; semua bar kosong bila tak ada nilai positif).
func barPct(val, max float64) int {
	if max <= 0 || val <= 0 {
		return 0
	}
	p := int(val / max * 100)
	if p > 100 {
		return 100
	}
	return p
}

// pctStr = porsi part/total sebagai "N%" (bulat). total<=0 → "0%".
func pctStr(part, total int64) string {
	if total <= 0 {
		return "0%"
	}
	return strconv.FormatInt(part*100/total, 10) + "%"
}

// daysStr = rata-rata hari "N,N hari" (koma desimal Indonesia); count<=0 →
// "—" (rata tak bermakna tanpa sampel — SQL sudah COALESCE NULL→0, jadi 0 di
// sini berarti "belum ada", bukan "nol hari").
func daysStr(avg pgtype.Numeric, count int64) string {
	if count <= 0 {
		return "—"
	}
	s := strconv.FormatFloat(numericFloat(avg), 'f', 1, 64)
	// "3.5" → "3,5" (konvensi angka Indonesia, konsisten UI lain).
	out := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '.' {
			out[i] = ','
		} else {
			out[i] = s[i]
		}
	}
	return string(out) + " hari"
}

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

// activityOwnerLabel = nama tampilan sales (nama → email → penanda). Nama tim
// internal, bukan PII pelanggan → tak di-mask (beda dari maskEmail kontak desa).
func activityOwnerLabel(name, email *string) string {
	if name != nil && *name != "" {
		return *name
	}
	if email != nil && *email != "" {
		return *email
	}
	return "(Tanpa pemilik)"
}

// forecastPeriodLabel mengubah 'YYYY-MM' (urut leksikografis di SQL) → "Mon YYYY"
// singkat Indonesia untuk tampilan. Format tak dikenal → apa adanya.
func forecastPeriodLabel(period string) string {
	if len(period) != 7 || period[4] != '-' {
		return period
	}
	months := [...]string{"", "Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Ags", "Sep", "Okt", "Nov", "Des"}
	m, err := strconv.Atoi(period[5:7])
	if err != nil || m < 1 || m > 12 {
		return period
	}
	return months[m] + " " + period[:4]
}
