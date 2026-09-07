package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// cs_journey_list.go — view Customer Journey / Lifecycle (CRM Modul 6, 6.2 —
// BL-77). Dashboard portofolio fase perjalanan desa. Murni-data: semua nilai
// (badge, %, hari-di-fase, peringatan) sudah diputuskan handler; view tak
// memanggil authz/session. Keyset pagination ASC (?after=), filter fase
// (?stage=), 4 KPI + funnel fase + onboarding aktif + tabel utama.
//
// F3 ownership diputuskan di handler lewat CSJourneyListFilterFor
// (ownership_cs_journey.go); view tak tahu cakupan.

// CSJourneyKPIs = agregat 4 kartu KPI header.
type CSJourneyKPIs struct {
	OnboardingCount   int
	OnboardingAvgDays int
	AdoptionCount     int
	AdoptionMaxDays   int
	RetentionCount    int
	RenewalCount      int
}

// CSJourneyPhaseRow = satu baris funnel "Fase Perjalanan Desa" (5 fase kanonik,
// sudah dilengkapi handler termasuk fase kosong).
type CSJourneyPhaseRow struct {
	Stage   string // label fase (Onboarding..Advocacy)
	Count   int
	AvgDays int
	Stalled int
	Pct     int // lebar bar relatif count maksimum antar-fase
}

// CSJourneyOnboardRow = satu baris panel "Onboarding Aktif".
type CSJourneyOnboardRow struct {
	AccountName  string
	Progress     int
	StatusLabel  string
	StatusBadge  string
	TargetGoLive string // tanggal terformat atau "—"
	TargetPassed bool   // target lampau → ⚠️
	CSMName      string
	HrefDetail   string
}

// CSJourneyAccountRow = satu baris tabel utama "Desa Binaan — Posisi Lifecycle".
type CSJourneyAccountRow struct {
	AccountName        string
	StageLabel         string
	StageBadge         string
	StageEntry         string // tanggal masuk fase terformat atau "—"
	DaysInStage        int
	DaysKnown          bool // stage_entry_date ada → tampilkan lama-di-fase
	Stalled            bool // lama-di-fase > ambang → ⚠️
	HealthScore        string
	HealthBadge        string
	Progress           int
	HasProgress        bool
	OnboardStatusLabel string
	OnboardStatusBadge string
	CSMName            string
	HrefDetail         string
}

// CSJourneyListView = data halaman /journey.
type CSJourneyListView struct {
	Base         string
	KPIs         CSJourneyKPIs
	Phases       []CSJourneyPhaseRow
	Onboarding   []CSJourneyOnboardRow
	Items        []CSJourneyAccountRow
	StageOptions []string // opsi dropdown fase (lifecycleStageOptions)
	FilterStage  string   // fase terpilih ("" = semua)
	CanWrite     bool
	NextCursor   string
	After        string
	Trail        string
	Err          string
	Msg          string
}

// CSJourneyList merender body halaman /journey. Dipanggil via
// renderWorkspaceShell — tidak membungkus AppShell sendiri.
func CSJourneyList(v CSJourneyListView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Customer Journey")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Posisi tiap desa di sepanjang siklus hidup — Onboarding hingga Advocacy.")),
			),
			ui.When(v.CanWrite, h.A(
				h.Href(v.Base+"/accounts"), h.Class("btn btn-primary min-h-11"),
				g.Text("+ Mulai Onboarding"),
			)),
		),
		csJourneyKPICards(v.KPIs),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "cs-journey-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "cs-journey-ok", g.Text(v.Msg)))
	}
	body = append(body, csJourneyPhasesCard(v))
	if len(v.Onboarding) > 0 {
		body = append(body, csJourneyOnboardingCard(v))
	}
	body = append(body, csJourneyAccountsCard(v))
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// csJourneyKPICards = 4 kartu metrik atas (mobile: 2 kolom, md: 4 kolom).
func csJourneyKPICards(k CSJourneyKPIs) g.Node {
	return h.Div(
		h.Class("grid grid-cols-2 md:grid-cols-4 gap-3 min-w-0"),
		csJourneyKPICard("Onboarding", strconv.Itoa(k.OnboardingCount),
			"rata "+strconv.Itoa(k.OnboardingAvgDays)+" hari di fase", "text-info"),
		csJourneyKPICard("Adoption", strconv.Itoa(k.AdoptionCount),
			"terlama "+strconv.Itoa(k.AdoptionMaxDays)+" hari", "text-primary"),
		csJourneyKPICard("Retention", strconv.Itoa(k.RetentionCount), "desa aktif", "text-success"),
		csJourneyKPICard("Menuju Renewal", strconv.Itoa(k.RenewalCount), "fase Renewal", "text-warning"),
	)
}

func csJourneyKPICard(label, value, sub, colorCls string) g.Node {
	numCls := "text-2xl font-bold"
	if colorCls != "" {
		numCls += " " + colorCls
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body p-3"),
			h.P(h.Class("text-xs text-base-content/60 truncate"), g.Text(label)),
			h.P(h.Class(numCls), g.Text(value)),
			h.P(h.Class("text-xs text-base-content/50 truncate"), g.Text(sub)),
		),
	)
}

// csJourneyPhasesCard = funnel "Fase Perjalanan Desa": bar lebar-CSS per fase +
// jumlah, rata hari, dan jumlah macet. Bar = div width (CSP-safe, tanpa JS).
func csJourneyPhasesCard(v CSJourneyListView) g.Node {
	rows := make([]g.Node, 0, len(v.Phases))
	for _, p := range v.Phases {
		rows = append(rows, csJourneyPhaseRow(p))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("card-title text-base"), g.Text("Fase Perjalanan Desa")),
			h.Div(h.Class("grid gap-3"), g.Group(rows)),
		),
	)
}

func csJourneyPhaseRow(p CSJourneyPhaseRow) g.Node {
	stalled := g.Node(nil)
	if p.Stalled > 0 {
		stalled = h.Span(h.Class("text-error"), g.Text("⚠ "+strconv.Itoa(p.Stalled)+" macet"))
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		h.Div(
			h.Class("flex flex-wrap items-baseline justify-between gap-2"),
			h.Span(h.Class("font-medium"), g.Text(p.Stage)),
			h.Div(
				h.Class("flex flex-wrap items-center gap-3 text-sm text-base-content/70"),
				h.Span(g.Text(strconv.Itoa(p.Count)+" desa")),
				h.Span(g.Text("rata "+strconv.Itoa(p.AvgDays)+" hari")),
				stalled,
			),
		),
		reportBar(p.Pct),
	)
}

// csJourneyOnboardingCard = tabel "Onboarding Aktif" (onboarding In Progress),
// dibungkus ui.TableScroll.
func csJourneyOnboardingCard(v CSJourneyListView) g.Node {
	rows := make([]g.Node, 0, len(v.Onboarding))
	for _, o := range v.Onboarding {
		rows = append(rows, csJourneyOnboardRow(o))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			h.H2(h.Class("card-title text-base"), g.Text("Onboarding Aktif")),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Desa")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Progress")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Target Go-Live")),
					h.Th(h.Class("py-2 font-medium"), g.Text("CSM")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func csJourneyOnboardRow(o CSJourneyOnboardRow) g.Node {
	target := g.Node(h.Span(g.Text(o.TargetGoLive)))
	if o.TargetPassed {
		target = h.Span(h.Class("text-error"), g.Text("⚠ "+o.TargetGoLive))
	}
	return h.Tr(
		h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
		h.Td(h.Class("py-2 pr-4 max-w-[160px]"),
			h.A(h.Href(o.HrefDetail), h.Class("link block truncate"), g.Text(orDash(o.AccountName)))),
		h.Td(h.Class("py-2 pr-4 min-w-[120px]"), csJourneyProgress(o.Progress)),
		h.Td(h.Class("py-2 pr-4"), h.Span(h.Class("badge badge-sm "+o.StatusBadge), g.Text(o.StatusLabel))),
		h.Td(h.Class("py-2 pr-4 text-sm"), target),
		h.Td(h.Class("py-2 text-sm text-base-content/70"), g.Text(orDash(o.CSMName))),
	)
}

// csJourneyAccountsCard = tabel utama "Desa Binaan — Posisi Lifecycle" +
// filter fase + pager + tombol ekspor CSV.
func csJourneyAccountsCard(v CSJourneyListView) g.Node {
	inner := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2"),
			h.H2(h.Class("card-title text-base"), g.Text("Desa Binaan — Posisi Lifecycle")),
			h.Div(
				h.Class("flex flex-wrap items-center gap-2"),
				csJourneyStageFilter(v),
				h.A(
					h.Href(csJourneyExportHref(v)),
					h.Class("btn btn-ghost btn-sm min-h-11"),
					g.Text("Ekspor CSV"),
				),
			),
		),
	}
	if len(v.Items) == 0 {
		inner = append(inner, csJourneyEmpty(v))
	} else {
		inner = append(inner, csJourneyTable(v), csJourneyPager(v))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(h.Class("card-body min-w-0 grid gap-3"), g.Group(inner)),
	)
}

// csJourneyStageFilter = dropdown "Fase" sebagai form GET native (CSP-safe,
// tanpa JS). Submit mengatur ulang ke halaman pertama (tanpa cursor).
func csJourneyStageFilter(v CSJourneyListView) g.Node {
	opts := make([]g.Node, 0, len(v.StageOptions)+1)
	opts = append(opts, csJourneyStageOption("", "Fase: Semua", v.FilterStage))
	for _, s := range v.StageOptions {
		opts = append(opts, csJourneyStageOption(s, s, v.FilterStage))
	}
	return h.FormEl(
		h.Method("get"), h.Action(v.Base+"/journey"),
		h.Class("flex items-center gap-1"),
		h.Select(
			h.Name("stage"), h.Class("select select-sm select-bordered min-h-11"),
			g.Group(opts),
		),
		h.Button(h.Type("submit"), h.Class("btn btn-sm min-h-11"), g.Text("Terapkan")),
	)
}

func csJourneyStageOption(value, label, selected string) g.Node {
	attrs := []g.Node{h.Value(value), g.Text(label)}
	if value == selected {
		attrs = append(attrs, h.Selected())
	}
	return h.Option(attrs...)
}

func csJourneyEmpty(v CSJourneyListView) g.Node {
	if v.NextCursor != "" || v.After != "" {
		return h.Div(
			h.P(h.Class("text-base-content/70"), g.Text("Tidak ada desa pada tampilan ini.")),
			h.A(h.Href(v.Base+"/journey"), h.Class("btn btn-ghost btn-sm min-h-11"),
				g.Text("« Kembali ke awal")),
		)
	}
	msg := "Belum ada data siklus hidup desa di ruang kerja ini."
	if v.FilterStage != "" {
		msg = "Tidak ada desa pada fase ini."
	}
	return h.P(h.Class("text-base-content/70"), g.Text(msg))
}

func csJourneyTable(v CSJourneyListView) g.Node {
	rows := make([]g.Node, 0, len(v.Items))
	for _, r := range v.Items {
		rows = append(rows, csJourneyTableRow(r))
	}
	return ui.TableScroll(h.Table(
		h.Class("w-full text-sm"),
		h.THead(h.Tr(
			h.Class("border-b border-base-300 text-left text-base-content/70"),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Desa")),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Fase Saat Ini")),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Masuk Fase")),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Lama di Fase")),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Health")),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Onboarding")),
			h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("CSM")),
			h.Th(h.Class("py-2 font-medium"), g.Text("Aksi")),
		)),
		h.TBody(g.Group(rows)),
	))
}

func csJourneyTableRow(r CSJourneyAccountRow) g.Node {
	days := g.Node(h.Span(g.Text("—")))
	if r.DaysKnown {
		cls := ""
		if r.Stalled {
			cls = "text-error font-medium"
		}
		label := strconv.Itoa(r.DaysInStage) + " hari"
		if r.Stalled {
			label = "⚠ " + label
		}
		days = h.Span(h.Class(cls), g.Text(label))
	}
	health := g.Node(h.Span(g.Text("—")))
	if r.HealthScore != "" {
		health = h.Span(h.Class("badge badge-sm "+r.HealthBadge), g.Text(r.HealthScore))
	}
	onboarding := g.Node(h.Span(h.Class("text-base-content/60"), g.Text("—")))
	if r.OnboardStatusLabel != "" {
		onboarding = h.Div(h.Class("flex flex-col gap-1 min-w-[120px]"),
			csJourneyProgress(r.Progress),
			h.Span(h.Class("badge badge-xs "+r.OnboardStatusBadge), g.Text(r.OnboardStatusLabel)),
		)
	}
	return h.Tr(
		h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
		h.Td(h.Class("py-2 pr-4 max-w-[160px]"),
			h.Span(h.Class("block truncate"), g.Text(orDash(r.AccountName)))),
		h.Td(h.Class("py-2 pr-4"), h.Span(h.Class("badge badge-sm "+r.StageBadge), g.Text(r.StageLabel))),
		h.Td(h.Class("py-2 pr-4 text-sm text-base-content/70"), g.Text(r.StageEntry)),
		h.Td(h.Class("py-2 pr-4 text-sm"), days),
		h.Td(h.Class("py-2 pr-4"), health),
		h.Td(h.Class("py-2 pr-4"), onboarding),
		h.Td(h.Class("py-2 pr-4 text-sm text-base-content/70"), g.Text(orDash(r.CSMName))),
		h.Td(h.Class("py-2"),
			h.A(h.Href(r.HrefDetail), h.Class("btn btn-ghost btn-xs min-h-11"), g.Text("Detail"))),
	)
}

// csJourneyProgress = bar progress onboarding + persentase.
func csJourneyProgress(pct int) g.Node {
	return h.Div(
		h.Class("flex items-center gap-2 min-w-0"),
		h.Div(h.Class("grow"), reportBar(pct)),
		h.Span(h.Class("text-xs text-base-content/70 shrink-0"), g.Text(strconv.Itoa(pct)+"%")),
	)
}

func csJourneyPager(v CSJourneyListView) g.Node {
	base := panelListHref(v.Base+"/journey", [2]string{"stage", v.FilterStage})
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}

// csJourneyExportHref = URL ekspor CSV yang mempertahankan filter fase.
func csJourneyExportHref(v CSJourneyListView) string {
	return panelListHref(v.Base+"/journey/export", [2]string{"stage", v.FilterStage})
}
