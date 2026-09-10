package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// HealthScoreKPIs — hasil CountHealthScoreKPIs untuk kartu KPI header.
// Sub-teks (BL-96) diturunkan di handler (view murni-data): AvgScoreSub &
// HealthySub bergantung data; sub Berisiko/Kritis statis dirender di view.
type HealthScoreKPIs struct {
	Total       int64
	Healthy     int64
	AtRisk      int64
	Critical    int64
	AvgScoreSub string // "rata skor 78" / "belum ada skor"
	HealthySub  string // "40% dari binaan"
}

// HealthScoreRowView — satu baris tabel desa + data health score.
// BL-96: kolom Sentimen dibuang; "Di Stage" → RenewalDue (jatuh tempo langganan);
// ActionLabel = aksi kontekstual per-status (Kritis→Playbook, Berisiko→Tinjau,
// selain itu→Lihat).
type HealthScoreRowView struct {
	ID          int64
	AccountName string
	Score       string
	StatusLabel string
	StatusBadge string // daisyUI badge class: badge-success / badge-warning / badge-error
	Adoption    string
	Engagement  string
	Support     string
	Trend       string
	RenewalDue  string
	ActionLabel string
	HrefDetail  string
}

// HealthScoreListView — data halaman /health-scores.
type HealthScoreListView struct {
	Base          string
	ActiveTab     string
	Segment       string // BL-114: "active" (default) | "churned"; segmen populasi
	Query         string // ?q= pencarian bebas (BL-6); "" = tak mencari
	NextCursor    string
	After         string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail         string // BL-7: jejak cursor halaman sebelumnya (?trail=)
	KPIs          HealthScoreKPIs
	Panels        HealthDashPanels // BL-96: panel Sebaran + Komposisi/Arah
	TableSubtitle string           // BL-96: mis. "Diurutkan dari terbaru · 42 desa binaan"
	Rows          []HealthScoreRowView
}

// HealthScoreList merender body konten halaman workspace-level Health Score (6.1).
// Dipanggil via renderWorkspaceShell — tidak membungkus AppShell sendiri.
func HealthScoreList(v HealthScoreListView) g.Node {
	seg := healthSegKeep(v.Segment)
	return h.Div(h.Class("space-y-4"),
		healthScoreSegments(v.Base, v.Segment, v.ActiveTab, v.Query),
		healthScoreKPICards(v.KPIs),
		healthDashPanels(v.Panels),
		tabSearchRow(healthScoreTabs(v.Base, v.ActiveTab, v.Query, seg),
			searchBoxInline(v.Base+"/health-scores", v.Query,
				"Cari desa…", "Cari health score",
				hiddenField{"tab", v.ActiveTab}, seg)),
		healthScoreTable(v),
	)
}

// healthSegKeep → hiddenField segment yang HANYA non-kosong untuk "churned"
// (withQuery/panelListHref mengabaikan value kosong) agar URL segmen default
// (active) tetap bersih tanpa ?segment=active.
func healthSegKeep(segment string) hiddenField {
	if segment == "churned" {
		return hiddenField{"segment", "churned"}
	}
	return hiddenField{"segment", ""}
}

// healthScoreSegments — pemilih segmen populasi (BL-114): Desa Aktif (pelanggan
// berlangganan hidup) vs Churned (eks-pelanggan). Prospek tanpa langganan tak
// muncul di keduanya. Tab status & pencarian dipertahankan saat berganti segmen.
func healthScoreSegments(base, active, tab, query string) g.Node {
	segs := []struct{ key, label string }{
		{"active", "Desa Aktif"},
		{"churned", "Churned"},
	}
	if active == "" {
		active = "active"
	}
	nodes := make([]g.Node, 0, len(segs))
	for _, s := range segs {
		href := withQuery(base+"/health-scores", query,
			hiddenField{"tab", tab}, hiddenField{"segment", s.key})
		cls := "tab"
		if s.key == active {
			cls += " tab-active"
		}
		nodes = append(nodes, h.A(h.Class(cls), h.Href(href), g.Text(s.label)))
	}
	return h.Div(h.Class("tabs tabs-boxed w-fit"), g.Group(nodes))
}

// healthScoreKPICards — 4 kartu KPI: Total / Sehat / Berisiko / Kritis. Ambang
// di label (≥80 / 40–79 / <40) HARUS cermin healthHealthyMin/healthAtRiskMin
// (handler/customer_success_view.go, sumber status turunan BL-24) — ubah salah
// satu tanpa yang lain = UI menjanjikan pemetaan skor→status yang tak ditegakkan.
func healthScoreKPICards(k HealthScoreKPIs) g.Node {
	return h.Div(h.Class("grid grid-cols-2 md:grid-cols-4 gap-3"),
		healthKPICard("Desa Binaan", strconv.FormatInt(k.Total, 10), "text-base-content", k.AvgScoreSub),
		healthKPICard("Sehat (≥80)", strconv.FormatInt(k.Healthy, 10), "text-success", k.HealthySub),
		healthKPICard("Berisiko (40–79)", strconv.FormatInt(k.AtRisk, 10), "text-warning", "perlu ditindaklanjuti"),
		healthKPICard("Kritis (<40)", strconv.FormatInt(k.Critical, 10), "text-error", "perlu playbook aktif"),
	)
}

// healthKPICard — kartu KPI dengan sub-teks (BL-96). sub kosong → baris sub
// tetap dirender kosong agar tinggi kartu seragam antar-kolom (grid stretch).
func healthKPICard(label, value, valueClass, sub string) g.Node {
	return h.Div(h.Class("card bg-base-100 shadow-sm"),
		h.Div(h.Class("card-body p-4"),
			h.P(h.Class("text-sm text-base-content/60"), g.Text(label)),
			h.P(h.Class("text-2xl font-bold "+valueClass), g.Text(value)),
			h.P(h.Class("text-xs text-base-content/50"), g.Text(sub)),
		),
	)
}

// healthScoreTabs — filter tab Semua / Sehat / Berisiko / Kritis. seg dibawa agar
// perpindahan tab mempertahankan segmen populasi aktif (BL-114).
func healthScoreTabs(base, active, query string, seg hiddenField) g.Node {
	tabs := []struct{ key, label string }{
		{"", "Semua"},
		{"sehat", "Sehat"},
		{"berisiko", "Berisiko"},
		{"kritis", "Kritis"},
	}
	nodes := make([]g.Node, 0, len(tabs))
	for _, t := range tabs {
		href := withQuery(base+"/health-scores", query, hiddenField{"tab", t.key}, seg)
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
			g.If(v.TableSubtitle != "",
				h.P(h.Class("text-xs text-base-content/50 px-4 pt-3"), g.Text(v.TableSubtitle))),
			ui.TableScroll(h.Table(h.Class("table table-sm"),
				h.THead(h.Tr(
					h.Th(g.Text("Desa")),
					h.Th(h.Class("text-center"), g.Text("Skor")),
					h.Th(g.Text("Status")),
					h.Th(h.Class("text-center"), g.Text("Adopsi")),
					h.Th(h.Class("text-center"), g.Text("Engagement")),
					h.Th(h.Class("text-center"), g.Text("Support")),
					h.Th(g.Text("Tren")),
					h.Th(g.Text("Jatuh Tempo")),
					h.Th(g.Text("")),
				)),
				h.TBody(healthScoreRows(v.Rows, v.Base, v.ActiveTab, v.Query, healthSegKeep(v.Segment))),
			)),
			healthScorePager(v.Base, v.ActiveTab, v.NextCursor, v.Query, v.After, v.Trail, healthSegKeep(v.Segment)),
		),
	)
}

func healthScoreRows(rows []HealthScoreRowView, base, tab, query string, seg hiddenField) g.Node {
	if len(rows) == 0 {
		msg := "Belum ada data health score di ruang kerja ini."
		if seg.Value == "churned" {
			msg = "Belum ada desa churned di ruang kerja ini."
		}
		cell := []g.Node{
			h.ColSpan("9"), h.Class("text-center text-base-content/50 py-8"),
		}
		if query != "" {
			msg = "Belum ada desa yang cocok pencarian."
			cell = append(cell, h.Div(g.Text(msg)),
				h.A(h.Href(withQuery(base+"/health-scores", "", hiddenField{"tab", tab}, seg)),
					h.Class("btn btn-ghost btn-sm min-h-11 mt-2"), g.Text("« Reset pencarian")))
			return h.Tr(h.Td(cell...))
		}
		cell = append(cell, g.Text(msg))
		return h.Tr(h.Td(cell...))
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
			h.Td(h.Class("text-sm"), g.Text(row.Trend)),
			h.Td(h.Class("text-sm text-base-content/60"), g.Text(row.RenewalDue)),
			h.Td(h.A(h.Href(row.HrefDetail), h.Class("btn btn-xs btn-ghost"), g.Text(row.ActionLabel))),
		))
	}
	return g.Group(nodes)
}

func healthScorePager(base, tab, nextCursor, query, after, trail string, seg hiddenField) g.Node {
	// Ujung daftar pada satu halaman: tanpa footer (perilaku lama dipertahankan).
	if nextCursor == "" && trail == "" {
		return nil
	}
	href := panelListHref(base+"/health-scores", [2]string{"tab", tab},
		[2]string{"q", query}, [2]string{seg.Name, seg.Value})
	return h.Div(h.Class("p-3 border-t border-base-200"),
		ui.KeysetPager(href, after, trail, nextCursor))
}
