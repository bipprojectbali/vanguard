package handler

import (
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"
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
