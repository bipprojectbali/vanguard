package handler

import (
	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// reports_support_rows.go — builder baris panel Support Report 8.3 (BL-46),
// dipisah dari reports_support_panels.go (ukuran file). Mengubah baris agregat
// SQL menjadi sub-view siap-render (panel.Support*Row). Perilaku identik; hanya
// organisasi file yang berubah.

// buildVolumeRows — panel 1: label bulan + Backlog kumulatif (masuk−selesai
// berjalan) dihitung dari urutan bulan (query ORDER BY period ASC). Backlog
// dijaga ≥0 (selesai kumulatif > masuk terjadi bila tiket dibuat sebelum
// jendela lalu selesai di dalamnya — bukan backlog negatif nyata).
func buildVolumeRows(vols []db.ReportTicketVolumeByMonthRow) []panel.SupportVolumeRow {
	rows := make([]panel.SupportVolumeRow, 0, len(vols))
	var backlog int64
	for _, r := range vols {
		backlog += r.Masuk - r.Selesai
		if backlog < 0 {
			backlog = 0
		}
		rows = append(rows, panel.SupportVolumeRow{
			Period:  forecastPeriodLabel(r.Period),
			Masuk:   r.Masuk,
			Selesai: r.Selesai,
			Backlog: backlog,
		})
	}
	return rows
}

// buildSLARows — panel 2 tabel per-prioritas. Target "N jam" atau "—"
// (target_minutes=0 → tak ada policy, sentinel COALESCE query). MetPct & bar =
// met/with_sla tingkat itu.
func buildSLARows(sla []db.ReportSLAByPriorityRow) []panel.SupportSLARow {
	rows := make([]panel.SupportSLARow, 0, len(sla))
	for _, r := range sla {
		target := slaTargetLabel(r.TargetMinutes)
		metPct := 0
		if r.WithSla > 0 {
			metPct = int(r.Met * 100 / r.WithSla)
		}
		rows = append(rows, panel.SupportSLARow{
			Priority: ticketPriorityLabelID(r.Priority),
			Target:   target,
			MetPct:   ratePct(r.Met, r.WithSla),
			Breached: r.Breached,
			BarPct:   metPct,
		})
	}
	return rows
}

// buildResolutionRows — panel 3 bar: rata jam per prioritas + bar relatif
// prioritas terlama (bar makin panjang = makin lama = makin buruk).
func buildResolutionRows(res []db.ReportResolutionByPriorityRow) []panel.SupportResolutionRow {
	var max float64
	for _, r := range res {
		if v := numericFloat(r.AvgResolutionHours); v > max {
			max = v
		}
	}
	rows := make([]panel.SupportResolutionRow, 0, len(res))
	for _, r := range res {
		rows = append(rows, panel.SupportResolutionRow{
			Priority: ticketPriorityLabelID(r.Priority),
			Avg:      hoursStrN(r.AvgResolutionHours, r.ResolvedCount),
			Count:    r.ResolvedCount,
			BarPct:   barPct(numericFloat(r.AvgResolutionHours), max),
		})
	}
	return rows
}

// hoursStrN = hoursStr tapi pakai count sebagai penanda "belum ada sampel"
// (query FILTER: avg NULL bila count=0 → sama hasilnya; count eksplisit lebih
// jelas untuk pembaca).
func hoursStrN(n pgtype.Numeric, count int64) string {
	if count <= 0 {
		return "—"
	}
	return hoursStr(n)
}

// buildKBRows — panel 4 (sebagian besar dilewatkan): hanya Judul · Dilihat ·
// Status artikel Published.
func buildKBRows(kb []db.ReportKBPublishedRow) []panel.SupportKBRow {
	rows := make([]panel.SupportKBRow, 0, len(kb))
	for _, r := range kb {
		rows = append(rows, panel.SupportKBRow{
			Title:  r.ArticleTitle,
			Views:  r.Views,
			Status: r.Status,
		})
	}
	return rows
}

// buildAgentRows — panel 5: per-agen + badge Status dari agentStatusBadge.
// Nama agen fallback ke email bila users.name kosong.
func buildAgentRows(agents []db.ReportAgentPerformanceRow) []panel.SupportAgentRow {
	rows := make([]panel.SupportAgentRow, 0, len(agents))
	for _, r := range agents {
		name := r.AgentEmail
		if r.AgentName != nil && *r.AgentName != "" {
			name = *r.AgentName
		}
		label, cls := agentStatusBadge(r.WithSla, r.Met, r.Handled, r.Resolved)
		rows = append(rows, panel.SupportAgentRow{
			Agent:         name,
			Handled:       r.Handled,
			Resolved:      r.Resolved,
			AvgResolution: hoursStrN(r.AvgResolutionHours, r.Resolved),
			SLACompliance: ratePct(r.Met, r.WithSla),
			StatusLabel:   label,
			StatusClass:   cls,
		})
	}
	return rows
}
