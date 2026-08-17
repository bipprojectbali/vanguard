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
type ActivityTimelineItem struct {
	ID      int64
	Kind    string // task/meeting/call/chat/note
	Subject string
	Date    string // created_at lokal, e.g. "8 Agu 2026"
	Status  string // "" untuk kind tanpa status (call/chat/note)
	Owner   string // nama pemilik, "" → "—"
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

	header := h.Div(
		h.Class("flex flex-wrap items-center justify-between gap-2 mb-3"),
		h.H2(h.Class("font-semibold"), g.Text("Aktivitas")),
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

// activityTimelineRow = satu baris: kind badge | subject + tanggal·owner | status badge.
func activityTimelineRow(item ActivityTimelineItem) g.Node {
	meta := item.Date
	if item.Owner != "" {
		meta += " · " + item.Owner
	}
	return h.Div(
		h.Class("flex flex-wrap items-start gap-2 py-2 border-b border-base-300/50 last:border-0 min-w-0"),
		// Kind badge — shrink-0 agar tidak menyusut saat subject panjang.
		h.Div(h.Class("shrink-0 pt-0.5"), activityKindBadge(item.Kind)),
		// Subject + meta (tanggal · owner).
		h.Div(
			h.Class("flex-1 min-w-0"),
			h.P(h.Class("text-sm font-medium truncate"), g.Text(item.Subject)),
			h.P(h.Class("text-xs text-base-content/60 mt-0.5"), g.Text(meta)),
		),
		// Status badge — shrink-0, hanya relevan untuk kind "task" dan "meeting".
		h.Div(h.Class("shrink-0"), activityStatusBadge(item.Status)),
	)
}
