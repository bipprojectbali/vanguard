package handler

import (
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// reports_cs_panels.go — builder tiap panel Customer Success Report 8.2 (BL-45):
// mengubah baris agregat SQL (internal/db) menjadi sub-view siap-render
// (panel.*Row). Nilai Rp (Nilai Hilang churn) lewat maskARR (F4); persen/porsi/
// bar-pct dihitung di sini agar view MURNI-DATA. Bar CSP-safe (lebar int →
// h.Style di view). Format angka reuse numericFloat/daysStr (reports_sales_
// panels.go, package sama). Data F3 sudah tersaring di query.

// csAvgStr = rata skor 1 desimal (koma Indonesia), TANPA satuan. count<=0 →
// "—" (rata tak bermakna tanpa sampel — SQL FILTER NULL, jadi count=scored).
// Dipakai KPI Rata Health Score.
func csAvgStr(avg pgtype.Numeric, count int64) string {
	if count <= 0 {
		return "—"
	}
	return commaDecimal(strconv.FormatFloat(numericFloat(avg), 'f', 1, 64))
}

// csPctStr = rata dalam persen ("NN,N%"); feature_adoption_rate SUDAH skala
// 0–100 jadi cukup tempel "%". count<=0 → "—". Dipakai KPI Adoption Rate.
func csPctStr(avg pgtype.Numeric, count int64) string {
	if count <= 0 {
		return "—"
	}
	return commaDecimal(strconv.FormatFloat(numericFloat(avg), 'f', 1, 64)) + "%"
}

// ratePct = part/whole sebagai "NN,N%" (1 desimal). whole<=0 → "—" (belum ada
// langganan → retention tak bermakna). Dipakai KPI Retention + kartu Retention/
// Churn panel 3.
func ratePct(part, whole int64) string {
	if whole <= 0 {
		return "—"
	}
	return commaDecimal(strconv.FormatFloat(float64(part)*100/float64(whole), 'f', 1, 64)) + "%"
}

// commaDecimal menukar titik desimal → koma (konvensi angka Indonesia, konsisten
// daysStr & UI lain). Hanya sentuh '.', biarkan digit/tanda lain.
func commaDecimal(s string) string {
	return strings.ReplaceAll(s, ".", ",")
}

// ownerDisplay memilih label pemilik: NULL id → penanda tak-ditugaskan; nama
// kosong → email; keduanya kosong → "#<id>". Meniru cara reports_sales
// menampilkan owner tapi lokal (nama+email berasal dari LEFT JOIN users).
func ownerDisplay(id *int64, name, email *string) string {
	if id == nil {
		return "— (Tak ditugaskan)"
	}
	if name != nil && *name != "" {
		return *name
	}
	if email != nil && *email != "" {
		return *email
	}
	return "#" + strconv.FormatInt(*id, 10)
}
