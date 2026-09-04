package handler

import (
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
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

// bandRow merakit satu baris distribusi (Label · Count · Porsi% · bar). Porsi &
// bar relatif TOTAL desa berskor (denominator) — distribusi, bukan nilai
// absolut. total<=0 → porsi "0%" & bar 0 (pctStr/barPct sudah guard).
func bandRow(label string, count, total int64) panel.ReportBandRow {
	return panel.ReportBandRow{
		Label:  label,
		Count:  count,
		Pct:    pctStr(count, total),
		BarPct: barPct(float64(count), float64(total)),
	}
}

// buildHealthBands — panel 1 (Health Score Report): distribusi band skor
// (Sehat 80–100 / Cukup 60–79 / Berisiko 40–59 / Kritis <40).
func buildHealthBands(h db.ReportCSHealthRow) []panel.ReportBandRow {
	return []panel.ReportBandRow{
		bandRow("Sehat (80–100)", h.BandHealthy, h.Scored),
		bandRow("Cukup (60–79)", h.BandFair, h.Scored),
		bandRow("Berisiko (40–59)", h.BandAtRisk, h.Scored),
		bandRow("Kritis (<40)", h.BandCritical, h.Scored),
	}
}

// buildAdoptionBands — panel 2 (Adoption Report): distribusi band adopsi fitur
// (Tinggi 80–100 / Sedang 60–79 / Rendah 40–59 / Sangat Rendah <40).
func buildAdoptionBands(a db.ReportCSAdoptionRow) []panel.ReportBandRow {
	return []panel.ReportBandRow{
		bandRow("Tinggi (80–100)", a.BandHigh, a.Scored),
		bandRow("Sedang (60–79)", a.BandMid, a.Scored),
		bandRow("Rendah (40–59)", a.BandLow, a.Scored),
		bandRow("Sangat Rendah (<40)", a.BandPoor, a.Scored),
	}
}

// buildOnboardingBands — panel 5 (Onboarding Report): distribusi onboarding_status.
// Label Indonesia; denominator = total desa ber-status.
func buildOnboardingBands(o db.ReportOnboardingRow) []panel.ReportBandRow {
	return []panel.ReportBandRow{
		bandRow("Selesai", o.Completed, o.Total),
		bandRow("Berjalan", o.InProgress, o.Total),
		bandRow("Belum Mulai", o.NotStarted, o.Total),
		bandRow("Tersendat", o.Stalled, o.Total),
	}
}

// buildChurnRows — panel 3 tabel Alasan Churn. Nilai Hilang di-mask F4
// (maskARR): Support lihat penanda tersembunyi, bukan Rupiah. Porsi relatif
// total desa churned (jumlah semua baris).
func buildChurnRows(rows []db.ReportChurnReasonsRow, businessRole string) []panel.CSChurnRow {
	var total int64
	for _, r := range rows {
		total += r.AccountCount
	}
	out := make([]panel.CSChurnRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, panel.CSChurnRow{
			Reason:    r.ChurnReason,
			Count:     r.AccountCount,
			Porsi:     pctStr(r.AccountCount, total),
			LostValue: maskARR(formatRupiah(r.LostValue), businessRole),
		})
	}
	return out
}

// buildComplianceRows — panel 6 kepatuhan per engagement_type: TouchPoint =
// "done / total"; Compliance = done/total%; bar relatif kepatuhan (done vs
// total baris itu sendiri, bukan lintas-tipe).
func buildComplianceRows(rows []db.ReportEngagementComplianceRow) []panel.CSComplianceRow {
	out := make([]panel.CSComplianceRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, panel.CSComplianceRow{
			Type:       engagementTypeLabel(r.EngagementType),
			TouchPoint: strconv.FormatInt(r.Done, 10) + " / " + strconv.FormatInt(r.Total, 10),
			Compliance: pctStr(r.Done, r.Total),
			BarPct:     barPct(float64(r.Done), float64(r.Total)),
		})
	}
	return out
}

// buildEngagementCSMRows — panel 6 tabel per-CSM. owner_id NULL (engagement tak
// ber-pemilik) → "— (Tak ditugaskan)"; nama kosong → email. Desa Dipegang =
// accounts.assigned_csm = owner (subquery SQL).
func buildEngagementCSMRows(rows []db.ReportEngagementByCSMRow) []panel.CSEngagementCSMRow {
	out := make([]panel.CSEngagementCSMRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, panel.CSEngagementCSMRow{
			CSM:        ownerDisplay(r.OwnerID, r.OwnerName, r.OwnerEmail),
			Villages:   r.AccountsAssigned,
			TouchPoint: strconv.FormatInt(r.Done, 10) + " / " + strconv.FormatInt(r.Total, 10),
			Compliance: pctStr(r.Done, r.Total),
			BarPct:     barPct(float64(r.Done), float64(r.Total)),
		})
	}
	return out
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
