package handler

import (
	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// reports_subscriptions_rows.go — builder baris panel Subscription Report 8.4
// (BL-47), dipisah dari reports_subscriptions_panels.go (ukuran file). Mengubah
// baris agregat SQL menjadi sub-view siap-render (panel.Sub*Row); masking F4 &
// perhitungan Rp/persen tetap di sini. Perilaku identik; hanya organisasi file.

// buildMRRComponents — Panel 1 tabel pergerakan MRR bulan ini. Nilai Rp
// di-mask F4 (maskARR); Porsi = magnitudo komponen relatif TOTAL magnitudo 4
// komponen (distribusi pergerakan, bukan nilai absolut — tak di-mask, bukan Rp).
func buildMRRComponents(m db.ReportSubMRRRow, br string) []panel.SubMRRComponentRow {
	newV := numericFloat(m.NewMrr)
	expV := numericFloat(m.ExpansionMrr)
	conV := numericFloat(m.ContractionMrr)
	chuV := numericFloat(m.ChurnMrr)
	total := newV + expV + conV + chuV

	comps := []struct {
		label string
		val   float64
		num   pgtype.Numeric
		count int64
	}{
		{"MRR Baru", newV, m.NewMrr, m.NewCount},
		{"Ekspansi", expV, m.ExpansionMrr, m.ExpansionCount},
		{"Kontraksi", conV, m.ContractionMrr, m.ContractionCount},
		{"Churn", chuV, m.ChurnMrr, m.ChurnCount},
	}
	rows := make([]panel.SubMRRComponentRow, 0, len(comps))
	for _, c := range comps {
		rows = append(rows, panel.SubMRRComponentRow{
			Component: c.label,
			Value:     maskARR(formatRupiah(c.num), br),
			Count:     c.count,
			Porsi:     sharePct(c.val, total),
		})
	}
	return rows
}

// buildRenewalMonths — Panel 2 tabel bulanan. Rate = diperpanjang/jatuh-tempo
// bulan itu (ratePct guard pembagian nol). Tak ada nilai Rp → tak di-mask.
func buildRenewalMonths(rows []db.ReportRenewalByMonthRow) []panel.SubRenewalMonthRow {
	out := make([]panel.SubRenewalMonthRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, panel.SubRenewalMonthRow{
			Period:  monthLabelFromDate(r.Period),
			Due:     r.Due,
			Renewed: r.Renewed,
			Rate:    ratePct(r.Renewed, r.Due),
		})
	}
	return out
}

// buildSubChurnReasons — Panel 3 breakdown alasan (REUSE ReportChurnReasons).
// Reason = label Indonesia (churnReasonID). Nilai Hilang Rp di-mask F4. Porsi =
// desa alasan itu relatif TOTAL desa churn (distribusi, bukan Rp → tak di-mask).
func buildSubChurnReasons(rows []db.ReportChurnReasonsRow, br string) []panel.SubChurnReasonRow {
	var total int64
	for _, r := range rows {
		total += r.AccountCount
	}
	out := make([]panel.SubChurnReasonRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, panel.SubChurnReasonRow{
			Reason:    churnReasonID(r.ChurnReason),
			Count:     r.AccountCount,
			LostValue: maskARR(formatRupiah(r.LostValue), br),
			Porsi:     pctStr(r.AccountCount, total),
		})
	}
	return out
}

// buildRevenueByPlan — Panel 4. MRR & Rata per Desa di-mask F4. Bar relatif
// paket MRR terbesar. Rata per Desa = mrr/desa (guard desa=0).
func buildRevenueByPlan(rows []db.ReportRevenueByPlanRow, br string) []panel.SubPlanRevenueRow {
	var max float64
	for _, r := range rows {
		if v := numericFloat(r.Mrr); v > max {
			max = v
		}
	}
	out := make([]panel.SubPlanRevenueRow, 0, len(rows))
	for _, r := range rows {
		mrr := numericFloat(r.Mrr)
		avg := 0.0
		if r.VillageCount > 0 {
			avg = mrr / float64(r.VillageCount)
		}
		out = append(out, panel.SubPlanRevenueRow{
			Plan:   r.PlanName,
			Count:  r.VillageCount,
			MRR:    maskARR(formatRupiah(r.Mrr), br),
			AvgPer: maskARR(rupiahFromFloat(avg), br),
			BarPct: barPct(mrr, max),
		})
	}
	return out
}

// buildSubAging — Panel 5. Label & Catatan per bucket (const). MRR di-mask F4.
// Renewal Rate = diperpanjang/jatuh-tempo; Churn Rate = churned/total bucket.
// Rata Health agregat (bukan PII per-desa).
func buildSubAging(rows []db.ReportSubscriptionAgingRow, br string) []panel.SubAgingRow {
	out := make([]panel.SubAgingRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, panel.SubAgingRow{
			Bucket:      agingBucketLabel(r.Bucket),
			Villages:    r.ActiveVillages,
			MRR:         maskARR(formatRupiah(r.Mrr), br),
			AvgHealth:   healthStr(r.AvgHealth),
			RenewalRate: ratePct(r.RenewedPast, r.DuePast),
			ChurnRate:   ratePct(r.Churned, r.Total),
			Note:        agingBucketNote(r.Bucket),
		})
	}
	return out
}
