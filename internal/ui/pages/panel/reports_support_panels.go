package panel

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// reports_support_panels.go — fungsi render 5 panel Support Report 8.3 (BL-46).
// Data & pembungkus (reportSupportPanelCard) di reports_support.go; helper
// bersama reportBar/reportEmpty/reportStat/rTh/rTd/csTableScroll dari
// reports_sales*.go & reports_cs_panels.go (package sama). Tiap <table> lewat
// ui.TableScroll (WAJIB — tabel telanjang meluber di viewport 375px).

// ── Panel 1: Ticket Volume Report ───────────────────────────────────────────

func reportsVolumePanel(v ReportsSupportView) g.Node {
	var body g.Node
	if len(v.VolumeRows) == 0 {
		body = reportEmpty("Belum ada tiket.")
	} else {
		rows := make([]g.Node, 0, len(v.VolumeRows))
		for _, r := range v.VolumeRows {
			rows = append(rows, h.Tr(
				h.Class("border-b border-base-300/50"),
				rTd("font-medium break-words", g.Text(r.Period)),
				rTd("", g.Text(strconv.FormatInt(r.Masuk, 10))),
				rTd("", g.Text(strconv.FormatInt(r.Selesai, 10))),
				rTd("", g.Text(strconv.FormatInt(r.Backlog, 10))),
			))
		}
		body = csTableScroll(
			[]g.Node{rTh("Periode"), rTh("Tiket Masuk"), rTh("Selesai"), rTh("Backlog")},
			rows,
		)
	}
	return reportSupportPanelCard(v.Base, v.Filter.QueryString, "Ticket Volume Report", "volume", body)
}

// ── Panel 2: SLA Compliance Report ──────────────────────────────────────────

func reportsSLAPanel(v ReportsSupportView) g.Node {
	cards := h.Div(h.Class("grid grid-cols-1 sm:grid-cols-3 gap-3 mb-3"),
		reportStat("Terpenuhi", v.SLAMetPct, "text-success"),
		reportStat("Terlanggar", v.SLABreachPct, "text-error"),
		reportStat("Berisiko", strconv.FormatInt(v.SLAAtRisk, 10), "text-warning"),
	)
	var table g.Node
	if len(v.SLARows) == 0 {
		table = reportEmpty("Belum ada tiket ber-SLA.")
	} else {
		rows := make([]g.Node, 0, len(v.SLARows))
		for _, r := range v.SLARows {
			rows = append(rows, h.Tr(
				h.Class("border-b border-base-300/50"),
				rTd("font-medium break-words", g.Text(r.Priority)),
				rTd("", g.Text(r.Target)),
				rTd("", g.Text(r.MetPct)),
				rTd("", g.Text(strconv.FormatInt(r.Breached, 10))),
				h.Td(h.Class("py-2 w-24"), reportBar(r.BarPct)),
			))
		}
		table = csTableScroll(
			[]g.Node{rTh("Prioritas"), rTh("Target SLA"), rTh("Terpenuhi"), rTh("Terlanggar"), rTh("")},
			rows,
		)
	}
	return reportSupportPanelCard(v.Base, v.Filter.QueryString, "SLA Compliance Report", "sla", cards, table)
}

// ── Panel 3: Resolution Time Report ─────────────────────────────────────────

func reportsResolutionPanel(v ReportsSupportView) g.Node {
	var byPriority g.Node
	if len(v.ResolutionRows) == 0 {
		byPriority = reportEmpty("Belum ada tiket selesai.")
	} else {
		rows := make([]g.Node, 0, len(v.ResolutionRows))
		for _, r := range v.ResolutionRows {
			rows = append(rows, h.Tr(
				h.Class("border-b border-base-300/50"),
				rTd("font-medium break-words", g.Text(r.Priority)),
				rTd("", g.Text(r.Avg)),
				rTd("", g.Text(strconv.FormatInt(r.Count, 10))),
				h.Td(h.Class("py-2 w-24"), reportBar(r.BarPct)),
			))
		}
		byPriority = csTableScroll(
			[]g.Node{rTh("Prioritas"), rTh("Rata Penyelesaian"), rTh("Selesai"), rTh("")},
			rows,
		)
	}
	// Baris metrik perbandingan bulan (ini vs lalu + perubahan).
	compare := h.Div(h.Class("grid grid-cols-1 sm:grid-cols-3 gap-3 mt-3"),
		reportStat(v.ResThisLabel, v.ResThisMonth, "text-primary"),
		reportStat(v.ResPrevLabel, v.ResPrevMonth, "text-base-content"),
		reportStat("Perubahan", v.ResChange, "text-secondary"),
	)
	return reportSupportPanelCard(v.Base, v.Filter.QueryString, "Resolution Time Report", "resolution", byPriority, compare)
}

// ── Panel 4: KB Usage Report (sebagian besar dilewatkan) ─────────────────────

func reportsKBPanel(v ReportsSupportView) g.Node {
	var body g.Node
	if len(v.KBRows) == 0 {
		body = reportEmpty("Belum ada artikel Published.")
	} else {
		rows := make([]g.Node, 0, len(v.KBRows))
		for _, r := range v.KBRows {
			rows = append(rows, h.Tr(
				h.Class("border-b border-base-300/50"),
				rTd("font-medium break-words", g.Text(r.Title)),
				rTd("", g.Text(strconv.FormatInt(r.Views, 10))),
				rTd("", g.Text(r.Status)),
			))
		}
		body = csTableScroll(
			[]g.Node{rTh("Judul"), rTh("Dilihat"), rTh("Status")},
			rows,
		)
	}
	return reportSupportPanelCard(v.Base, v.Filter.QueryString, "KB Usage Report", "kb", body)
}

// ── Panel 5: Agent Performance Report (selebar penuh) ────────────────────────

func reportsAgentPanel(v ReportsSupportView) g.Node {
	var body g.Node
	if len(v.AgentRows) == 0 {
		body = reportEmpty("Belum ada tiket yang ditugaskan ke agen.")
	} else {
		rows := make([]g.Node, 0, len(v.AgentRows))
		for _, r := range v.AgentRows {
			rows = append(rows, h.Tr(
				h.Class("border-b border-base-300/50"),
				rTd("font-medium break-words", g.Text(r.Agent)),
				rTd("", g.Text(strconv.FormatInt(r.Handled, 10))),
				rTd("", g.Text(strconv.FormatInt(r.Resolved, 10))),
				rTd("", g.Text(r.AvgResolution)),
				rTd("", g.Text(r.SLACompliance)),
				h.Td(h.Class("py-2"),
					h.Span(h.Class("badge "+r.StatusClass), g.Text(r.StatusLabel)),
				),
			))
		}
		body = csTableScroll(
			[]g.Node{rTh("Agen"), rTh("Tiket Ditangani"), rTh("Selesai"), rTh("Rata Penyelesaian"), rTh("Kepatuhan SLA"), rTh("Status")},
			rows,
		)
	}
	return reportSupportPanelCard(v.Base, v.Filter.QueryString, "Agent Performance Report", "agent", body)
}
