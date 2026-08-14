package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// cs_renewals_list.go — tampilan daftar Renewal Management CS (Modul 6 slice 6.6).
// Menampilkan langganan dengan field AKSI CS: stage, risk, owner, action plan,
// next action date. Pola identik dengan engagements_list.go.

// CSRenewalTab — satu tab filter renewal stage.
type CSRenewalTab struct {
	Key   string // nilai filter (kosong = semua)
	Label string // label tampil
}

// CSRenewalRow — satu baris tabel Renewal Management.
type CSRenewalRow struct {
	ID             int64
	VillageName    string
	PlanName       string
	RenewalDate    string // end_date dari subscription (read-only)
	RenewalStatus  string // renewal_status dari subscription
	StageLabel     string
	StageBadge     string // badge-* daisyUI
	RiskLabel      string
	RiskBadge      string // badge-* daisyUI
	OwnerName      string
	NextActionDate string
	HrefEdit       string // "/w/{slug}/renewal-management/{id}/edit"
}

// CSRenewalsListView — data lengkap halaman daftar Renewal Management.
type CSRenewalsListView struct {
	Base       string         // "/w/{slug}"
	Tab        string         // nilai tab aktif
	Tabs       []CSRenewalTab // daftar tab dari handler
	Msg        string         // pesan sukses ?ok=
	Err        string         // pesan galat ?err=
	Items      []CSRenewalRow
	CanWrite   bool
	NextCursor string
}

// CSRenewalsList merender halaman daftar Renewal Management.
func CSRenewalsList(v CSRenewalsListView) g.Node {
	return h.Div(h.Class("space-y-4"),
		csRenewalsHeader(v),
		csRenewalsAlert(v.Msg, v.Err),
		csRenewalTabsNav(v),
		csRenewalsTable(v),
		csRenewalsPager(v),
	)
}

func csRenewalsHeader(v CSRenewalsListView) g.Node {
	return h.Div(h.Class("flex flex-wrap items-center justify-between gap-2"),
		h.H1(h.Class("text-xl font-bold"), g.Text("Renewal Management")),
		h.P(h.Class("text-sm text-base-content/70"),
			g.Text("Kelola tahap & rencana aksi renewal langganan desa.")),
	)
}

func csRenewalsAlert(msg, errMsg string) g.Node {
	if msg != "" {
		return h.Div(h.Class("alert alert-success"), g.Text(msg))
	}
	if errMsg != "" {
		return h.Div(h.Class("alert alert-error"), g.Text(errMsg))
	}
	return nil
}

func csRenewalTabsNav(v CSRenewalsListView) g.Node {
	tabs := make([]g.Node, 0, len(v.Tabs))
	for _, t := range v.Tabs {
		active := t.Key == v.Tab
		cls := "tab"
		if active {
			cls += " tab-active"
		}
		href := v.Base + "/renewal-management"
		if t.Key != "" {
			href += "?tab=" + t.Key
		}
		tabs = append(tabs, h.A(h.Href(href), h.Class(cls), g.Text(t.Label)))
	}
	return h.Div(h.Class("tabs tabs-border overflow-x-auto flex-wrap"), g.Group(tabs))
}

func csRenewalsTable(v CSRenewalsListView) g.Node {
	if len(v.Items) == 0 {
		return h.Div(h.Class("card bg-base-100 shadow-sm"),
			h.Div(h.Class("card-body"),
				h.P(h.Class("text-sm text-base-content/60"), g.Text("Tidak ada langganan yang sesuai filter.")),
			),
		)
	}
	headers := []g.Node{
		h.Th(g.Text("Desa")),
		h.Th(g.Text("Plan")),
		h.Th(g.Text("Tgl Renewal")),
		h.Th(g.Text("Status")),
		h.Th(g.Text("Stage Aksi")),
		h.Th(g.Text("Risiko")),
		h.Th(g.Text("Owner")),
		h.Th(g.Text("Tindakan Berikutnya")),
	}
	if v.CanWrite {
		headers = append(headers, h.Th(g.Text("")))
	}
	rows := make([]g.Node, 0, len(v.Items))
	for _, row := range v.Items {
		rows = append(rows, csRenewalTableRow(row, v.CanWrite))
	}
	return h.Div(h.Class("card bg-base-100 shadow-sm"),
		ui.TableScroll(h.Table(h.Class("table table-sm"),
			h.THead(h.Tr(g.Group(headers))),
			h.TBody(g.Group(rows)),
		)),
	)
}

func csRenewalTableRow(r CSRenewalRow, canWrite bool) g.Node {
	cells := []g.Node{
		h.Td(h.Class("py-2 pr-4 max-w-[150px]"), h.Span(h.Class("block truncate"), g.Text(r.VillageName))),
		h.Td(h.Class("py-2 pr-4 whitespace-nowrap"), g.Text(r.PlanName)),
		h.Td(h.Class("py-2 pr-4 whitespace-nowrap"), g.Text(orDash(r.RenewalDate))),
		h.Td(h.Class("py-2 pr-4 whitespace-nowrap text-sm text-base-content/70"), g.Text(orDash(r.RenewalStatus))),
		h.Td(h.Class("py-2 pr-4"),
			h.Span(h.Class("badge badge-sm "+r.StageBadge), g.Text(r.StageLabel)),
		),
		h.Td(h.Class("py-2 pr-4"),
			h.Span(h.Class("badge badge-sm "+r.RiskBadge), g.Text(r.RiskLabel)),
		),
		h.Td(h.Class("py-2 pr-4 text-sm"), g.Text(orDash(r.OwnerName))),
		h.Td(h.Class("py-2 pr-4 whitespace-nowrap text-sm text-base-content/70"), g.Text(orDash(r.NextActionDate))),
	}
	if canWrite {
		cells = append(cells, h.Td(h.Class("py-2"),
			h.A(h.Href(r.HrefEdit), h.Class("btn btn-xs btn-ghost min-h-11"), g.Text("Edit")),
		))
	}
	return h.Tr(h.Class("border-b border-base-300/50 hover:bg-base-200/50"), g.Group(cells))
}

func csRenewalsPager(v CSRenewalsListView) g.Node {
	if v.NextCursor == "" {
		return h.Div(h.Class("flex flex-wrap items-center gap-2"),
			h.Span(h.Class("text-sm text-base-content/60"), g.Text("Ujung daftar.")))
	}
	href := v.Base + "/renewal-management?after=" + v.NextCursor
	if v.Tab != "" {
		href += "&tab=" + v.Tab
	}
	return h.Div(
		h.Class("flex flex-wrap items-center gap-2"),
		h.A(h.Href(href), h.Class("btn min-h-11"), g.Text("Berikutnya »")),
	)
}
