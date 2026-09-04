package handler

import (
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// reports_cs_rows.go — builder band/baris panel Customer Success Report,
// dipisah dari reports_cs_panels.go (ukuran file). Perilaku identik; hanya
// organisasi file yang berubah.

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
