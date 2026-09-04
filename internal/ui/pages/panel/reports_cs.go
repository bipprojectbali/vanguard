package panel

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// reports_cs.go — Customer Success Report (Modul 8, wireframe 8.2, BL-45): view
// MURNI-DATA; handler (internal/handler/reports_cs*.go) yang menghitung, memformat
// (persen/koma Indonesia), & memasking Rp (maskARR pada Nilai Hilang). Struktur
// data 6 panel di sini; fungsi render tiap panel di reports_cs_panels.go (satu
// file di bawah ambang, pola sama reports_sales). Kartu KPI meniru dashboardKPICard
// (dashboard.go, package sama); tiap <table> dibungkus ui.TableScroll (WAJIB,
// gotcha overflow). Panel yang datanya belum ada (NPS/CSAT dll) TAK dirender —
// keputusan sadar BL-45, lihat doc comment handler.

// ── Struktur data 6 panel (semua string SUDAH diformat & di-mask di handler) ──

// ReportBandRow = satu baris distribusi band (dipakai Health, Adoption,
// Onboarding — bentuk sama: Label · Count · Porsi% · bar). Pct & BarPct relatif
// denominator berskor (distribusi, bukan nilai absolut).
type ReportBandRow struct {
	Label  string
	Count  int64
	Pct    string
	BarPct int
}

// CSChurnRow = satu alasan churn (panel 3 tabel). Porsi "% dari total churn";
// LostValue = Rupiah SUDAH di-mask F4 (Support lihat penanda tersembunyi).
type CSChurnRow struct {
	Reason    string
	Count     int64
	Porsi     string
	LostValue string
}

// CSComplianceRow = satu tipe engagement (panel 6 kepatuhan). TouchPoint
// "done / total"; Compliance "%"; BarPct kepatuhan tipe itu.
type CSComplianceRow struct {
	Type       string
	TouchPoint string
	Compliance string
	BarPct     int
}

// CSEngagementCSMRow = satu baris per-CSM (panel 6 tabel). Villages = Desa
// Dipegang (accounts.assigned_csm = CSM).
type CSEngagementCSMRow struct {
	CSM        string
	Villages   int64
	TouchPoint string
	Compliance string
	BarPct     int
}

// ReportsCSView — data siap-render /reports/customer-success (3 KPI + 6 panel;
// NPS/CSAT dilewatkan). String KPI ("—" bila tak bermakna) & persen dihitung
// handler.
type ReportsCSView struct {
	Base string

	// KPI ringkas.
	AvgHealth     string
	AvgAdoption   string
	RetentionRate string

	// Panel 1 — Health Score Report.
	HealthScored int64
	HealthBands  []ReportBandRow

	// Panel 2 — Adoption Report (sederhana).
	AdoptionScored int64
	AdoptionBands  []ReportBandRow

	// Panel 3 — Retention/Churn Report.
	RetentionPct string
	ChurnPct     string
	ActiveCount  int64
	ChurnedCount int64
	ChurnReasons []CSChurnRow

	// Panel 5 — Onboarding Report (panel 4 NPS/CSAT dilewatkan).
	OnboardAvgDuration string
	OnboardCompleted   int64
	OnboardLate        int64
	OnboardStatus      []ReportBandRow

	// Panel 6 — Engagement Report.
	EngagementTypes []CSComplianceRow
	EngagementCSMs  []CSEngagementCSMRow
}

// ReportsCSBody merender header + 3 KPI + 6 panel. Mobile-first: KPI grid-cols-1
// (dasar) → sm:grid-cols-3; panel ditumpuk grid-cols-1 → lg:grid-cols-2.
func ReportsCSBody(v ReportsCSView) g.Node {
	return h.Div(h.Class("grid gap-4 min-w-0"),
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text("Customer Success Report")),
			h.P(h.Class("text-base-content/70"), g.Text("Health, adopsi, retensi/churn, onboarding, & engagement — panel lain menyusul saat datanya tersedia.")),
		),
		reportsCSKPICards(v),
		h.Div(h.Class("grid grid-cols-1 lg:grid-cols-2 gap-4 min-w-0"),
			reportsHealthPanel(v),
			reportsAdoptionPanel(v),
			reportsRetentionPanel(v),
			reportsOnboardingPanel(v),
		),
		reportsEngagementPanel(v),
	)
}

func reportsCSKPICards(v ReportsCSView) g.Node {
	return h.Div(h.Class("grid grid-cols-1 sm:grid-cols-3 gap-3"),
		dashboardKPICard("Rata Health Score", v.AvgHealth, "text-primary"),
		dashboardKPICard("Adoption Rate", v.AvgAdoption, "text-secondary"),
		dashboardKPICard("Retention Rate", v.RetentionRate, "text-success"),
	)
}

// reportCSPanelCard = pembungkus kartu panel seragam CS: judul + tautan Export
// CSV per-panel (panelKey → ?panel=…) + isi. Filter interaktif ditunda BL lain
// (tak ada QueryString). min-w-0 agar card menyusut (bukan meluber) di grid mobile.
func reportCSPanelCard(base, title, panelKey string, body ...g.Node) g.Node {
	href := base + "/reports/customer-success/export?panel=" + panelKey
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

// reportBandTable = tabel distribusi band seragam (Health/Adoption/Onboarding).
// labelHead menamai kolom pertama (mis. "Band"/"Status"). Dibungkus ui.TableScroll.
func reportBandTable(labelHead string, bands []ReportBandRow) g.Node {
	rows := make([]g.Node, 0, len(bands))
	for _, b := range bands {
		rows = append(rows, h.Tr(
			h.Class("border-b border-base-300/50"),
			rTd("font-medium break-words", g.Text(b.Label)),
			rTd("", g.Text(strconv.FormatInt(b.Count, 10))),
			rTd("", g.Text(b.Pct)),
			h.Td(h.Class("py-2 w-24"), reportBar(b.BarPct)),
		))
	}
	return csTableScroll(
		[]g.Node{rTh(labelHead), rTh("Jumlah Desa"), rTh("Porsi"), rTh("")},
		rows,
	)
}
