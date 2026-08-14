package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// HealthScoreKPIs — hasil CountHealthScoreKPIs untuk kartu KPI header.
type HealthScoreKPIs struct {
	Total    int64
	Healthy  int64
	AtRisk   int64
	Critical int64
}

// HealthScoreRowView — satu baris tabel desa + data health score.
type HealthScoreRowView struct {
	ID          int64
	AccountName string
	Score       string
	StatusLabel string
	StatusBadge string // daisyUI badge class: badge-success / badge-warning / badge-error
	Adoption    string
	Engagement  string
	Support     string
	Sentiment   string
	Trend       string
	DaysInStage string
	HrefDetail  string
}

// HealthScoreListView — data halaman /health-scores.
type HealthScoreListView struct {
	Base       string
	ActiveTab  string
	NextCursor string
	KPIs       HealthScoreKPIs
	Rows       []HealthScoreRowView
}

// HealthScoreList merender body konten halaman workspace-level Health Score (6.1).
// Dipanggil via renderWorkspaceShell — tidak membungkus AppShell sendiri.
func HealthScoreList(v HealthScoreListView) g.Node {
	return h.Div(h.Class("space-y-4"),
		healthScoreKPICards(v.KPIs),
		healthScoreTabs(v.Base, v.ActiveTab),
		healthScoreTable(v),
	)
}

// healthScoreKPICards — 4 kartu KPI: Total / Sehat / Berisiko / Kritis.
func healthScoreKPICards(k HealthScoreKPIs) g.Node {
	return h.Div(h.Class("grid grid-cols-2 md:grid-cols-4 gap-3"),
		healthKPICard("Desa Total", strconv.FormatInt(k.Total, 10), "text-base-content"),
		healthKPICard("Sehat (≥80)", strconv.FormatInt(k.Healthy, 10), "text-success"),
		healthKPICard("Berisiko (40–79)", strconv.FormatInt(k.AtRisk, 10), "text-warning"),
		healthKPICard("Kritis (<40)", strconv.FormatInt(k.Critical, 10), "text-error"),
	)
}

func healthKPICard(label, value, valueClass string) g.Node {
	return h.Div(h.Class("card bg-base-100 shadow-sm"),
		h.Div(h.Class("card-body p-4"),
			h.P(h.Class("text-sm text-base-content/60"), g.Text(label)),
			h.P(h.Class("text-2xl font-bold "+valueClass), g.Text(value)),
		),
	)
}

// healthScoreTabs — filter tab Semua / Sehat / Berisiko / Kritis.
func healthScoreTabs(base, active string) g.Node {
	tabs := []struct{ key, label string }{
		{"", "Semua"},
		{"sehat", "Sehat"},
		{"berisiko", "Berisiko"},
		{"kritis", "Kritis"},
	}
	nodes := make([]g.Node, 0, len(tabs))
	for _, t := range tabs {
		href := base + "/health-scores"
		if t.key != "" {
			href += "?tab=" + t.key
		}
		cls := "tab"
		if t.key == active {
			cls += " tab-active"
		}
		nodes = append(nodes, h.A(h.Class(cls), h.Href(href), g.Text(t.label)))
	}
	return h.Div(h.Class("tabs tabs-bordered flex-wrap"), g.Group(nodes))
}

// healthScoreTable — tabel desa dengan skor dan status.
func healthScoreTable(v HealthScoreListView) g.Node {
	return h.Div(h.Class("card bg-base-100 shadow-sm"),
		h.Div(h.Class("card-body p-0"),
			ui.TableScroll(h.Table(h.Class("table table-sm"),
				h.THead(h.Tr(
					h.Th(g.Text("Desa")),
					h.Th(h.Class("text-center"), g.Text("Skor")),
					h.Th(g.Text("Status")),
					h.Th(h.Class("text-center"), g.Text("Adopsi")),
					h.Th(h.Class("text-center"), g.Text("Engagement")),
					h.Th(h.Class("text-center"), g.Text("Support")),
					h.Th(h.Class("text-center"), g.Text("Sentimen")),
					h.Th(g.Text("Tren")),
					h.Th(g.Text("Di Stage")),
					h.Th(g.Text("")),
				)),
				h.TBody(healthScoreRows(v.Rows)),
			)),
			healthScorePager(v.Base, v.ActiveTab, v.NextCursor),
		),
	)
}

func healthScoreRows(rows []HealthScoreRowView) g.Node {
	if len(rows) == 0 {
		return h.Tr(
			h.Td(h.ColSpan("10"), h.Class("text-center text-base-content/50 py-8"),
				g.Text("Belum ada data health score di ruang kerja ini."),
			),
		)
	}
	nodes := make([]g.Node, 0, len(rows))
	for _, row := range rows {
		nodes = append(nodes, h.Tr(
			h.Td(h.A(h.Href(row.HrefDetail), h.Class("link link-hover font-medium"),
				g.Text(row.AccountName),
			)),
			h.Td(h.Class("text-center font-mono"), g.Text(row.Score)),
			h.Td(h.Span(h.Class("badge badge-sm "+row.StatusBadge), g.Text(row.StatusLabel))),
			h.Td(h.Class("text-center text-sm"), g.Text(row.Adoption)),
			h.Td(h.Class("text-center text-sm"), g.Text(row.Engagement)),
			h.Td(h.Class("text-center text-sm"), g.Text(row.Support)),
			h.Td(h.Class("text-center text-sm"), g.Text(row.Sentiment)),
			h.Td(h.Class("text-sm"), g.Text(row.Trend)),
			h.Td(h.Class("text-sm text-base-content/60"), g.Text(row.DaysInStage)),
			h.Td(h.A(h.Href(row.HrefDetail), h.Class("btn btn-xs btn-ghost"), g.Text("Lihat"))),
		))
	}
	return g.Group(nodes)
}

func healthScorePager(base, tab, nextCursor string) g.Node {
	if nextCursor == "" {
		return nil
	}
	href := base + "/health-scores?after=" + nextCursor
	if tab != "" {
		href += "&tab=" + tab
	}
	return h.Div(h.Class("flex justify-end p-3 border-t border-base-200"),
		h.A(h.Href(href), h.Class("btn btn-sm btn-ghost"), g.Text("Berikutnya →")),
	)
}
