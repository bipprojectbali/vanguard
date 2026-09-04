package panel

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// reports_support.go — Support Report (Modul 8, wireframe 8.3, BL-46): view
// MURNI-DATA; handler (internal/handler/reports_support*.go) yang menghitung &
// memformat (persen/jam/koma Indonesia). Struktur data 5 panel di sini; fungsi
// render tiap panel di reports_support_panels.go (pola sama reports_cs). Kartu
// KPI meniru dashboardKPICard (package sama); tiap <table> dibungkus
// ui.TableScroll (WAJIB, gotcha overflow).
//
// Panel/kolom yang datanya BELUM ADA TAK dirender — keputusan sadar BL-46
// (docs/crm/tasks.md, "data belum ada DILEWATKAN dulu"): CSAT (KPI ke-4, tak
// ada tabel survei), bar per-kategori & Kanal (tak ada kolom), Respons pertama
// & FCR/Sekali-selesai (tak ada first_response_at/reopen), KB rating/deflection/
// membantu (tak ada kolomnya).

// ── Struktur data 5 panel (semua string SUDAH diformat di handler) ──

// SupportVolumeRow = satu baris bulanan panel 1 (Ticket Volume). Backlog =
// kumulatif (masuk−selesai berjalan) DIHITUNG handler dari urutan bulan.
type SupportVolumeRow struct {
	Period  string
	Masuk   int64
	Selesai int64
	Backlog int64
}

// SupportSLARow = satu baris per-prioritas panel 2 (SLA Compliance). Target =
// "N jam" atau "—" (tak ada policy). MetPct "%"; BarPct = kepatuhan tingkat itu.
type SupportSLARow struct {
	Priority string
	Target   string
	MetPct   string
	Breached int64
	BarPct   int
}

// SupportResolutionRow = satu baris per-prioritas panel 3 (bar rata jam
// penyelesaian). Avg "N,N jam" atau "—"; BarPct relatif prioritas terlama.
type SupportResolutionRow struct {
	Priority string
	Avg      string
	Count    int64
	BarPct   int
}

// SupportKBRow = satu artikel Published panel 4 (sebagian besar dilewatkan).
type SupportKBRow struct {
	Title  string
	Views  int64
	Status string
}

// SupportAgentRow = satu agen panel 5. Semua kolom SUDAH diformat; Status =
// badge kinerja (label + kelas daisyUI) diturunkan handler dari kepatuhan SLA
// + rasio penyelesaian (const ambang, bukan hardcode tersebar).
type SupportAgentRow struct {
	Agent         string
	Handled       int64
	Resolved      int64
	AvgResolution string
	SLACompliance string
	StatusLabel   string
	StatusClass   string
}

// SupportReportFilterView = sub-view filter interaktif Periode + Prioritas
// (BL-51). Reuse SalesFilterOption (bentuk opsi identik). QueryString ditempel ke
// tautan Export CSV agar CSV tersaring identik HTML. CustomStart/End = echo input
// date (YYYY-MM-DD) saat Periode = Kustom. Kedua dropdown selalu dirender (enum
// tetap, tak bergantung scope).
type SupportReportFilterView struct {
	PeriodValue   string
	Periods       []SalesFilterOption
	PriorityValue string
	Priorities    []SalesFilterOption
	CustomStart   string
	CustomEnd     string
	QueryString   string
}

// ReportsSupportView — data siap-render /reports/support (3 KPI nyata + 5
// panel; CSAT dilewatkan). String KPI "—" bila tak bermakna.
type ReportsSupportView struct {
	Base string

	// Filter interaktif Periode + Prioritas (BL-51).
	Filter SupportReportFilterView

	// KPI ringkas (CSAT dilewatkan → 3 kartu).
	TotalTickets  string
	SLACompliance string
	AvgResolution string

	// Panel 1 — Ticket Volume Report.
	VolumeRows []SupportVolumeRow

	// Panel 2 — SLA Compliance Report.
	SLAMetPct    string
	SLABreachPct string
	SLAAtRisk    int64
	SLARows      []SupportSLARow

	// Panel 3 — Resolution Time Report.
	ResolutionRows []SupportResolutionRow
	ResThisLabel   string
	ResThisMonth   string
	ResPrevLabel   string
	ResPrevMonth   string
	ResChange      string

	// Panel 4 — KB Usage Report (sebagian besar dilewatkan).
	KBRows []SupportKBRow

	// Panel 5 — Agent Performance Report.
	AgentRows []SupportAgentRow
}

// ReportsSupportBody merender header + 3 KPI + 5 panel. Mobile-first: KPI
// grid-cols-1 (dasar) → sm:grid-cols-3; panel ditumpuk grid-cols-1 →
// lg:grid-cols-2; Agent Performance selebar penuh (tabel banyak kolom).
func ReportsSupportBody(v ReportsSupportView) g.Node {
	return h.Div(h.Class("grid gap-4 min-w-0"),
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text("Support Report")),
			h.P(h.Class("text-base-content/70"), g.Text("Volume tiket, SLA, waktu penyelesaian, KB, & kinerja agen — panel lain menyusul saat datanya tersedia.")),
		),
		reportsSupportFilterForm(v),
		reportsSupportKPICards(v),
		h.Div(h.Class("grid grid-cols-1 lg:grid-cols-2 gap-4 min-w-0"),
			reportsVolumePanel(v),
			reportsSLAPanel(v),
			reportsResolutionPanel(v),
			reportsKBPanel(v),
		),
		reportsAgentPanel(v),
	)
}

// reportsSupportFilterForm = baris filter Periode + Prioritas (BL-51). Form GET
// native (bookmarkable, lolos CSP gotcha #16) → submit re-render seluruh halaman
// tersaring. Kedua dropdown selalu dirender (enum tetap). Input tanggal Kustom
// selalu tampil (tanpa JS toggle, CSP-safe) tapi hanya berdampak saat
// Periode=Kustom (handler). Catatan snapshot memperjelas Periode tak menyentuh
// baris "bulan ini vs lalu" & panel KB. Mobile-first: flex-wrap (nol overflow
// 375px), tiap kontrol text-base (≥16px → iOS tak auto-zoom) + min-h-11 (tap ≥44px).
func reportsSupportFilterForm(v ReportsSupportView) g.Node {
	f := v.Filter
	return h.Div(h.Class("grid gap-1 min-w-0"),
		h.Form(
			h.Method("get"), h.Action(v.Base+"/reports/support"),
			h.Class("flex flex-wrap items-end gap-3 min-w-0"),
			salesFilterSelect("period", "Periode", f.Periods),
			salesFilterSelect("priority", "Prioritas", f.Priorities),
			salesFilterDate("start", "Dari (Kustom)", f.CustomStart),
			salesFilterDate("end", "Sampai (Kustom)", f.CustomEnd),
			h.Div(h.Class("flex flex-wrap items-end gap-2"),
				h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text("Terapkan")),
				g.If(f.QueryString != "", h.A(
					h.Href(v.Base+"/reports/support"),
					h.Class("btn btn-ghost min-h-11"), g.Text("Reset"),
				)),
			),
		),
		h.P(h.Class("text-xs text-base-content/60"),
			g.Text("Periode menyaring Volume (masuk/selesai), SLA, Resolusi, & Agen; baris \"bulan ini vs lalu\" dan panel KB menampilkan kondisi terkini.")),
	)
}

func reportsSupportKPICards(v ReportsSupportView) g.Node {
	return h.Div(h.Class("grid grid-cols-1 sm:grid-cols-3 gap-3"),
		dashboardKPICard("Total Tiket", v.TotalTickets, "text-primary"),
		dashboardKPICard("Kepatuhan SLA", v.SLACompliance, "text-success"),
		dashboardKPICard("Rata Penyelesaian", v.AvgResolution, "text-secondary"),
	)
}

// reportSupportPanelCard = pembungkus kartu panel seragam Support: judul +
// tautan Export CSV per-panel (panelKey → ?panel=…, ditempel filterQS BL-51 agar
// CSV tersaring identik) + isi. min-w-0 agar card menyusut (bukan meluber) di
// grid mobile. Pola sama reportCSPanelCard.
func reportSupportPanelCard(base, filterQS, title, panelKey string, body ...g.Node) g.Node {
	href := base + "/reports/support/export?panel=" + panelKey
	if filterQS != "" {
		href += "&" + filterQS
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(h.Class("card-body min-w-0"),
			h.Div(h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
				h.H2(h.Class("font-semibold"), g.Text(title)),
				h.A(
					h.Href(href),
					h.Class("btn btn-outline btn-sm min-h-11"),
					g.Text("Export CSV"),
				),
			),
			g.Group(body),
		),
	)
}
