package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_leads.go — view hub Lead: daftar (dengan tab), baris, penolakan 403.
// Murni-data: setiap nilai (termasuk yang disamarkan F4) SUDAH diputuskan
// handler; view tak memanggil authz/session. Meniru accounts.go.

// LeadRow = satu baris daftar lead. EstValue SUDAH diformat & disamarkan F4 di
// handler (kosong bila tak berhak). Owner = nama orang (bukan id).
type LeadRow struct {
	ID            int64
	EntityCode    string
	LeadName      string
	ContactPerson string
	LeadSource    string
	Status        string
	Rating        string
	EstValue      string
	Owner         string
}

// LeadsListView = data halaman daftar lead. Tab = "" / "my" / "unqualified"
// (menentukan tab aktif). CanWrite = tombol "Lead Baru" tampil. NextCursor "" =
// halaman terakhir. HideMyTab = sembunyikan tab "Lead Saya" saat cakupan aktor
// 'own' (BL-1): filter dasar sudah mengunci lead_owner=uid → "Semua" ≡ "Lead
// Saya", jadi tab itu redundan. Diputuskan HANDLER (dari filter.IsOwn), bukan view.
type LeadsListView struct {
	Base       string
	Items      []LeadRow
	Tab        string
	Query      string // ?q= pencarian bebas (BL-6); "" = tak mencari
	CanWrite   bool
	HideMyTab  bool
	NextCursor string
	After      string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail      string // BL-7: jejak cursor halaman sebelumnya (?trail=)
	Err        string
	Msg        string
}

// leadTabDef = satu tab daftar (label + nilai query). Sumber tunggal urutan tab.
type leadTabDef struct {
	key   string
	label string
}

var leadTabs = []leadTabDef{
	{"", "Semua"},
	{"my", "Lead Saya"},
	{"unqualified", "Unqualified"},
}

// LeadsList merender daftar lead: header + aksi tambah, tab, alert, tabel keyset.
func LeadsList(v LeadsListView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Leads")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Prospek yang belum jadi pelanggan — kualifikasi lalu konversi.")),
			),
			ui.When(v.CanWrite, h.A(
				h.Href(v.Base+"/leads/new"), h.Class("btn btn-primary min-h-11"),
				g.Text("Lead Baru"),
			)),
		),
		leadTabsNav(v),
		searchBox(v.Base+"/leads", v.Query, "Cari lead — nama atau kode…", "Cari lead",
			hiddenField{"tab", v.Tab}),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "leads-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "leads-ok", g.Text(v.Msg)))
	}
	if len(v.Items) == 0 {
		body = append(body, emptyLeads(v))
	} else {
		body = append(body, leadsTable(v))
		body = append(body, leadsPager(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// leadTabsNav = tab sebagai LINK <a> berparam (navigasi bookmarkable, lolos
// gotcha #16). Tab aktif ditandai; ganti tab mereset cursor (halaman pertama).
func leadTabsNav(v LeadsListView) g.Node {
	tabs := make([]g.Node, 0, len(leadTabs))
	for _, t := range leadTabs {
		// BL-1: tab "Lead Saya" (my) redundan saat cakupan 'own' — lewati.
		if t.key == "my" && v.HideMyTab {
			continue
		}
		// q dibawa lintas tab (mencari lalu ganti tab tak menghapus pencarian).
		href := withQuery(v.Base+"/leads", v.Query, hiddenField{"tab", t.key})
		cls := "tab"
		if t.key == v.Tab {
			cls += " tab-active font-medium"
		}
		tabs = append(tabs, h.A(h.Href(href), h.Class(cls+" min-h-11"), g.Text(t.label)))
	}
	return h.Div(h.Role("tablist"), h.Class("tabs tabs-bordered flex-wrap"), g.Group(tabs))
}

func emptyLeads(v LeadsListView) g.Node {
	if v.NextCursor == "" {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body items-start"),
				h.P(h.Class("text-base-content/70"),
					g.Text("Belum ada lead yang cocok pada tampilan ini.")),
				h.A(h.Href(withQuery(v.Base+"/leads", v.Query, hiddenField{"tab", v.Tab})),
					h.Class("btn btn-ghost btn-sm min-h-11"), g.Text("« Kembali ke awal")),
			),
		)
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body"),
			h.P(h.Class("text-base-content/70"), g.Text("Belum ada lead."))),
	)
}

// leadsTable = tabel daftar lead, dibungkus ui.TableScroll (scroll terkurung).
func leadsTable(v LeadsListView) g.Node {
	rows := make([]g.Node, 0, len(v.Items))
	for _, l := range v.Items {
		rows = append(rows, leadRow(v.Base, l))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Kode")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Lead")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Sumber")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Rating")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Estimasi")),
					h.Th(h.Class("py-2 font-medium"), g.Text("Pemilik")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func leadRow(base string, l LeadRow) g.Node {
	href := base + "/leads/" + strconv.FormatInt(l.ID, 10)
	link := func(text string, cls string) g.Node {
		return h.Td(h.Class(cls), h.A(h.Href(href), h.Class("block truncate"), g.Text(text)))
	}
	return h.Tr(
		h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
		link(orDash(l.EntityCode), "py-2 pr-4 font-mono text-xs"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), h.Class("block min-w-0"),
			h.Div(h.Class("truncate font-medium"), g.Text(l.LeadName)),
			ui.When(l.ContactPerson != "", h.Div(
				h.Class("truncate text-xs text-base-content/60"), g.Text(l.ContactPerson))),
		)),
		link(orDash(l.LeadSource), "py-2 pr-4"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), leadStatusBadge(l.Status))),
		link(orDash(l.Rating), "py-2 pr-4"),
		link(orDash(l.EstValue), "py-2 pr-4"),
		link(orDash(l.Owner), "py-2"),
	)
}

// leadStatusBadge = badge status berwarna semantik daisyUI (token, bukan absolut).
func leadStatusBadge(status string) g.Node {
	cls := "badge badge-ghost"
	switch status {
	case "Qualified":
		cls = "badge badge-success"
	case "Unqualified":
		cls = "badge badge-error"
	case "Contacted":
		cls = "badge badge-info"
	case "Converted":
		cls = "badge badge-primary"
	}
	return h.Span(h.Class(cls), g.Text(orDash(status)))
}

func leadsPager(v LeadsListView) g.Node {
	base := panelListHref(v.Base+"/leads", [2]string{"tab", v.Tab}, [2]string{"q", v.Query})
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}

// SalesForbidden = penolakan 403 modul Sales (Leads/Deals). module = nama modul
// untuk judul & kalimat. Menyebut SIAPA yang bisa membantu, bukan "akses ditolak".
func SalesForbidden(module string) g.Node {
	return h.Div(
		h.Class("grid gap-4 min-w-0 max-w-xl mx-auto py-8"),
		h.H1(h.Class("text-xl font-semibold mb-2"), g.Text(module)),
		h.P(h.Class("text-base-content/70"),
			g.Text("Modul "+module+" hanya bisa dibuka oleh pemegang peran CRM "+
				"(admin, manajer, sales, atau CS).")),
		h.P(h.Class("text-sm text-base-content/60"),
			g.Text("Hubungi admin workspace bila Anda perlu akses ke data ini.")),
	)
}
