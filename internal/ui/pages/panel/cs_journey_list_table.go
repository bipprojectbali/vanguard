package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// cs_journey_list_table.go — filter fase, tabel utama, pager, dan ekspor CSV
// dari /journey (BL-77). Tipe & kartu KPI/funnel/onboarding di cs_journey_list.go.

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
