package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// reports_cs_panels.go — fungsi render 6 panel Customer Success Report 8.2
// (BL-45). Data & pembungkus (reportCSPanelCard/reportBandTable) di reports_cs.go;
// helper bersama reportBar/reportEmpty/reportStat/rTh/rTd dari reports_sales*.go
// (package sama). Tiap <table> lewat ui.TableScroll (WAJIB — tabel telanjang
// meluber di viewport 375px). NPS/CSAT (panel 4) TAK dirender: tak ada datanya.

// csTableScroll = tabel seragam CS terbungkus ui.TableScroll. head = sel <th>,
// rows = sel <tr> <tbody>. Menutup pengulangan kelas THead/Table panjang.
func csTableScroll(head, rows []g.Node) g.Node {
	return ui.TableScroll(h.Table(
		h.Class("w-full text-sm"),
		h.THead(h.Tr(
			h.Class("border-b border-base-300 text-left text-base-content/70"),
			g.Group(head),
		)),
		h.TBody(g.Group(rows)),
	))
}

// ── Panel 1: Health Score Report ────────────────────────────────────────────

func reportsHealthPanel(v ReportsCSView) g.Node {
	var body g.Node
	if v.HealthScored == 0 {
		body = reportEmpty("Belum ada desa dengan health score.")
	} else {
		body = reportBandTable("Band Kesehatan", v.HealthBands)
	}
	return reportCSPanelCard(v.Base, "Health Score Report", "health", body)
}

// ── Panel 2: Adoption Report (sederhana) ────────────────────────────────────

func reportsAdoptionPanel(v ReportsCSView) g.Node {
	var body g.Node
	if v.AdoptionScored == 0 {
		body = reportEmpty("Belum ada desa dengan skor adopsi fitur.")
	} else {
		body = reportBandTable("Band Adopsi", v.AdoptionBands)
	}
	return reportCSPanelCard(v.Base, "Adoption Report", "adoption", body)
}

// ── Panel 3: Retention/Churn Report ─────────────────────────────────────────

func reportsRetentionPanel(v ReportsCSView) g.Node {
	cards := h.Div(h.Class("grid grid-cols-2 gap-3 mb-3"),
		reportStat("Retention ("+strconv.FormatInt(v.ActiveCount, 10)+" aktif)", v.RetentionPct, "text-success"),
		reportStat("Churn ("+strconv.FormatInt(v.ChurnedCount, 10)+" churn)", v.ChurnPct, "text-error"),
	)
	var reasons g.Node
	if len(v.ChurnReasons) == 0 {
		reasons = reportEmpty("Belum ada langganan churn.")
	} else {
		rows := make([]g.Node, 0, len(v.ChurnReasons))
		for _, r := range v.ChurnReasons {
			rows = append(rows, h.Tr(
				h.Class("border-b border-base-300/50"),
				rTd("font-medium break-words", g.Text(r.Reason)),
				rTd("", g.Text(strconv.FormatInt(r.Count, 10))),
				rTd("", g.Text(r.Porsi)),
				rTd("", g.Text(r.LostValue)),
			))
		}
		reasons = csTableScroll(
			[]g.Node{rTh("Alasan Churn"), rTh("Desa"), rTh("Porsi"), rTh("Nilai Hilang")},
			rows,
		)
	}
	return reportCSPanelCard(v.Base, "Retention/Churn Report", "retention", cards, reasons)
}

// ── Panel 5: Onboarding Report (panel 4 NPS/CSAT dilewatkan) ─────────────────

func reportsOnboardingPanel(v ReportsCSView) g.Node {
	cards := h.Div(h.Class("grid grid-cols-1 sm:grid-cols-3 gap-3 mb-3"),
		reportStat("Rata Durasi", v.OnboardAvgDuration, "text-primary"),
		reportStat("Selesai", strconv.FormatInt(v.OnboardCompleted, 10), "text-success"),
		reportStat("Terlambat", strconv.FormatInt(v.OnboardLate, 10), "text-warning"),
	)
	return reportCSPanelCard(v.Base, "Onboarding Report", "onboarding",
		cards, reportBandTable("Status", v.OnboardStatus))
}

// ── Panel 6: Engagement Report (kepatuhan per tipe + per CSM) ────────────────

func reportsEngagementPanel(v ReportsCSView) g.Node {
	var byType g.Node
	if len(v.EngagementTypes) == 0 {
		byType = reportEmpty("Belum ada engagement terjadwal.")
	} else {
		rows := make([]g.Node, 0, len(v.EngagementTypes))
		for _, r := range v.EngagementTypes {
			rows = append(rows, h.Tr(
				h.Class("border-b border-base-300/50"),
				rTd("font-medium break-words", g.Text(r.Type)),
				rTd("", g.Text(r.TouchPoint)),
				rTd("", g.Text(r.Compliance)),
				h.Td(h.Class("py-2 w-24"), reportBar(r.BarPct)),
			))
		}
		byType = csTableScroll(
			[]g.Node{rTh("Tipe"), rTh("Touch Point"), rTh("Kepatuhan"), rTh("")},
			rows,
		)
	}

	var byCSM g.Node
	if len(v.EngagementCSMs) == 0 {
		byCSM = reportEmpty("Belum ada engagement per CSM.")
	} else {
		rows := make([]g.Node, 0, len(v.EngagementCSMs))
		for _, r := range v.EngagementCSMs {
			rows = append(rows, h.Tr(
				h.Class("border-b border-base-300/50"),
				rTd("font-medium break-words", g.Text(r.CSM)),
				rTd("", g.Text(strconv.FormatInt(r.Villages, 10))),
				rTd("", g.Text(r.TouchPoint)),
				rTd("", g.Text(r.Compliance)),
				h.Td(h.Class("py-2 w-24"), reportBar(r.BarPct)),
			))
		}
		byCSM = csTableScroll(
			[]g.Node{rTh("CSM"), rTh("Desa Dipegang"), rTh("Touch Point"), rTh("Kepatuhan"), rTh("")},
			rows,
		)
	}

	// Sub-judul + tautan CSV per-CSM terpisah (panelKey berbeda) di dalam kartu
	// yang sama; kartu utama meng-export kepatuhan per-tipe.
	perCSMHead := h.Div(h.Class("flex flex-wrap items-center justify-between gap-2 mt-4 mb-2"),
		h.H3(h.Class("font-medium text-sm text-base-content/70"), g.Text("Per CSM")),
		h.A(
			h.Href(v.Base+"/reports/customer-success/export?panel=engagement-csm"),
			h.Class("btn btn-outline btn-xs min-h-9"),
			g.Text("Export CSV"),
		),
	)
	return reportCSPanelCard(v.Base, "Engagement Report", "engagement", byType, perCSMHead, byCSM)
}
