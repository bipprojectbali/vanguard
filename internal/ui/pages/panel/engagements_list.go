package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// engagements_list.go — view daftar Engagements / Check-ins (Modul 6 Customer
// Success, slice 6.5). Murni-data: semua nilai sudah diputuskan handler; view
// tak memanggil authz/session. Keyset pagination (?after=), filter tab (?tab=),
// KPI cards. Meniru tickets_list.go.
//
// F3 ownership (scope_all/is_own) diputuskan di handler melalui
// EngagementsListFilterFor (ownership.go); view tak tahu cakupan.

// EngagementKPIs = agregat KPI header halaman.
type EngagementKPIs struct {
	Total   int
	Planned int
	Done    int
	Missed  int
	DueSoon int
}

// EngagementRow = satu baris daftar engagement. Nilai sudah diformat handler.
type EngagementRow struct {
	ID          int64
	AccountName string
	Subject     string
	TypeLabel   string // label UI dari kode DB (mis. "QBR", "Touch Point")
	Channel     string // label UI dari kode DB (mis. "WhatsApp", "Call")
	Scheduled   string // formatted datetime
	StatusLabel string // label UI (mis. "Planned", "Done")
	StatusBadge string // daisyUI badge class
	OwnerName   string
	NextDue     string // formatted date atau "—"
	Outcome     string // ringkasan hasil; "" bila belum ada (prefill panel Done, BL-30)
	HrefDetail  string // href ke /engagements/{id} — reserved, v1 pakai untuk status form
}

// EngagementAccountOption = satu opsi dropdown desa untuk form engagement.
type EngagementAccountOption struct {
	ID   int64
	Name string
}

// EngagementMemberOption = satu opsi dropdown anggota untuk field owner.
type EngagementMemberOption struct {
	ID   int64
	Name string
}

// engagementTabDef = satu tab filter.
type engagementTabDef struct {
	key   string
	label string
}

// engagementTabs = urutan tab filter; cermin switch handler.
var engagementTabs = []engagementTabDef{
	{"", "Semua"},
	{"planned", "Planned"},
	{"done", "Done"},
	{"skipped", "Skipped"},
	{"rescheduled", "Rescheduled"},
}

// EngagementsListView = data halaman /engagements.
type EngagementsListView struct {
	Base       string
	KPIs       EngagementKPIs
	Items      []EngagementRow
	Tab        string
	Query      string // ?q= pencarian bebas (BL-6); "" = tak mencari
	CanWrite   bool
	NextCursor string
	After      string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail      string // BL-7: jejak cursor halaman sebelumnya (?trail=)
	Err        string
	Msg        string
}

// EngagementsList merender body halaman /engagements. Dipanggil via
// renderWorkspaceShell — tidak membungkus AppShell sendiri.
func EngagementsList(v EngagementsListView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Engagements")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Touch point dan interaksi CS dengan desa — jadwalkan, catat, dan pantau tindak lanjut.")),
			),
			ui.When(v.CanWrite, h.A(
				h.Href(v.Base+"/engagements/new"), h.Class("btn btn-primary min-h-11"),
				g.Text("+ Engagement Baru"),
			)),
		),
		engagementKPICards(v.KPIs, v.Base),
		tabSearchRow(engagementTabsNav(v),
			searchBox(v.Base+"/engagements", v.Query,
				"Cari engagement — subjek atau desa…", "Cari engagement",
				hiddenField{"tab", v.Tab})),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "engagements-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "engagements-ok", g.Text(v.Msg)))
	}
	if len(v.Items) == 0 {
		body = append(body, emptyEngagements(v))
	} else {
		body = append(body, engagementsTable(v))
		body = append(body, engagementsPager(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// engagementKPICards = 5 kartu metrik atas halaman (mobile: 2 kolom, md: 5 kolom).
func engagementKPICards(k EngagementKPIs, base string) g.Node {
	return h.Div(
		h.Class("grid grid-cols-2 md:grid-cols-5 gap-3 min-w-0"),
		engagementKPICard("Total Engagement", strconv.Itoa(k.Total), base+"/engagements", ""),
		engagementKPICard("Planned", strconv.Itoa(k.Planned), base+"/engagements?tab=planned", "text-info"),
		engagementKPICard("Done", strconv.Itoa(k.Done), base+"/engagements?tab=done", "text-success"),
		engagementKPICard("Skipped/Reschedule", strconv.Itoa(k.Missed), base+"/engagements?tab=skipped", "text-warning"),
		engagementKPICard("Jatuh Tempo 7 Hari", strconv.Itoa(k.DueSoon), base+"/engagements?tab=planned", "text-warning"),
	)
}

func engagementKPICard(label, value, href, colorCls string) g.Node {
	numCls := "text-2xl font-bold"
	if colorCls != "" {
		numCls += " " + colorCls
	}
	return h.A(
		h.Href(href),
		h.Class("card bg-base-100 border border-base-300 hover:bg-base-200/50 transition-colors min-w-0"),
		h.Div(
			h.Class("card-body p-3"),
			h.P(h.Class("text-xs text-base-content/60 truncate"), g.Text(label)),
			h.P(h.Class(numCls), g.Text(value)),
		),
	)
}

// engagementTabsNav = tab sebagai LINK <a> berparam (navigasi bookmarkable).
func engagementTabsNav(v EngagementsListView) g.Node {
	tabs := make([]g.Node, 0, len(engagementTabs))
	for _, t := range engagementTabs {
		href := withQuery(v.Base+"/engagements", v.Query, hiddenField{"tab", t.key})
		cls := "tab"
		if t.key == v.Tab {
			cls += " tab-active font-medium"
		}
		tabs = append(tabs, h.A(h.Href(href), h.Class(cls+" min-h-11"), g.Text(t.label)))
	}
	return h.Div(h.Role("tablist"), h.Class("tabs tabs-bordered flex-wrap"), g.Group(tabs))
}

func emptyEngagements(v EngagementsListView) g.Node {
	if v.Query != "" {
		reset := withQuery(v.Base+"/engagements", "", hiddenField{"tab", v.Tab})
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body items-start"),
				h.P(h.Class("text-base-content/70"),
					g.Text("Belum ada engagement yang cocok pencarian.")),
				h.A(h.Href(reset), h.Class("btn btn-ghost btn-sm min-h-11"),
					g.Text("« Reset pencarian")),
			),
		)
	}
	if v.NextCursor != "" {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body items-start"),
				h.P(h.Class("text-base-content/70"),
					g.Text("Tidak ada engagement pada tampilan ini.")),
				h.A(h.Href(v.Base+"/engagements"), h.Class("btn btn-ghost btn-sm min-h-11"),
					g.Text("« Kembali ke awal")),
			),
		)
	}
	msg := "Belum ada engagement di ruang kerja ini."
	if v.Tab != "" {
		msg = "Tidak ada engagement dengan status ini."
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body"),
			h.P(h.Class("text-base-content/70"), g.Text(msg))),
	)
}

// engagementsTable = tabel daftar engagement, dibungkus ui.TableScroll.
// Kolom Aksi hanya muncul bila canWrite=true.
func engagementsTable(v EngagementsListView) g.Node {
	head := []g.Node{
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Desa")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Subjek")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Tipe")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Channel")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Jadwal")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Owner")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Next Due")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Outcome")),
	}
	if v.CanWrite {
		head = append(head, h.Th(h.Class("py-2 font-medium"), g.Text("Aksi")))
	}
	rows := make([]g.Node, 0, len(v.Items))
	for _, row := range v.Items {
		rows = append(rows, engagementTableRow(v.Base, row, v.CanWrite))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					g.Group(head),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func engagementTableRow(base string, r EngagementRow, canWrite bool) g.Node {
	id := strconv.FormatInt(r.ID, 10)
	cells := []g.Node{
		h.Td(h.Class("py-2 pr-4 max-w-[120px]"), h.Span(h.Class("block truncate"), g.Text(orDash(r.AccountName)))),
		h.Td(h.Class("py-2 pr-4 max-w-[200px]"), h.Span(h.Class("block truncate"), g.Text(orDash(r.Subject)))),
		h.Td(h.Class("py-2 pr-4 whitespace-nowrap"), g.Text(orDash(r.TypeLabel))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(r.Channel))),
		h.Td(h.Class("py-2 pr-4 whitespace-nowrap text-sm text-base-content/70"), g.Text(r.Scheduled)),
		h.Td(h.Class("py-2 pr-4"), h.Span(h.Class("badge badge-sm "+r.StatusBadge), g.Text(r.StatusLabel))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(r.OwnerName))),
		h.Td(h.Class("py-2 pr-4 text-sm text-base-content/70"), g.Text(r.NextDue)),
		h.Td(h.Class("py-2 pr-4 max-w-[160px]"),
			h.Span(h.Class("block truncate text-base-content/70"), g.Text(orDash(r.Outcome)))),
	}
	if canWrite {
		cells = append(cells, h.Td(h.Class("py-2"), engagementStatusForm(base, id, r.StatusLabel, r.Outcome)))
	}
	return h.Tr(h.Class("border-b border-base-300/50 hover:bg-base-200/50"), g.Group(cells))
}

// engagementStatusForm = aksi ganti status per baris (canWrite). Transisi:
// planned → done/skipped/rescheduled; done/skipped/rescheduled → planned.
//
// BL-30: "✓ Done" & "Reschedule" TAK langsung submit — mereka membuka panel
// inline (Datastar data-show, state form efemeral; pola BL-19/BL-26/BL-28
// showWhen, BUKAN mekanisme baru) untuk menampung outcome (Done) atau jadwal-
// baru (Reschedule). "Skip"/"Plan Ulang" tetap submit langsung — hanya kirim
// status; query COALESCE menjaga outcome/next_due/scheduled lama agar tak jadi
func engagementsPager(v EngagementsListView) g.Node {
	base := panelListHref(v.Base+"/engagements", [2]string{"tab", v.Tab}, [2]string{"q", v.Query})
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}
