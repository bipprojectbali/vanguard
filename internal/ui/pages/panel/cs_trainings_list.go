package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// cs_trainings_list.go — view daftar Training Schedule (Modul 6 Customer
// Success, sub-item Onboarding 6.2.1.2). Murni-data: semua nilai sudah
// diputuskan handler; view tak memanggil authz/session. Keyset pagination
// (?after=), filter tab (?tab=), KPI cards. Meniru cs_impl_tasks_list.go /
// engagements_list.go.
//
// F3 ownership (scope_all/is_own) diputuskan di handler melalui
// CSTrainingsListFilterFor (ownership_cs_onboarding.go); view tak tahu cakupan.

// CSTrainingKPIs = agregat KPI header halaman.
type CSTrainingKPIs struct {
	Total       int
	Scheduled   int
	Completed   int
	Rescheduled int
	Cancelled   int
}

// CSTrainingRow = satu baris daftar training. Nilai sudah diformat handler.
type CSTrainingRow struct {
	ID            int64
	AccountName   string
	TrainingTopic string
	StatusLabel   string
	StatusBadge   string
	TrainerName   string
	TrainingDate  string // formatted tanggal+jam
	Participants  string // "—" bila NULL
	Attendance    string // "—" bila NULL
	Notes         string // catatan/kesimpulan; "" bila belum ada (BL-28 #3)
	HrefDetail    string // href ke /trainings/{id} — reserved
}

// CSTrainingAccountOption = satu opsi dropdown desa untuk form training.
type CSTrainingAccountOption struct {
	ID   int64
	Name string
}

// CSTrainingTrainerOption = satu opsi dropdown trainer (anggota) untuk form.
type CSTrainingTrainerOption struct {
	ID   int64
	Name string
}

type csTrainingTabDef struct {
	key   string
	label string
}

// csTrainingTabs = urutan tab filter; cermin switch handler.
var csTrainingTabs = []csTrainingTabDef{
	{"", "Semua"},
	{"scheduled", "Scheduled"},
	{"completed", "Completed"},
	{"rescheduled", "Rescheduled"},
	{"cancelled", "Cancelled"},
}

// CSTrainingsListView = data halaman /trainings.
type CSTrainingsListView struct {
	Base       string
	KPIs       CSTrainingKPIs
	Items      []CSTrainingRow
	Tab        string
	Query      string // ?q= pencarian bebas (BL-6); "" = tak mencari
	CanWrite   bool
	NextCursor string
	After      string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail      string // BL-7: jejak cursor halaman sebelumnya (?trail=)
	Err        string
	Msg        string
}

// CSTrainingsList merender body halaman /trainings. Dipanggil via
// renderWorkspaceShell — tidak membungkus AppShell sendiri.
func CSTrainingsList(v CSTrainingsListView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Training Schedule")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Jadwal pelatihan per desa — pantau kehadiran & keterlibatan onboarding.")),
			),
			ui.When(v.CanWrite, h.A(
				h.Href(v.Base+"/trainings/new"), h.Class("btn btn-primary min-h-11"),
				g.Text("+ Jadwalkan Training"),
			)),
		),
		csTrainingKPICards(v.KPIs, v.Base),
		csTrainingTabsNav(v),
		searchBox(v.Base+"/trainings", v.Query,
			"Cari training — topik atau desa…", "Cari training",
			hiddenField{"tab", v.Tab}),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "cs-trainings-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "cs-trainings-ok", g.Text(v.Msg)))
	}
	if len(v.Items) == 0 {
		body = append(body, emptyCSTrainings(v))
	} else {
		body = append(body, csTrainingsTable(v))
		body = append(body, csTrainingsPager(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// csTrainingKPICards = 5 kartu metrik atas halaman (mobile: 2 kolom, md: 5 kolom).
func csTrainingKPICards(k CSTrainingKPIs, base string) g.Node {
	return h.Div(
		h.Class("grid grid-cols-2 md:grid-cols-5 gap-3 min-w-0"),
		csTrainingKPICard("Total Training", strconv.Itoa(k.Total), base+"/trainings", ""),
		csTrainingKPICard("Scheduled", strconv.Itoa(k.Scheduled), base+"/trainings?tab=scheduled", "text-info"),
		csTrainingKPICard("Completed", strconv.Itoa(k.Completed), base+"/trainings?tab=completed", "text-success"),
		csTrainingKPICard("Rescheduled", strconv.Itoa(k.Rescheduled), base+"/trainings?tab=rescheduled", "text-warning"),
		csTrainingKPICard("Cancelled", strconv.Itoa(k.Cancelled), base+"/trainings?tab=cancelled", "text-error"),
	)
}

func csTrainingKPICard(label, value, href, colorCls string) g.Node {
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

// csTrainingTabsNav = tab sebagai LINK <a> berparam (navigasi bookmarkable).
func csTrainingTabsNav(v CSTrainingsListView) g.Node {
	tabs := make([]g.Node, 0, len(csTrainingTabs))
	for _, t := range csTrainingTabs {
		href := withQuery(v.Base+"/trainings", v.Query, hiddenField{"tab", t.key})
		cls := "tab"
		if t.key == v.Tab {
			cls += " tab-active font-medium"
		}
		tabs = append(tabs, h.A(h.Href(href), h.Class(cls+" min-h-11"), g.Text(t.label)))
	}
	return h.Div(h.Role("tablist"), h.Class("tabs tabs-bordered flex-wrap"), g.Group(tabs))
}

func emptyCSTrainings(v CSTrainingsListView) g.Node {
	if v.Query != "" {
		reset := withQuery(v.Base+"/trainings", "", hiddenField{"tab", v.Tab})
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body items-start"),
				h.P(h.Class("text-base-content/70"),
					g.Text("Belum ada training yang cocok pencarian.")),
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
					g.Text("Tidak ada training pada tampilan ini.")),
				h.A(h.Href(v.Base+"/trainings"), h.Class("btn btn-ghost btn-sm min-h-11"),
					g.Text("« Kembali ke awal")),
			),
		)
	}
	msg := "Belum ada jadwal training di ruang kerja ini."
	if v.Tab != "" {
		msg = "Tidak ada training dengan status ini."
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body"),
			h.P(h.Class("text-base-content/70"), g.Text(msg))),
	)
}

// csTrainingsTable = tabel daftar training, dibungkus ui.TableScroll. Kolom
// Aksi hanya muncul bila canWrite=true.
func csTrainingsTable(v CSTrainingsListView) g.Node {
	head := []g.Node{
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Desa")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Topik")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Jadwal")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Trainer")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Peserta")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Attendance")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Catatan")),
	}
	if v.CanWrite {
		head = append(head, h.Th(h.Class("py-2 font-medium"), g.Text("Aksi")))
	}
	rows := make([]g.Node, 0, len(v.Items))
	for _, row := range v.Items {
		rows = append(rows, csTrainingTableRow(v.Base, row, v.CanWrite))
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

func csTrainingTableRow(base string, r CSTrainingRow, canWrite bool) g.Node {
	id := strconv.FormatInt(r.ID, 10)
	cells := []g.Node{
		h.Td(h.Class("py-2 pr-4 max-w-[120px]"), h.Span(h.Class("block truncate"), g.Text(orDash(r.AccountName)))),
		h.Td(h.Class("py-2 pr-4 max-w-[200px]"), h.Span(h.Class("block truncate"), g.Text(orDash(r.TrainingTopic)))),
		h.Td(h.Class("py-2 pr-4 text-sm text-base-content/70"), g.Text(r.TrainingDate)),
		h.Td(h.Class("py-2 pr-4"), h.Span(h.Class("badge badge-sm "+r.StatusBadge), g.Text(r.StatusLabel))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(r.TrainerName))),
		h.Td(h.Class("py-2 pr-4"), g.Text(r.Participants)),
		h.Td(h.Class("py-2 pr-4"), g.Text(r.Attendance)),
		h.Td(h.Class("py-2 pr-4 max-w-[160px]"),
			h.Span(h.Class("block truncate text-base-content/70"), g.Text(orDash(r.Notes)))),
	}
	if canWrite {
		cells = append(cells, h.Td(h.Class("py-2"), csTrainingStatusForm(base, id, r.StatusLabel, r.Notes)))
	}
	return h.Tr(h.Class("border-b border-base-300/50 hover:bg-base-200/50"), g.Group(cells))
}

// csTrainingStatusForm = aksi ganti status per baris (canWrite). Transisi:
// scheduled → completed/rescheduled/cancelled; rescheduled →
// completed/cancelled; completed/cancelled → bisa dibuka ke scheduled.
//
// BL-28: "✓ Selesai" & "Jadwal Ulang" TAK langsung submit — mereka membuka
// panel inline (Datastar data-show, state form efemeral; pola BL-19/BL-26
// showWhen, BUKAN mekanisme baru) untuk menampung attendance/peserta/catatan
// (Selesai) atau tanggal-baru/catatan (Jadwal Ulang). "Batal"/"Buka Ulang"
// tetap submit langsung — hanya kirim status; query COALESCE menjaga field

func csTrainingsPager(v CSTrainingsListView) g.Node {
	base := panelListHref(v.Base+"/trainings", [2]string{"tab", v.Tab}, [2]string{"q", v.Query})
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}
