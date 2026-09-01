package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// all_activities.go — halaman daftar Activities lintas-context (M7 top-level).
// Berbeda dari sales_activities.go (context='sales') — halaman ini menampilkan
// semua aktivitas (sales+cs+general) dalam satu tabel. Murni-data: Owner sudah
// nama, Created sudah diformat handler. Reuse komponen bersama (badge, target
// link, tabel row) dari sales_activities.go (paket sama).

// AllActivitiesListView = data halaman daftar lintas-context. Items = satu
// halaman keyset; NextCursor kosong = ujung daftar.
type AllActivitiesListView struct {
	Base       string
	CanWrite   bool
	Err        string
	Msg        string
	Items      []ActivityRow
	NextCursor string
	Query      string // ?q= pencarian bebas (BL-6); "" = tak mencari
}

// AllActivitiesList merender halaman: header + tombol buat per-kind + alert +
// tabel (atau keadaan kosong) + pager.
func AllActivitiesList(v AllActivitiesListView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Aktivitas")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Semua catatan aktivitas lintas-modul — Sales, Customer Success, dan umum.")),
			),
			ui.When(v.CanWrite, allActivityNewButtons(v.Base)),
		),
		searchBox(v.Base+"/activity-log", v.Query, "Cari aktivitas — subjek…", "Cari aktivitas"),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "allact-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "allact-ok", g.Text(v.Msg)))
	}
	if len(v.Items) == 0 {
		body = append(body, emptyAllActivities(v))
	} else {
		body = append(body, allActivitiesTable(v), allActivitiesPager(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// allActivityNewButtons = tombol buat aktivitas (tanpa pre-fill target).
// Reuse URL /activities/new (form Sales Activity yang sudah ada) karena CRUD
// belum dipecah per-context di iterasi ini. Baris flex-wrap agar tak mendorong.
func allActivityNewButtons(base string) g.Node {
	btn := func(label, kind string) g.Node {
		return h.A(
			h.Href(base+"/activities/new?kind="+kind),
			h.Class("btn btn-sm btn-primary min-h-11"), g.Text(label))
	}
	return h.Div(
		h.Class("flex flex-wrap items-center gap-2"),
		btn("Tugas", "task"), btn("Panggilan", "call"), btn("Catatan", "note"),
	)
}

// allActivitiesTable = tabel aktivitas lintas-context. Menambah kolom "Konteks"
// di samping kolom lain agar pengguna tahu dari modul mana aktivitas berasal.
func allActivitiesTable(v AllActivitiesListView) g.Node {
	rows := make([]g.Node, 0, len(v.Items))
	for _, a := range v.Items {
		rows = append(rows, allActivityTableRow(v.Base, a))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Jenis")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Subjek")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Target")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Konteks")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Pemilik")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
					h.Th(h.Class("py-2 font-medium"), g.Text("Tanggal")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

// allActivityTableRow = satu baris tabel. Sama dengan activityTableRow di
// sales_activities.go + kolom Konteks (badge per activity_context).
func allActivityTableRow(base string, a ActivityRow) g.Node {
	href := base + "/activities/" + strconv.FormatInt(a.ID, 10)
	link := func(text, cls string) g.Node {
		return h.Td(h.Class(cls), h.A(h.Href(href), h.Class("block truncate"), g.Text(text)))
	}
	return h.Tr(
		h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), activityKindBadge(a.Kind))),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), h.Class("block truncate font-medium"),
			g.Text(a.Subject))),
		h.Td(h.Class("py-2 pr-4"), activityTargetLink(base, a.TargetType, a.TargetID)),
		h.Td(h.Class("py-2 pr-4"), activityContextBadge(a.Context)),
		link(orDash(a.Owner), "py-2 pr-4"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), activityStatusBadge(a.Status))),
		link(orDash(a.Created), "py-2"),
	)
}

// activityContextBadge = badge ringan per activity_context untuk kolom Konteks.
// Warna token semantik daisyUI (bukan absolut).
func activityContextBadge(ctx string) g.Node {
	cls, label := "badge badge-ghost badge-sm", activityContextLabel(ctx)
	switch ctx {
	case "sales":
		cls = "badge badge-info badge-sm"
	case "cs":
		cls = "badge badge-success badge-sm"
	case "general":
		cls = "badge badge-ghost badge-sm"
	}
	return h.Span(h.Class(cls), g.Text(label))
}

// activityContextLabel memetakan activity_context → label Indonesia.
func activityContextLabel(ctx string) string {
	switch ctx {
	case "sales":
		return "Sales"
	case "cs":
		return "CS"
	case "general":
		return "Umum"
	default:
		return ctx
	}
}

func emptyAllActivities(v AllActivitiesListView) g.Node {
	// Pencarian tanpa hasil: pesan khusus + tautan reset (buang q).
	if v.Query != "" {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body items-start"),
				h.P(h.Class("text-base-content/70"),
					g.Text("Tak ada aktivitas yang cocok pencarian.")),
				h.A(h.Href(v.Base+"/activity-log"), h.Class("btn btn-ghost btn-sm min-h-11"),
					g.Text("Reset pencarian")),
			),
		)
	}
	if v.NextCursor == "" {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body"),
				h.P(h.Class("text-base-content/70"), g.Text("Belum ada aktivitas."))),
		)
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body items-start"),
			h.P(h.Class("text-base-content/70"),
				g.Text("Belum ada aktivitas pada tampilan ini.")),
			h.A(h.Href(v.Base+"/activity-log"), h.Class("btn btn-ghost btn-sm min-h-11"),
				g.Text("« Kembali ke awal")),
		),
	)
}

func allActivitiesPager(v AllActivitiesListView) g.Node {
	if v.NextCursor == "" {
		return h.Div(h.Class("flex flex-wrap items-center gap-2"),
			h.Span(h.Class("text-sm text-base-content/60"), g.Text("Ujung daftar.")))
	}
	href := appendQuery(v.Base+"/activity-log?after="+v.NextCursor, v.Query)
	return h.Div(
		h.Class("flex flex-wrap items-center gap-2"),
		h.A(h.Href(href), h.Class("btn min-h-11"), g.Text("Berikutnya »")),
	)
}
