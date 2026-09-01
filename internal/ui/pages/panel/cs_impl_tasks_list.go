package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// cs_impl_tasks_list.go — view daftar Implementation Tracker (Modul 6
// Customer Success, sub-item Onboarding 6.2.1.1). Murni-data: semua nilai
// sudah diputuskan handler; view tak memanggil authz/session. Keyset
// pagination (?after=), filter tab (?tab=), KPI cards. Meniru
// engagements_list.go.
//
// F3 ownership (scope_all/is_own) diputuskan di handler melalui
// CSImplTasksListFilterFor (ownership_cs_onboarding.go); view tak tahu cakupan.

// CSImplTaskKPIs = agregat KPI header halaman.
type CSImplTaskKPIs struct {
	Total      int
	ToDo       int
	InProgress int
	Done       int
	Blocked    int
}

// CSImplTaskRow = satu baris daftar task. Nilai sudah diformat handler.
type CSImplTaskRow struct {
	ID          int64
	AccountName string
	TaskName    string
	StatusLabel string
	StatusBadge string
	OwnerName   string
	DueDate     string // formatted date atau "—"
	HrefDetail  string // href ke /impl-tasks/{id} — reserved
}

// CSImplTaskAccountOption = satu opsi dropdown desa untuk form task.
type CSImplTaskAccountOption struct {
	ID   int64
	Name string
}

// CSImplTaskMemberOption = satu opsi dropdown anggota untuk field owner.
type CSImplTaskMemberOption struct {
	ID   int64
	Name string
}

type csImplTaskTabDef struct {
	key   string
	label string
}

// csImplTaskTabs = urutan tab filter; cermin switch handler.
var csImplTaskTabs = []csImplTaskTabDef{
	{"", "Semua"},
	{"to_do", "To Do"},
	{"in_progress", "In Progress"},
	{"done", "Done"},
	{"blocked", "Blocked"},
}

// CSImplTasksListView = data halaman /impl-tasks.
type CSImplTasksListView struct {
	Base       string
	KPIs       CSImplTaskKPIs
	Items      []CSImplTaskRow
	Tab        string
	CanWrite   bool
	NextCursor string
	After      string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail      string // BL-7: jejak cursor halaman sebelumnya (?trail=)
	Err        string
	Msg        string
}

// CSImplTasksList merender body halaman /impl-tasks. Dipanggil via
// renderWorkspaceShell — tidak membungkus AppShell sendiri.
func CSImplTasksList(v CSImplTasksListView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Implementation Tracker")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Checklist task onboarding/implementasi per desa — pantau progres setup awal.")),
			),
			ui.When(v.CanWrite, h.A(
				h.Href(v.Base+"/impl-tasks/new"), h.Class("btn btn-primary min-h-11"),
				g.Text("+ Task Baru"),
			)),
		),
		csImplTaskKPICards(v.KPIs, v.Base),
		csImplTaskTabsNav(v),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "cs-impl-tasks-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "cs-impl-tasks-ok", g.Text(v.Msg)))
	}
	if len(v.Items) == 0 {
		body = append(body, emptyCSImplTasks(v))
	} else {
		body = append(body, csImplTasksTable(v))
		body = append(body, csImplTasksPager(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// csImplTaskKPICards = 5 kartu metrik atas halaman (mobile: 2 kolom, md: 5 kolom).
func csImplTaskKPICards(k CSImplTaskKPIs, base string) g.Node {
	return h.Div(
		h.Class("grid grid-cols-2 md:grid-cols-5 gap-3 min-w-0"),
		csImplTaskKPICard("Total Task", strconv.Itoa(k.Total), base+"/impl-tasks", ""),
		csImplTaskKPICard("To Do", strconv.Itoa(k.ToDo), base+"/impl-tasks?tab=to_do", ""),
		csImplTaskKPICard("In Progress", strconv.Itoa(k.InProgress), base+"/impl-tasks?tab=in_progress", "text-info"),
		csImplTaskKPICard("Done", strconv.Itoa(k.Done), base+"/impl-tasks?tab=done", "text-success"),
		csImplTaskKPICard("Blocked", strconv.Itoa(k.Blocked), base+"/impl-tasks?tab=blocked", "text-error"),
	)
}

func csImplTaskKPICard(label, value, href, colorCls string) g.Node {
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

// csImplTaskTabsNav = tab sebagai LINK <a> berparam (navigasi bookmarkable).
func csImplTaskTabsNav(v CSImplTasksListView) g.Node {
	tabs := make([]g.Node, 0, len(csImplTaskTabs))
	for _, t := range csImplTaskTabs {
		href := v.Base + "/impl-tasks"
		if t.key != "" {
			href += "?tab=" + t.key
		}
		cls := "tab"
		if t.key == v.Tab {
			cls += " tab-active font-medium"
		}
		tabs = append(tabs, h.A(h.Href(href), h.Class(cls+" min-h-11"), g.Text(t.label)))
	}
	return h.Div(h.Role("tablist"), h.Class("tabs tabs-bordered flex-wrap"), g.Group(tabs))
}

func emptyCSImplTasks(v CSImplTasksListView) g.Node {
	if v.NextCursor != "" {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body items-start"),
				h.P(h.Class("text-base-content/70"),
					g.Text("Tidak ada task pada tampilan ini.")),
				h.A(h.Href(v.Base+"/impl-tasks"), h.Class("btn btn-ghost btn-sm min-h-11"),
					g.Text("« Kembali ke awal")),
			),
		)
	}
	msg := "Belum ada task implementasi di ruang kerja ini."
	if v.Tab != "" {
		msg = "Tidak ada task dengan status ini."
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body"),
			h.P(h.Class("text-base-content/70"), g.Text(msg))),
	)
}

// csImplTasksTable = tabel daftar task, dibungkus ui.TableScroll. Kolom Aksi
// hanya muncul bila canWrite=true.
func csImplTasksTable(v CSImplTasksListView) g.Node {
	head := []g.Node{
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Desa")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Task")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Owner")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Due Date")),
	}
	if v.CanWrite {
		head = append(head, h.Th(h.Class("py-2 font-medium"), g.Text("Aksi")))
	}
	rows := make([]g.Node, 0, len(v.Items))
	for _, row := range v.Items {
		rows = append(rows, csImplTaskTableRow(v.Base, row, v.CanWrite))
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

func csImplTaskTableRow(base string, r CSImplTaskRow, canWrite bool) g.Node {
	id := strconv.FormatInt(r.ID, 10)
	cells := []g.Node{
		h.Td(h.Class("py-2 pr-4 max-w-[120px]"), h.Span(h.Class("block truncate"), g.Text(orDash(r.AccountName)))),
		h.Td(h.Class("py-2 pr-4 max-w-[220px]"), h.Span(h.Class("block truncate"), g.Text(orDash(r.TaskName)))),
		h.Td(h.Class("py-2 pr-4"), h.Span(h.Class("badge badge-sm "+r.StatusBadge), g.Text(r.StatusLabel))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(r.OwnerName))),
		h.Td(h.Class("py-2 pr-4 text-sm text-base-content/70"), g.Text(r.DueDate)),
	}
	if canWrite {
		cells = append(cells, h.Td(h.Class("py-2"), csImplTaskStatusForm(base, id, r.StatusLabel)))
	}
	return h.Tr(h.Class("border-b border-base-300/50 hover:bg-base-200/50"), g.Group(cells))
}

// csImplTaskStatusForm = form mini ganti status per baris (canWrite).
// Transisi: to_do → in_progress; in_progress → done/blocked; blocked →
// in_progress; done → dibiarkan (sudah final, tapi tetap bisa dibuka ke
// in_progress bila salah tandai).
func csImplTaskStatusForm(base, id, currentStatusLabel string) g.Node {
	nodes := []g.Node{}
	switch currentStatusLabel {
	case "To Do":
		nodes = append(nodes, csImplTaskActionBtn(base, id, "in_progress", "▶ Mulai", "btn-info"))
	case "In Progress":
		nodes = append(nodes,
			csImplTaskActionBtn(base, id, "done", "✓ Selesai", "btn-success"),
			csImplTaskActionBtn(base, id, "blocked", "Blocked", "text-error btn-ghost"),
		)
	case "Blocked":
		nodes = append(nodes, csImplTaskActionBtn(base, id, "in_progress", "▶ Lanjutkan", "btn-info"))
	default: // Done → bisa dibuka ulang
		nodes = append(nodes, csImplTaskActionBtn(base, id, "in_progress", "Buka Ulang", "btn-ghost"))
	}
	return h.Div(h.Class("flex flex-wrap items-center gap-1"), g.Group(nodes))
}

func csImplTaskActionBtn(base, id, targetStatus, label, extraCls string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/impl-tasks/"+id+"/status"),
		h.Input(h.Type("hidden"), h.Name("status"), h.Value(targetStatus)),
		h.Button(h.Type("submit"), h.Class("btn btn-xs min-h-11 "+extraCls),
			g.Text(label)),
	)
}

func csImplTasksPager(v CSImplTasksListView) g.Node {
	base := panelListHref(v.Base+"/impl-tasks", [2]string{"tab", v.Tab})
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}
