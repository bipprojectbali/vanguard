package panel

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// reports_subscriptions_panels.go — fungsi render 5 panel Subscription Report
// 8.4 (BL-47). Data & pembungkus (reportSubPanelCard) di
// reports_subscriptions.go; helper bersama reportBar/reportEmpty/reportStat/
// rTh/rTd/csTableScroll dari reports_sales*.go & reports_cs_panels.go (package
// sama). Tiap <table> lewat csTableScroll → ui.TableScroll (WAJIB — tabel
// telanjang meluber di viewport 375px).

// ── Panel 1: MRR/ARR Report ─────────────────────────────────────────────────

func reportsSubMRRPanel(v ReportsSubscriptionsView) g.Node {
	cards := h.Div(h.Class("grid grid-cols-1 sm:grid-cols-2 gap-3 mb-3"),
		reportStat("MRR (berjalan)", v.MRR, "text-primary"),
		reportStat("ARR (berjalan)", v.ARR, "text-secondary"),
	)
	var table g.Node
	if len(v.MRRComponents) == 0 {
		table = reportEmpty("Belum ada pergerakan MRR bulan ini.")
	} else {
		rows := make([]g.Node, 0, len(v.MRRComponents))
		for _, r := range v.MRRComponents {
			rows = append(rows, h.Tr(
				h.Class("border-b border-base-300/50"),
				rTd("font-medium break-words", g.Text(r.Component)),
				rTd("", g.Text(r.Value)),
				rTd("", g.Text(strconv.FormatInt(r.Count, 10))),
				rTd("", g.Text(r.Porsi)),
			))
		}
		table = csTableScroll(
			[]g.Node{rTh("Komponen MRR"), rTh("Nilai"), rTh("Desa"), rTh("Porsi")},
			rows,
		)
	}
	return reportSubPanelCard(v.Base, v.Filter.QueryString, "MRR/ARR Report", "mrr", cards, table)
}

// ── Panel 2: Renewal Report ─────────────────────────────────────────────────

func reportsSubRenewalPanel(v ReportsSubscriptionsView) g.Node {
	cards := h.Div(h.Class("grid grid-cols-1 sm:grid-cols-3 gap-3 mb-3"),
		reportStat("Renewal Rate", v.RenewalRateCard, "text-success"),
		reportStat("Nilai Diperpanjang", v.RenewedValue, "text-primary"),
		reportStat("Jatuh Tempo 30 Hari", strconv.FormatInt(v.Due30, 10), "text-warning"),
	)
	var table g.Node
	if len(v.RenewalMonths) == 0 {
		table = reportEmpty("Belum ada langganan ber-jatuh-tempo.")
	} else {
		rows := make([]g.Node, 0, len(v.RenewalMonths))
		for _, r := range v.RenewalMonths {
			rows = append(rows, h.Tr(
				h.Class("border-b border-base-300/50"),
				rTd("font-medium break-words", g.Text(r.Period)),
				rTd("", g.Text(strconv.FormatInt(r.Due, 10))),
				rTd("", g.Text(strconv.FormatInt(r.Renewed, 10))),
				rTd("", g.Text(r.Rate)),
			))
		}
		table = csTableScroll(
			[]g.Node{rTh("Periode"), rTh("Jatuh Tempo"), rTh("Diperpanjang"), rTh("Rate")},
			rows,
		)
	}
	return reportSubPanelCard(v.Base, v.Filter.QueryString, "Renewal Report", "renewal", cards, table)
}

// ── Panel 3: Churn Report ───────────────────────────────────────────────────

func reportsSubChurnPanel(v ReportsSubscriptionsView) g.Node {
	cards := h.Div(h.Class("grid grid-cols-1 sm:grid-cols-3 gap-3 mb-3"),
		reportStat("Desa Churn", strconv.FormatInt(v.ChurnVillages, 10), "text-error"),
		reportStat("Nilai Hilang", v.LostValue, "text-error"),
		reportStat("Rata Umur", v.AvgAge, "text-base-content"),
	)
	var table g.Node
	if len(v.ChurnReasons) == 0 {
		table = reportEmpty("Belum ada langganan berhenti.")
	} else {
		rows := make([]g.Node, 0, len(v.ChurnReasons))
		for _, r := range v.ChurnReasons {
			rows = append(rows, h.Tr(
				h.Class("border-b border-base-300/50"),
				rTd("font-medium break-words", g.Text(r.Reason)),
				rTd("", g.Text(strconv.FormatInt(r.Count, 10))),
				rTd("", g.Text(r.LostValue)),
				rTd("", g.Text(r.Porsi)),
			))
		}
		table = csTableScroll(
			[]g.Node{rTh("Alasan"), rTh("Desa"), rTh("Nilai Hilang"), rTh("Porsi")},
			rows,
		)
	}
	return reportSubPanelCard(v.Base, v.Filter.QueryString, "Churn Report", "churn", cards, table)
}

// ── Panel 4: Revenue by Plan ────────────────────────────────────────────────

func reportsSubRevenuePanel(v ReportsSubscriptionsView) g.Node {
	var table g.Node
	if len(v.RevenueByPlan) == 0 {
		table = reportEmpty("Belum ada langganan aktif ber-paket.")
	} else {
		rows := make([]g.Node, 0, len(v.RevenueByPlan))
		for _, r := range v.RevenueByPlan {
			rows = append(rows, h.Tr(
				h.Class("border-b border-base-300/50"),
				rTd("font-medium break-words", g.Text(r.Plan)),
				rTd("", g.Text(strconv.FormatInt(r.Count, 10))),
				rTd("", g.Text(r.MRR)),
				rTd("", g.Text(r.AvgPer)),
				h.Td(h.Class("py-2 w-24"), reportBar(r.BarPct)),
			))
		}
		table = csTableScroll(
			[]g.Node{rTh("Paket"), rTh("Desa"), rTh("MRR"), rTh("Rata per Desa"), rTh("")},
			rows,
		)
	}
	return reportSubPanelCard(v.Base, v.Filter.QueryString, "Revenue by Plan", "plan", table)
}

// ── Panel 5: Subscription Aging (selebar penuh) ─────────────────────────────

func reportsSubAgingPanel(v ReportsSubscriptionsView) g.Node {
	var body g.Node
	if len(v.AgingRows) == 0 {
		body = reportEmpty("Belum ada langganan ber-tanggal mulai.")
	} else {
		rows := make([]g.Node, 0, len(v.AgingRows))
		for _, r := range v.AgingRows {
			rows = append(rows, h.Tr(
				h.Class("border-b border-base-300/50"),
				rTd("font-medium break-words", g.Text(r.Bucket)),
				rTd("", g.Text(strconv.FormatInt(r.Villages, 10))),
				rTd("", g.Text(r.MRR)),
				rTd("", g.Text(r.AvgHealth)),
				rTd("", g.Text(r.RenewalRate)),
				rTd("", g.Text(r.ChurnRate)),
				rTd("break-words", g.Text(r.Note)),
			))
		}
		body = csTableScroll(
			[]g.Node{rTh("Kelompok Umur"), rTh("Desa"), rTh("MRR"), rTh("Rata Health"), rTh("Renewal Rate"), rTh("Churn Rate"), rTh("Catatan")},
			rows,
		)
	}
	return reportSubPanelCard(v.Base, v.Filter.QueryString, "Subscription Aging", "aging", body)
}
