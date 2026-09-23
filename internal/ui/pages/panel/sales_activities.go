package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_activities.go — view daftar Sales Activity Log (4.4): tabel berkeyset
// aktivitas (task/call/note) bercakupan F3. Murni-data: Owner sudah nama, Created
// sudah diformat handler. Target dirender TIPE + tautan by-id (bukan nama) untuk
// hindari N+1 (rule 13) — nama target diresolusi hanya di detail. Meniru
// sales_deals.go tampilan Tabel.
//
// Row-rendering & badge helper (baris tabel, badge jenis/status, tautan
// target) di sales_activities_row.go — dipisah krn ambang File Health
// (View/Component 300 baris).

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

	// ── Baris SUMBER CS (engagement) di feed Activities global (BL-41) ──
	// Source = penanda sumber baris: "" (default = activity/Sales, render tak
	// berubah) · "cs" (dari engagements). Bila "cs", baris di-render read-only
	// (tautan ke desa induk, bukan /activities/{id}) dan kolom Jenis/Status
	// memakai peta engagement, bukan peta activity.
	Source string
	// TypeLabel = label jenis untuk baris CS (engagement_type sudah dilabeli
	// handler, mis. "QBR"). Dipakai badge kolom "Jenis" karena engagement tak
	// punya "kind" activity. "" untuk baris activity.
	TypeLabel string
	// StatusBadgeClass = kelas daisyUI badge status yang sudah diputuskan handler
	// untuk baris CS (peta status engagement beda dari activity). "" → baris
	// activity pakai activityStatusBadge(Status).
	StatusBadgeClass string
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
	Sort         string // BL-157j: kolom sort aktif ("" = default created_at DESC)
	Dir          string // "asc"/"desc"; kosong hanya saat Sort kosong
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
			ui.When(v.CanWrite, activityNewButton(v.Base, v.TargetFilter, false)),
		),
		searchBox(v.Base+"/activities", v.Query, "Cari aktivitas — subjek…", "Cari aktivitas",
			hiddenField{"target", v.TargetFilter},
			hiddenField{"sort", v.Sort}, hiddenField{"dir", v.Dir}),
	}
	if v.Err != "" {
		body = append(body, ui.Toast(ui.VariantDestructive, "act-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Toast(ui.VariantSuccess, "act-ok", g.Text(v.Msg)))
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
//
// fromLog (BL-161 lanjutan): true dari AllActivitiesList (feed lintas-context
// /activity-log) → tautan menyertakan "?from=log", dibaca
// activityCurrentPathFromQuery (handler) agar sidebar form ActivityNew tetap
// menyala "Activities", bukan "Sales Activities". ActivitiesList (halaman Sales
// Activities sendiri) kirim false — perilaku lama tetap benar di sana.
func activityNewButton(base, targetFilter string, fromLog bool) g.Node {
	href := base + "/activities/new"
	sep := "?"
	if targetFilter != "" {
		href += sep + "target=" + targetFilter
		sep = "&"
	}
	if fromLog {
		href += sep + "from=log"
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
					h.Th(h.Class("py-2 pr-4 font-medium"), activitySortHeader(v, "kind", "Jenis")),
					h.Th(h.Class("py-2 pr-4 font-medium"), activitySortHeader(v, "subject", "Subjek")),
					h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Target")),
					h.Th(h.Class("py-2 pr-4 font-medium"), activitySortHeader(v, "owner", "Pemilik")),
					h.Th(h.Class("py-2 pr-4 font-medium"), activitySortHeader(v, "status", "Status")),
					h.Th(h.Class("py-2 font-medium"), activitySortHeader(v, "date", "Tanggal")),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

// activitySortHeader = header tabel yang bisa diklik untuk urut per kolom
// (BL-157j), mirror dealSortHeader (sales_deals.go). Nonaktif (teks polos)
// saat TargetFilter aktif — mode filter per-entitas pakai
// ListActivitiesByTarget, yang tak punya varian sort.
func activitySortHeader(v ActivitiesListView, col, label string) g.Node {
	if v.TargetFilter != "" {
		return g.Text(label)
	}
	active := v.Sort == col
	nextDir := "asc"
	if active && v.Dir == "asc" {
		nextDir = "desc"
	}
	href := withQuery(v.Base+"/activities", v.Query,
		hiddenField{"target", v.TargetFilter},
		hiddenField{"sort", col}, hiddenField{"dir", nextDir})
	return sortHeaderLink(href, label, active, v.Dir)
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
	base := panelListHref(v.Base+"/activities", [2]string{"target", v.TargetFilter}, [2]string{"q", v.Query},
		[2]string{"sort", v.Sort}, [2]string{"dir", v.Dir})
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}
