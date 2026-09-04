package handler

import (
	"strconv"

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
