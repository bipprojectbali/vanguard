package panel

import (
	"fmt"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// activities_timeline.go — kartu timeline aktivitas reusable, di-embed di halaman
// detail entitas (Account, Deal, Contact, Ticket). Murni-data: semua nilai sudah
// diputuskan handler. Reuse activityKindBadge/activityStatusBadge/activityKindLabel
// dari sales_activities.go (paket sama).
//
// Komponen ini TIDAK memuat data sendiri — handler detail masing-masing entitas
// yang memanggil activitiesTimelineFor dan mengoper hasilnya ke struct detail.
// Gate izin DI HANDLER entitas (bukan di sini): siapa pun yang boleh lihat
// detail entitasnya mendapat timeline-nya.

// ActivityTimelineItem = satu baris timeline aktivitas untuk tampilan kartu.
// Semua nilai sudah diformat handler: Date = waktu lokal (gotcha #14), Owner = nama.
//
// BL-31: field Source/TypeLabel/StatusBadgeClass adalah tambahan OPSIONAL untuk
// linimasa terpadu detail Account (gabung activities Sales + engagements CS).
// Semuanya "" secara default → baris dari detail Deal/Contact (murni activities)
// merender persis seperti sebelumnya, tanpa penanda sumber.
type ActivityTimelineItem struct {
	ID      int64
	Kind    string // task/meeting/call/chat/note
	Subject string
	Date    string // created_at lokal, e.g. "8 Agu 2026"
	Status  string // "" untuk kind tanpa status (call/chat/note)
	Owner   string // nama pemilik, "" → "—"

	// Source = penanda sumber baris pada linimasa terpadu: "" (default, tak ada
	// penanda) · "sales" (dari activities) · "cs" (dari engagements). Bila terisi,
	// baris menampilkan chip Sales/CS di meta.
	Source string
	// TypeLabel = label jenis untuk baris SUMBER CS (engagement_type sudah
	// dilabeli handler, mis. "QBR"). Bila terisi, badge kiri memakai teks ini
	// (baris engagement tak punya "kind" activity). "" untuk baris activity.
	TypeLabel string
	// StatusBadgeClass = kelas daisyUI badge status yang sudah diputuskan handler
	// (mis. "badge badge-success"). Dipakai baris CS karena peta statusnya beda
	// dari activity. "" → baris activity pakai activityStatusBadge(Status).
	StatusBadgeClass string
}

// ActivityTimelineView = data kartu timeline untuk satu entitas target.
// TargetType + TargetID dipakai merakit link "Log Aktivitas" pre-filled.
type ActivityTimelineView struct {
	Base       string // prefix URL workspace, e.g. "/w/desa-plus"
	TargetType string // "account" / "deal" / "contact" / "ticket"
	TargetID   int64
	CanWrite   bool
	Items      []ActivityTimelineItem
	NextCursor string // "" = ujung daftar (tak ada pager)
	// Title = judul kartu; "" → "Aktivitas" (default Deal/Contact). Detail
	// Account memakainya untuk "Linimasa" karena kartunya menggabungkan sumber
	// Sales + CS (BL-31), bukan activities murni.
	Title string
}

// ActivityTimeline merender kartu timeline aktivitas.
// - Header: judul "Aktivitas" + tombol "+ Log Aktivitas" (bila CanWrite).
// - Body: daftar item atau pesan kosong.
// - Footer: tautan "Lihat semua" bila masih ada halaman berikutnya (terfilter per-entitas).
func ActivityTimeline(v ActivityTimelineView) g.Node {
	newLink := fmt.Sprintf("%s/activities/new?target=%s:%d",
		v.Base, v.TargetType, v.TargetID)
	// allLink = daftar aktivitas ter-filter entitas ini (?target=type:id).
	// Membawa pengguna ke /activities dengan hanya baris entitas ini, bukan semua.
	allLink := fmt.Sprintf("%s/activities?target=%s:%d",
		v.Base, v.TargetType, v.TargetID)

	title := v.Title
	if title == "" {
		title = "Aktivitas"
	}
	header := h.Div(
		h.Class("flex flex-wrap items-center justify-between gap-2 mb-3"),
		h.H2(h.Class("font-semibold"), g.Text(title)),
		ui.When(v.CanWrite, h.A(
			h.Href(newLink),
			h.Class("btn btn-sm btn-ghost min-h-9"),
			g.Text("+ Log Aktivitas"),
		)),
	)

	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			header,
			activityTimelineBody(v, allLink),
		),
	)
}

// activityTimelineBody = daftar item bila ada, keadaan kosong bila tidak.
func activityTimelineBody(v ActivityTimelineView, allLink string) g.Node {
	if len(v.Items) == 0 {
		return h.Div(
			h.Class("py-6 text-center text-sm text-base-content/50"),
			g.Text("Belum ada aktivitas tercatat untuk entitas ini."),
		)
	}
	rows := make([]g.Node, 0, len(v.Items)+1)
	for _, item := range v.Items {
		rows = append(rows, activityTimelineRow(item))
	}
	if v.NextCursor != "" {
		rows = append(rows, h.Div(
			h.Class("pt-3 text-center"),
			h.A(
				h.Href(allLink),
				h.Class("text-sm link link-primary"),
				g.Text("Lihat semua aktivitas →"),
			),
		))
	}
	return g.Group(rows)
}

// activityTimelineRow = satu baris: badge kiri | subject + meta | status badge.
// Baris activity (Source="") tampil persis seperti semula. Baris terpadu BL-31
// (Source terisi) menambah chip sumber Sales/CS di meta; baris CS (TypeLabel
// terisi) memakai badge jenis engagement + status badge dari StatusBadgeClass.
func activityTimelineRow(item ActivityTimelineItem) g.Node {
	// meta = [chip sumber] tanggal · [jenis engagement] · owner.
	meta := item.Date
	if item.TypeLabel != "" {
		meta += " · " + item.TypeLabel
	}
	if item.Owner != "" {
		meta += " · " + item.Owner
	}

	// Badge kiri: baris CS pakai label jenis (engagement tak punya "kind"),
	// baris activity pakai activityKindBadge seperti biasa.
	leftBadge := activityKindBadge(item.Kind)
	if item.TypeLabel != "" {
		leftBadge = h.Span(h.Class("badge badge-secondary"), g.Text(item.TypeLabel))
	}

	// Status badge: baris CS pakai kelas yang sudah diputuskan handler,
	// baris activity pakai peta status activity.
	statusBadge := activityStatusBadge(item.Status)
	if item.StatusBadgeClass != "" {
		statusBadge = h.Span(h.Class(item.StatusBadgeClass), g.Text(item.Status))
	}

	return h.Div(
		h.Class("flex flex-wrap items-start gap-2 py-2 border-b border-base-300/50 last:border-0 min-w-0"),
		// Badge kiri — shrink-0 agar tidak menyusut saat subject panjang.
		h.Div(h.Class("shrink-0 pt-0.5"), leftBadge),
		// Subject + meta (chip sumber · tanggal · jenis · owner).
		h.Div(
			h.Class("flex-1 min-w-0"),
			h.P(h.Class("text-sm font-medium truncate"), g.Text(item.Subject)),
			h.P(
				h.Class("flex flex-wrap items-center gap-1.5 text-xs text-base-content/60 mt-0.5"),
				timelineSourceChip(item.Source),
				g.Text(meta),
			),
		),
		// Status badge — shrink-0.
		h.Div(h.Class("shrink-0"), statusBadge),
	)
}

// timelineSourceChip merender chip sumber Sales/CS untuk linimasa terpadu
// (BL-31). Source="" → tak ada chip (baris detail Deal/Contact tak berubah).
// Token semantik daisyUI: Sales=primary, CS=secondary (gotcha #4/#11 — jangan
// warna absolut).
func timelineSourceChip(source string) g.Node {
	switch source {
	case "sales":
		return h.Span(h.Class("badge badge-xs badge-primary badge-outline shrink-0"), g.Text("Sales"))
	case "cs":
		return h.Span(h.Class("badge badge-xs badge-secondary badge-outline shrink-0"), g.Text("CS"))
	default:
		return g.Text("")
	}
}
