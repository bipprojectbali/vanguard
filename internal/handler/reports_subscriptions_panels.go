package handler

import (
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// reports_subscriptions_panels.go — builder tiap panel Subscription Report 8.4
// (BL-47): mengubah baris agregat SQL (internal/db) menjadi sub-view
// siap-render (panel.Sub*Row). Rp/persen/porsi + masking F4 (maskARR) dihitung
// DI SINI agar view MURNI-DATA. Bar CSP-safe: lebar = int (barPct) dipakai
// h.Style di view. Data F3 sudah tersaring di query. Helper bersama
// numericFloat/barPct/ratePct/formatRupiah/maskARR dari reports_sales_panels.go
// & reports_cs_panels.go & fls.go.

// churnReasonLabelID memetakan picklist churn_reason (Inggris, 00012) → label
// Indonesia untuk tampilan. TIDAK menambah nilai enum baru — hanya label
// tampilan (spec 8.4). Nilai tak dikenal / '(Tanpa alasan)' diteruskan apa
// adanya (fail-safe: pembaca tetap lihat sesuatu, bukan string kosong).
var churnReasonLabelID = map[string]string{
	"Budget":               "Anggaran tidak lanjut",
	"No Adoption":          "Adopsi rendah",
	"Change of Leadership": "Pergantian pimpinan",
	"Competitor":           "Pindah vendor",
	"Dissatisfaction":      "Ketidakpuasan",
	"Feature Gap":          "Fitur kurang",
}

func churnReasonID(raw string) string {
	if v, ok := churnReasonLabelID[raw]; ok {
		return v
	}
	return raw
}

// Label & Catatan interpretatif per bucket umur (Panel 5). Const bernama
// (bukan angka/teks tersebar) agar ambang & narasi bisnis eksplisit. Index =
// bucket key numerik dari query (1..4); indeks 0 = fallback aman.
var (
	agingBucketLabels = [...]string{"—", "< 6 bulan", "6–12 bulan", "1–2 tahun", "> 2 tahun"}
	agingBucketNotes  = [...]string{
		"—",
		"Baru — fokus onboarding & adopsi awal",
		"Stabilisasi — pastikan nilai terasa sebelum tahun pertama",
		"Matang — peluang ekspansi & penguatan retensi",
		"Loyal — jaga hubungan, waspadai kejenuhan",
	}
)

func agingBucketLabel(b int32) string {
	if b >= 1 && int(b) < len(agingBucketLabels) {
		return agingBucketLabels[b]
	}
	return agingBucketLabels[0]
}

func agingBucketNote(b int32) string {
	if b >= 1 && int(b) < len(agingBucketNotes) {
		return agingBucketNotes[b]
	}
	return agingBucketNotes[0]
}

// sharePct = porsi val/total sebagai "N%" (bulat). total<=0 atau val<=0 → "0%".
// Untuk pembagian nilai float (Rp) yang tak muat int64 aman di pctStr.
func sharePct(val, total float64) string {
	if total <= 0 || val <= 0 {
		return "0%"
	}
	return strconv.FormatInt(int64(val/total*100+0.5), 10) + "%"
}

// ageDaysStr = rata umur langganan churn "N hari" (bulat). count<=0 → "—"
// (tak ada sampel; query FILTER NULL→invalid, count eksplisit lebih jelas).
func ageDaysStr(n pgtype.Numeric, count int64) string {
	if count <= 0 {
		return "—"
	}
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return "—"
	}
	return strconv.FormatInt(int64(f.Float64+0.5), 10) + " hari"
}

// healthStr = rata health score bucket (Panel 5) "N" (bulat 0–100) atau "—"
// bila tak ada desa ber-CS (LEFT JOIN → NULL).
func healthStr(n pgtype.Numeric) string {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return "—"
	}
	return strconv.FormatInt(int64(f.Float64+0.5), 10)
}

// rupiahFromFloat = float (mis. rata MRR/desa hasil bagi) → "Rp 5.000.000"
// (bulat, pemisah ribuan). Meniru formatRupiah tapi dari float turunan
// (formatRupiah hanya terima Numeric). Dipakai kolom Rata per Desa (Panel 4).
func rupiahFromFloat(v float64) string {
	n := int64(v + 0.5)
	neg := n < 0
	if neg {
		n = -n
	}
	g := groupThousands(strconv.FormatInt(n, 10))
	if neg {
		return "Rp -" + g
	}
	return "Rp " + g
}

// monthLabelFromDate = pgtype.Date (awal bulan) → "Mon YYYY" via
// forecastPeriodLabel. Invalid → "—".
func monthLabelFromDate(d pgtype.Date) string {
	if !d.Valid {
		return "—"
	}
	return forecastPeriodLabel(d.Time.Format("2006-01"))
}

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
