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
	After      string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail      string // BL-7: jejak cursor halaman sebelumnya (?trail=)
	Query      string // ?q= pencarian bebas (BL-6); "" = tak mencari
}

// AllActivitiesList merender halaman: header + tombol "Tambah Aktivitas" (pola
// 1-form BL-19/BL-42) + alert + tabel (atau keadaan kosong) + pager.
func AllActivitiesList(v AllActivitiesListView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Aktivitas")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Semua catatan aktivitas lintas-modul — Sales, Customer Success, dan umum.")),
			),
			// BL-42: satu tombol "Tambah Aktivitas" (jenis dipilih di form) alih-alih
			// 3 tombol per-kind — seragam dgn Sales Activities (BL-19) & menjangkau
			// SEMUA kind (termasuk meeting/chat). Tanpa targetFilter (feed global tak
			// pre-seleksi target).
			ui.When(v.CanWrite, activityNewButton(v.Base, "")),
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

// allActivityTableRow = satu baris tabel. Baris ACTIVITY (Sales/umum) sama dengan
// activityTableRow di sales_activities.go + kolom Konteks. Baris ENGAGEMENT (CS,
// Source=="cs", BL-41) dirender READ-ONLY: tak ada halaman /activities/{id} untuk
// engagement (CRUD-nya di modul CS), jadi tautan mengarah ke DESA induk; kolom
// Jenis pakai TypeLabel (engagement_type) & Status pakai StatusBadgeClass yang
// sudah diputuskan handler (peta status engagement ≠ activity).
func allActivityTableRow(base string, a ActivityRow) g.Node {
	if a.Source == "cs" {
		return allActivityEngagementRow(base, a)
	}
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

// allActivityEngagementRow = baris CS engagement di feed Activities global (BL-41).
// Read-only: tautan ke desa induk (base/accounts/{TargetID}), bukan /activities/{id}.
// Jenis = badge TypeLabel (engagement_type); Status = StatusBadgeClass dari handler.
func allActivityEngagementRow(base string, a ActivityRow) g.Node {
	href := base + "/accounts/" + strconv.FormatInt(a.TargetID, 10)
	link := func(text, cls string) g.Node {
		return h.Td(h.Class(cls), h.A(h.Href(href), h.Class("block truncate"), g.Text(text)))
	}
	return h.Tr(
		h.Class("border-b border-base-300/50 hover:bg-base-200/50"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), engagementTypeBadge(a.TypeLabel))),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), h.Class("block truncate font-medium"),
			g.Text(a.Subject))),
		h.Td(h.Class("py-2 pr-4"), activityTargetLink(base, a.TargetType, a.TargetID)),
		h.Td(h.Class("py-2 pr-4"), activityContextBadge(a.Context)),
		link(orDash(a.Owner), "py-2 pr-4"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), engagementStatusBadge(a.Status, a.StatusBadgeClass))),
		link(orDash(a.Created), "py-2"),
	)
}

// engagementTypeBadge = badge kolom Jenis untuk baris CS. Warna secondary agar
// beda visual dari kind activity (info/success/…); label sudah di-Indonesia-kan
// handler (engagementTypeLabel). Kosong → "—".
func engagementTypeBadge(label string) g.Node {
	if label == "" {
		return h.Span(h.Class("text-base-content/50"), g.Text("—"))
	}
	return h.Span(h.Class("badge badge-secondary"), g.Text(label))
}

// engagementStatusBadge = badge status engagement; kelas daisyUI sudah diputuskan
// handler (engagementStatusLabel → badge-info/success/ghost/warning). Kosong → "—".
func engagementStatusBadge(label, badgeClass string) g.Node {
	if label == "" {
		return h.Span(h.Class("text-base-content/50"), g.Text("—"))
	}
	cls := "badge badge-ghost"
	if badgeClass != "" {
		cls = "badge " + badgeClass
	}
	return h.Span(h.Class(cls), g.Text(label))
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
	base := panelListHref(v.Base+"/activity-log", [2]string{"q", v.Query})
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}
