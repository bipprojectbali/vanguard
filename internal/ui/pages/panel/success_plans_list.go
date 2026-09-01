package panel

import (
	"fmt"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// success_plans_list.go — tampilan daftar Success Plans (Modul 6 slice 6.3).
// View murni-data: tidak ada import db/authz/session. Handler memetakan
// db.ListSuccessPlansRow → SuccessPlanRow.

// SuccessPlanTab — satu tab filter status plan.
type SuccessPlanTab struct {
	Key   string // nilai filter (kosong = semua)
	Label string // label tampil
}

// SuccessPlanRow — satu baris tabel Success Plans.
type SuccessPlanRow struct {
	ID          int64
	AccountName string
	PlanName    string
	Objective   string
	StatusLabel string
	StatusBadge string // badge-* daisyUI
	Progress    int    // 0–100
	OwnerName   string
	TargetDate  string // YYYY-MM-DD, bisa kosong
	HrefEdit    string // "/w/{slug}/success-plans/{id}/edit"
}

// SuccessPlansListView — data lengkap halaman daftar Success Plans.
type SuccessPlansListView struct {
	Base       string           // "/w/{slug}"
	Tab        string           // nilai tab aktif
	Query      string           // ?q= pencarian bebas (BL-6); "" = tak mencari
	Tabs       []SuccessPlanTab // daftar tab dari handler
	Msg        string           // pesan sukses ?ok=
	Err        string           // pesan galat ?err=
	Items      []SuccessPlanRow
	CanWrite   bool
	NextCursor string
	After      string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail      string // BL-7: jejak cursor halaman sebelumnya (?trail=)
}

// SuccessPlansList merender halaman daftar Success Plans.
func SuccessPlansList(v SuccessPlansListView) g.Node {
	return h.Div(h.Class("space-y-4"),
		successPlansListHeader(v),
		successPlansListAlert(v.Msg, v.Err),
		successPlanTabsNav(v),
		searchBox(v.Base+"/success-plans", v.Query,
			"Cari plan — nama plan atau desa…", "Cari success plan",
			hiddenField{"tab", v.Tab}),
		successPlansTable(v),
		successPlansPager(v),
	)
}

func successPlansListHeader(v SuccessPlansListView) g.Node {
	var createBtn g.Node
	if v.CanWrite {
		createBtn = h.A(
			h.Href(v.Base+"/success-plans/new"),
			h.Class("btn btn-primary btn-sm min-h-11"),
			g.Text("+ Buat Plan"),
		)
	}
	return h.Div(h.Class("flex flex-wrap items-center justify-between gap-2"),
		h.Div(
			h.H1(h.Class("text-xl font-bold"), g.Text("Success Plans")),
			h.P(h.Class("text-sm text-base-content/70"),
				g.Text("Rencana sukses per-desa — tujuan, metrik, dan progres pencapaian.")),
		),
		g.If(createBtn != nil, createBtn),
	)
}

func successPlansListAlert(msg, errMsg string) g.Node {
	if msg != "" {
		return h.Div(h.Class("alert alert-success"), g.Text(msg))
	}
	if errMsg != "" {
		return h.Div(h.Class("alert alert-error"), g.Text(errMsg))
	}
	return nil
}

func successPlanTabsNav(v SuccessPlansListView) g.Node {
	tabs := make([]g.Node, 0, len(v.Tabs))
	for _, t := range v.Tabs {
		active := t.Key == v.Tab
		cls := "tab"
		if active {
			cls += " tab-active"
		}
		href := withQuery(v.Base+"/success-plans", v.Query, hiddenField{"tab", t.Key})
		tabs = append(tabs, h.A(h.Href(href), h.Class(cls), g.Text(t.Label)))
	}
	return h.Div(h.Class("tabs tabs-border overflow-x-auto flex-wrap"), g.Group(tabs))
}

func successPlansTable(v SuccessPlansListView) g.Node {
	if len(v.Items) == 0 {
		msg := "Belum ada success plan yang sesuai filter."
		var reset g.Node
		if v.Query != "" {
			msg = "Belum ada success plan yang cocok pencarian."
			reset = h.A(
				h.Href(withQuery(v.Base+"/success-plans", "", hiddenField{"tab", v.Tab})),
				h.Class("btn btn-ghost btn-sm min-h-11"), g.Text("« Reset pencarian"))
		}
		return h.Div(h.Class("card bg-base-100 shadow-sm"),
			h.Div(h.Class("card-body items-start"),
				h.P(h.Class("text-sm text-base-content/60"), g.Text(msg)),
				g.If(reset != nil, reset),
			),
		)
	}
	headers := []g.Node{
		h.Th(g.Text("Desa")),
		h.Th(g.Text("Nama Plan")),
		h.Th(g.Text("Objektif")),
		h.Th(g.Text("Status")),
		h.Th(g.Text("Progres")),
		h.Th(g.Text("Owner CS")),
		h.Th(g.Text("Target")),
	}
	if v.CanWrite {
		headers = append(headers, h.Th(g.Text("")))
	}
	rows := make([]g.Node, 0, len(v.Items))
	for _, row := range v.Items {
		rows = append(rows, successPlanTableRow(row, v.CanWrite))
	}
	return h.Div(h.Class("card bg-base-100 shadow-sm"),
		ui.TableScroll(h.Table(h.Class("table table-sm"),
			h.THead(h.Tr(g.Group(headers))),
			h.TBody(g.Group(rows)),
		)),
	)
}

func successPlanTableRow(r SuccessPlanRow, canWrite bool) g.Node {
	cells := []g.Node{
		h.Td(h.Class("py-2 pr-4 max-w-[150px]"),
			h.Span(h.Class("block truncate"), g.Text(r.AccountName))),
		h.Td(h.Class("py-2 pr-4 max-w-[180px]"),
			h.Span(h.Class("block truncate font-medium"), g.Text(r.PlanName))),
		h.Td(h.Class("py-2 pr-4 max-w-[200px] text-sm text-base-content/70"),
			h.Span(h.Class("block truncate"), g.Text(orDash(r.Objective)))),
		h.Td(h.Class("py-2 pr-4"),
			h.Span(h.Class("badge badge-sm "+r.StatusBadge), g.Text(r.StatusLabel))),
		h.Td(h.Class("py-2 pr-4 whitespace-nowrap"),
			successPlanProgressBar(r.Progress)),
		h.Td(h.Class("py-2 pr-4 text-sm"), g.Text(orDash(r.OwnerName))),
		h.Td(h.Class("py-2 pr-4 whitespace-nowrap text-sm text-base-content/70"),
			g.Text(orDash(r.TargetDate))),
	}
	if canWrite {
		cells = append(cells, h.Td(h.Class("py-2"),
			h.A(h.Href(r.HrefEdit), h.Class("btn btn-xs btn-ghost min-h-11"),
				g.Text("Edit")),
		))
	}
	return h.Tr(h.Class("border-b border-base-300/50 hover:bg-base-200/50"), g.Group(cells))
}

// successPlanProgressBar — progress % dengan label angka + bar visual.
func successPlanProgressBar(pct int) g.Node {
	barWidth := fmt.Sprintf("%d%%", pct)
	barColor := "bg-success"
	switch {
	case pct < 30:
		barColor = "bg-error"
	case pct < 70:
		barColor = "bg-warning"
	}
	return h.Div(h.Class("flex items-center gap-2"),
		h.Span(h.Class("text-xs text-base-content/70 w-8 text-right"),
			g.Text(fmt.Sprintf("%d%%", pct))),
		h.Div(h.Class("flex-1 bg-base-300 rounded-full h-1.5 min-w-[60px]"),
			h.Div(h.Class(barColor+" h-1.5 rounded-full"), h.Style("width:"+barWidth)),
		),
	)
}

func successPlansPager(v SuccessPlansListView) g.Node {
	base := panelListHref(v.Base+"/success-plans", [2]string{"tab", v.Tab}, [2]string{"q", v.Query})
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}
