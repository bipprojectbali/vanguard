package handler

import (
	"strconv"
	"strings"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// reports_support_panels.go — builder tiap panel Support Report 8.3 (BL-46):
// mengubah baris agregat SQL (internal/db) menjadi sub-view siap-render
// (panel.Support*Row). Persen/jam/bar-pct dihitung di sini agar view MURNI-DATA.
// Bar CSP-safe: lebar = nilai/maks*100 (int) dipakai h.Style di view (bukan
// script). Data F3 sudah tersaring di query. Helper bersama numericFloat/barPct/
// ratePct/commaDecimal dari reports_sales_panels.go & reports_cs_panels.go.

// Ambang badge Status agen (Panel 5). Diturunkan dari Kepatuhan SLA agen
// (met/with_sla) — deadline SLA berbasis target penyelesaian, jadi kepatuhan
// SLA sudah mencerminkan kecepatan resolusi. Bila agen tak punya tiket ber-SLA,
// dipakai rasio penyelesaian (resolved/handled) sebagai proksi. Konstanta
// bernama (bukan angka tersebar) agar ambang bisnis eksplisit & mudah disetel.
const (
	agentSLAExcellentPct = 90 // ≥90% kepatuhan SLA → Sangat baik
	agentSLAGoodPct      = 70 // ≥70% → Baik; <70% → Perlu bimbingan
	agentResolveGoodPct  = 80 // fallback tanpa SLA: ≥80% resolved/handled → Baik
)

// formatInt = int64 → string desimal (kartu KPI Total Tiket).
func formatInt(n int64) string { return strconv.FormatInt(n, 10) }

// hoursStr = rata jam penyelesaian "N,N jam" (koma Indonesia). NULL/invalid →
// "—" (tak ada tiket selesai → rata tak bermakna; validitas Numeric = ada/tidak
// sampel, bukan angka nol).
func hoursStr(n pgtype.Numeric) string {
	f, err := n.Float64Value()
	if err != nil || !f.Valid {
		return "—"
	}
	return commaDecimal(strconv.FormatFloat(f.Float64, 'f', 1, 64)) + " jam"
}

// slaTargetLabel — Target SLA panel 2 dari resolution_target_minutes policy
// (00016 mengubah satuan jam→menit). Tampil dalam jam (menit/60), ",0" jam bulat
// dipangkas ("24 jam", "1,5 jam"); 0/tanpa policy → "—".
func slaTargetLabel(minutes int32) string {
	if minutes <= 0 {
		return "—"
	}
	s := commaDecimal(strconv.FormatFloat(float64(minutes)/60.0, 'f', 1, 64))
	s = strings.TrimSuffix(s, ",0")
	return s + " jam"
}

// resolutionMonthLabels = label "Bulan ini" & "Bulan lalu" (mis. "Sep 2026").
// Batas bulan query = UTC (date_trunc); label pakai appTZ agar konsisten UI.
func resolutionMonthLabels(now time.Time) (thisLabel, prevLabel string) {
	this := now.Format("2006-01")
	prev := now.AddDate(0, -1, 0).Format("2006-01")
	return forecastPeriodLabel(this), forecastPeriodLabel(prev)
}

// resolutionChange = selisih rata jam (ini − lalu) "▲/▼ N,N jam"; salah satu
// NULL → "—" (tak ada pembanding). Naik = lebih lama (buruk) → panah merah di
// view lewat teks; di sini cukup tanda arah.
func resolutionChange(this, prev pgtype.Numeric) string {
	tf, terr := this.Float64Value()
	pf, perr := prev.Float64Value()
	if terr != nil || perr != nil || !tf.Valid || !pf.Valid {
		return "—"
	}
	delta := tf.Float64 - pf.Float64
	arrow := "▲ "
	if delta < 0 {
		arrow = "▼ "
		delta = -delta
	} else if delta == 0 {
		return "0 jam"
	}
	return arrow + commaDecimal(strconv.FormatFloat(delta, 'f', 1, 64)) + " jam"
}

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

// agentStatusBadge menurunkan badge kinerja agen (label + kelas daisyUI) dari
// kepatuhan SLA; tanpa tiket ber-SLA jatuh ke rasio penyelesaian; tanpa tiket
// sama sekali → "Belum cukup data" (jujur, bukan menebak). Ambang = const di
// atas.
func agentStatusBadge(withSLA, met, handled, resolved int64) (label, class string) {
	switch {
	case withSLA > 0:
		pct := int(met * 100 / withSLA)
		switch {
		case pct >= agentSLAExcellentPct:
			return "Sangat baik", "badge-success"
		case pct >= agentSLAGoodPct:
			return "Baik", "badge-info"
		default:
			return "Perlu bimbingan", "badge-warning"
		}
	case handled > 0:
		if resolved*100/handled >= agentResolveGoodPct {
			return "Baik", "badge-info"
		}
		return "Perlu bimbingan", "badge-warning"
	default:
		return "Belum cukup data", "badge-ghost"
	}
}

// ticketPriorityLabelID menerjemahkan prioritas mentah (rendah/sedang/tinggi) ke
// label Indonesia. Kembar panel.ticketPriorityLabel (package view) tapi handler
// butuh label saat merakit baris; disalin ringkas (3 case) agar tak melintas
// package.
func ticketPriorityLabelID(p string) string {
	switch p {
	case "tinggi":
		return "Tinggi"
	case "sedang":
		return "Sedang"
	case "rendah":
		return "Rendah"
	default:
		return p
	}
}
