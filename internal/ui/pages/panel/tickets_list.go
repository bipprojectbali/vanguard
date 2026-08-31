package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// tickets_list.go — view daftar Tickets / Cases (Modul 6 Customer Success,
// slice B2, wireframe 6.9). Murni-data: semua nilai sudah diputuskan handler;
// view tak memanggil authz/session. Keyset pagination (?after=), filter tab
// (?tab=), KPI cards. Meniru sales_leads.go + sla_policies.go.
//
// F3 ownership (scope_all/is_own) diputuskan di handler melalui
// TicketsListFilterFor (ownership.go); view tak tahu cakupan — dia hanya render
// data yang dioper.

// TicketKPIs = agregat KPI header halaman. Semua int (CountTicketKPIsRow sudah
// dipetakan handler via ticketKPIView).
type TicketKPIs struct {
	Open          int
	Unassigned    int
	AtRisk        int
	Breached      int
	ResolvedToday int
}

// TicketRow = satu baris daftar tiket. Semua nilai sudah diformat handler
// (ticketRowView): Number="#TK-{id}", SLALabel=label deadline/status.
type TicketRow struct {
	ID          int64
	Number      string
	AccountName string
	Subject     string
	Priority    string
	Status      string
	SLALabel    string
	AssignedTo  string
}

// ticketTabDef = satu tab filter daftar tiket.
type ticketTabDef struct {
	key   string // nilai ?tab= (kosong = Semua)
	label string
}

// ticketTabs = urutan tab filter; sumber tunggal, cermin filter handler.
//
//	key ""              → semua status
//	key "baru"          → status='baru'
//	key "ditugaskan"    → status='ditugaskan'
//	key "eskalasi"      → status='eskalasi'
//	key "sla-risiko"    → filter_sla_at_risk=true
//	key "sla-langgar"   → filter_sla_breached=true
//	key "selesai"       → status='selesai'
var ticketTabs = []ticketTabDef{
	{"", "Semua"},
	{"baru", "Baru"},
	{"ditugaskan", "Ditugaskan"},
	{"eskalasi", "Eskalasi"},
	{"sla-risiko", "SLA Berisiko"},
	{"sla-langgar", "Terlanggar"},
	{"selesai", "Selesai"},
}

// TicketsListView = data halaman /tickets. CanWrite = Support/Manager/Admin
// (tombol Tiket Baru + aksi ubah status). NextCursor "" = halaman terakhir.
type TicketsListView struct {
	Base       string
	KPIs       TicketKPIs
	Items      []TicketRow
	Tab        string
	CanWrite   bool
	NextCursor string
	Err        string
	Msg        string
}

// TicketsList merender halaman daftar: KPI cards + tab + alert + tabel keyset.
func TicketsList(v TicketsListView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Tickets / Cases")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Permintaan layanan dan masalah desa — lacak dari pembukaan hingga penyelesaian.")),
			),
			ui.When(v.CanWrite, h.A(
				h.Href(v.Base+"/tickets/new"), h.Class("btn btn-primary min-h-11"),
				g.Text("+ Tiket Baru"),
			)),
		),
		ticketKPICards(v.KPIs, v.Base),
		ticketTabsNav(v),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "tickets-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "tickets-ok", g.Text(v.Msg)))
	}
	if len(v.Items) == 0 {
		body = append(body, emptyTickets(v))
	} else {
		body = append(body, ticketsTable(v))
		body = append(body, ticketsPager(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// ticketKPICards = 5 kartu metrik operasional atas halaman (mobile: 2 kolom,
// md: 5 kolom). Token semantik daisyUI; bukan warna absolut.
func ticketKPICards(k TicketKPIs, base string) g.Node {
	return h.Div(
		h.Class("grid grid-cols-2 md:grid-cols-5 gap-3 min-w-0"),
		ticketKPICard("Tiket Terbuka", strconv.Itoa(k.Open), base+"/tickets", ""),
		ticketKPICard("Belum Ditugaskan", strconv.Itoa(k.Unassigned), base+"/tickets?tab=baru", "text-warning"),
		ticketKPICard("SLA Berisiko", strconv.Itoa(k.AtRisk), base+"/tickets?tab=sla-risiko", "text-warning"),
		ticketKPICard("SLA Terlanggar", strconv.Itoa(k.Breached), base+"/tickets?tab=sla-langgar", "text-error"),
		ticketKPICard("Selesai Hari Ini", strconv.Itoa(k.ResolvedToday), base+"/tickets?tab=selesai", "text-success"),
	)
}

// ticketKPICard = satu kartu KPI: label di atas, angka di bawah. Link ke tab
// terkait agar dapat di-klik langsung jadi filter. colorCls="" = warna default.
func ticketKPICard(label, value, href, colorCls string) g.Node {
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

// ticketTabsNav = tab sebagai LINK <a> berparam (navigasi bookmarkable, lolos
// gotcha #16). Ganti tab reset cursor (halaman pertama).
func ticketTabsNav(v TicketsListView) g.Node {
	tabs := make([]g.Node, 0, len(ticketTabs))
	for _, t := range ticketTabs {
		href := v.Base + "/tickets"
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

func emptyTickets(v TicketsListView) g.Node {
	if v.NextCursor != "" {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body items-start"),
				h.P(h.Class("text-base-content/70"),
					g.Text("Tidak ada tiket pada tampilan ini.")),
				h.A(h.Href(v.Base+"/tickets"), h.Class("btn btn-ghost btn-sm min-h-11"),
					g.Text("« Kembali ke awal")),
			),
		)
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body"),
			h.P(h.Class("text-base-content/70"), g.Text("Belum ada tiket di ruang kerja ini."))),
	)
}

func ticketsPager(v TicketsListView) g.Node {
	if v.NextCursor == "" {
		return h.Div(h.Class("flex flex-wrap items-center gap-2"),
			h.Span(h.Class("text-sm text-base-content/60"), g.Text("Ujung daftar.")))
	}
	href := v.Base + "/tickets?after=" + v.NextCursor
	if v.Tab != "" {
		href += "&tab=" + v.Tab
	}
	return h.Div(
		h.Class("flex flex-wrap items-center gap-2"),
		h.A(h.Href(href), h.Class("btn min-h-11"), g.Text("Berikutnya »")),
	)
}
