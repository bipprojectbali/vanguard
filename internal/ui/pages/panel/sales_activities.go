package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_activities.go — view daftar Sales Activity Log (4.4): tabel berkeyset
// aktivitas (task/call/note) bercakupan F3. Murni-data: Owner sudah nama, Created
// sudah diformat handler. Target dirender TIPE + tautan by-id (bukan nama) untuk
// hindari N+1 (rule 13) — nama target diresolusi hanya di detail. Meniru
// sales_deals.go tampilan Tabel.

// ActivityRow = satu aktivitas untuk baris Tabel. TargetType/TargetID mentah agar
// view merakit tautan; Status kosong ("") untuk kind tanpa status (call/note).
// Context = activity_context ("sales"/"cs"/"general") — dipakai kolom Konteks
// di halaman lintas-context (AllActivitiesList); diabaikan di Sales Activities.
type ActivityRow struct {
	ID         int64
	Kind       string
	Subject    string
	TargetType string
	TargetID   int64
	Owner      string
	Status     string
	Created    string
	Context    string // activity_context; "" di Sales Activities (tak ditampilkan)
}

// ActivitiesListView = data halaman daftar. Items = satu halaman keyset;
// NextCursor kosong = ujung daftar. TargetFilter "type:id" (e.g. "account:42")
// mengaktifkan mode filter: hanya aktivitas entitas itu yang ditampilkan;
// TargetLabel = nama entitas untuk header ("Desa Maju Jaya"). Kosong = tanpa filter.
type ActivitiesListView struct {
	Base         string
	CanWrite     bool
	Err          string
	Msg          string
	Items        []ActivityRow
	NextCursor   string
	After        string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail        string // BL-7: jejak cursor halaman sebelumnya (?trail=)
	Query        string // ?q= pencarian bebas (BL-6); "" = tak mencari
	TargetFilter string // "type:id" saat difilter per-entitas; "" = semua
	TargetLabel  string // nama entitas yang di-resolve handler (best-effort)
}

// ActivitiesList merender halaman: header + tombol buat per-kind + alert + tabel
// (atau keadaan kosong) + pager. Saat TargetFilter terisi, header menampilkan
// nama entitas dan ada tautan "Kembali ke semua" untuk lepas filter.
func ActivitiesList(v ActivitiesListView) g.Node {
	title := "Sales Activities"
	subtitle := "Catatan aktivitas penjualan — tugas, panggilan, dan catatan."
	var backLink g.Node
	if v.TargetFilter != "" {
		if v.TargetLabel != "" {
			title = "Aktivitas — " + v.TargetLabel
		}
		subtitle = "Menampilkan aktivitas untuk entitas ini saja."
		backLink = h.A(
			h.Href(v.Base+"/activities"),
			h.Class("text-sm text-base-content/60 link link-hover"),
			g.Text("« Semua aktivitas"),
		)
	}
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				backLink,
				h.H1(h.Class("text-xl font-semibold"), g.Text(title)),
				h.P(h.Class("text-base-content/70"), g.Text(subtitle)),
			),
			ui.When(v.CanWrite, activityNewButton(v.Base, v.TargetFilter)),
		),
		searchBox(v.Base+"/activities", v.Query, "Cari aktivitas — subjek…", "Cari aktivitas",
			hiddenField{"target", v.TargetFilter}),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "act-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "act-ok", g.Text(v.Msg)))
	}

	if len(v.Items) == 0 {
		body = append(body, emptyActivities(v))
	} else {
		body = append(body, activitiesTable(v), activitiesPager(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// activityNewButton = satu tautan "Tambah Aktivitas" → form tunggal (BL-19).
// Jenis dipilih di dalam form (dropdown), bukan lagi 3 tombol per-kind — pola
// per-tombol tak menskala saat kind tumbuh (M7: meeting/chat/email). Saat
// targetFilter terisi (mode filter per-entitas), link menyertakan ?target= agar
// form langsung pre-seleksi target tersebut. Baris flex-wrap agar tak mendorong
// di mobile 375px.
func activityNewButton(base, targetFilter string) g.Node {
	href := base + "/activities/new"
	if targetFilter != "" {
		href += "?target=" + targetFilter
	}
	return h.Div(
		h.Class("flex flex-wrap items-center gap-2"),
		h.A(h.Href(href), h.Class("btn btn-sm btn-primary min-h-11"),
			g.Text("Tambah Aktivitas")),
	)
}

// activitiesTable = tabel aktivitas, dibungkus ui.TableScroll (scroll terkurung,
// tak meluberkan halaman di mobile).
func activitiesTable(v ActivitiesListView) g.Node {
	rows := make([]g.Node, 0, len(v.Items))
	for _, a := range v.Items {
		rows = append(rows, activityTableRow(v.Base, a))
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
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Pemilik")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
					h.Th(h.Class("py-2 font-medium"), g.Text("Tanggal")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func activityTableRow(base string, a ActivityRow) g.Node {
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
		link(orDash(a.Owner), "py-2 pr-4"),
		h.Td(h.Class("py-2 pr-4"), h.A(h.Href(href), activityStatusBadge(a.Status))),
		link(orDash(a.Created), "py-2"),
	)
}

// activityKindBadge = badge jenis berwarna token semantik daisyUI (bukan absolut).
// Mencakup seluruh 5 kind yang dibangun M7 (task/meeting/call/chat/note).
func activityKindBadge(kind string) g.Node {
	cls, label := "badge badge-ghost", activityKindLabel(kind)
	switch kind {
	case "task":
		cls = "badge badge-info"
	case "meeting":
		cls = "badge badge-secondary"
	case "call":
		cls = "badge badge-success"
	case "chat":
		cls = "badge badge-accent"
	case "note":
		cls = "badge badge-ghost"
	}
	return h.Span(h.Class(cls), g.Text(label))
}

// activityKindLabel memetakan enum kind → label Indonesia.
// Mencakup seluruh 5 kind M7 (email sengaja dikecualikan iterasi ini).
func activityKindLabel(kind string) string {
	switch kind {
	case "task":
		return "Tugas"
	case "meeting":
		return "Pertemuan"
	case "call":
		return "Panggilan"
	case "chat":
		return "Chat"
	case "note":
		return "Catatan"
	default:
		return kind
	}
}

// activityStatusBadge = badge status Task berwarna token semantik; status kosong
// (call/note) → teks "—" (tak ada daur status).
func activityStatusBadge(status string) g.Node {
	if status == "" {
		return h.Span(h.Class("text-base-content/50"), g.Text("—"))
	}
	cls := "badge badge-ghost"
	switch status {
	case "Completed":
		cls = "badge badge-success"
	case "In Progress":
		cls = "badge badge-info"
	case "Deferred":
		cls = "badge badge-warning"
	}
	return h.Span(h.Class(cls), g.Text(status))
}

// activityTargetLink = tautan ke halaman target (tipe + id, tanpa lookup nama →
// hindari N+1). Label = "Deal #123" / "Desa #45" / "Kontak #7".
func activityTargetLink(base, targetType string, id int64) g.Node {
	seg, label := activityTargetParts(targetType, id)
	if seg == "" {
		return h.Span(h.Class("text-base-content/50"), g.Text("—"))
	}
	return h.A(
		h.Href(base+"/"+seg+"/"+strconv.FormatInt(id, 10)),
		h.Class("link link-hover truncate block"), g.Text(label))
}

// activityTargetParts memetakan target_type → (segmen path, label tampil).
// Segmen kosong untuk tipe tak ber-halaman v1 (tak seharusnya terjadi lewat picker).
func activityTargetParts(targetType string, id int64) (seg, label string) {
	idStr := strconv.FormatInt(id, 10)
	switch targetType {
	case "deal":
		return "deals", "Deal #" + idStr
	case "account":
		return "accounts", "Desa #" + idStr
	case "contact":
		return "contacts", "Kontak #" + idStr
	default:
		return "", ""
	}
}

func emptyActivities(v ActivitiesListView) g.Node {
	// Pencarian tanpa hasil: pesan khusus + tautan reset (buang q, pertahankan target).
	if v.Query != "" {
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body items-start"),
				h.P(h.Class("text-base-content/70"),
					g.Text("Tak ada aktivitas yang cocok pencarian.")),
				h.A(h.Href(withQuery(v.Base+"/activities", "", hiddenField{"target", v.TargetFilter})),
					h.Class("btn btn-ghost btn-sm min-h-11"), g.Text("Reset pencarian")),
			),
		)
	}
	if v.NextCursor == "" {
		msg := "Belum ada aktivitas."
		if v.TargetFilter != "" {
			msg = "Belum ada aktivitas untuk entitas ini."
		}
		return h.Div(
			h.Class("card bg-base-100 border border-base-300"),
			h.Div(h.Class("card-body"),
				h.P(h.Class("text-base-content/70"), g.Text(msg))),
		)
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body items-start"),
			h.P(h.Class("text-base-content/70"),
				g.Text("Belum ada aktivitas pada tampilan ini.")),
			h.A(h.Href(v.Base+"/activities"), h.Class("btn btn-ghost btn-sm min-h-11"),
				g.Text("« Kembali ke awal")),
		),
	)
}

// activitiesPager = navigasi halaman berikutnya. Saat TargetFilter aktif,
// menyertakan ?target= agar halaman berikut tetap terfilter entitas yang sama.
func activitiesPager(v ActivitiesListView) g.Node {
	base := panelListHref(v.Base+"/activities", [2]string{"target", v.TargetFilter}, [2]string{"q", v.Query})
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}
