package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// sales_activities_form.go — form buat/sunting aktivitas. Murni-data.
//
// BL-19: SATU form dengan dropdown "Jenis" (create) menggantikan pola 3-form
// per-kind. Saat create, semua grup field per-kind dirender di DOM lalu di-toggle
// klien via Datastar (`data.Show("$kind == '…'")`), select "Jenis" di-bind ke
// signal $kind. Ini menukar prinsip lama "no-JS ramping" jadi show/hide Datastar
// — trade-off wajar demi satu tombol yang menskala saat kind tumbuh (M7). State
// form efemeral = ranah sah Datastar (unsafe-eval sudah aktif; ekspresi hanya
// literal enum internal, BUKAN input user & BUKAN manipulasi tabel). Backend TETAP
// penjaga: parseActivityForm hanya membaca field kind terpilih, field tersembunyi
// yang ikut terkirim diabaikan.
//
// kind IMMUTABLE saat EDIT: dibaca dari DB, ditampilkan read-only; ActivityUpdate
// memakai a.Kind (bukan form) → ganti jenis (mis. call→note) mustahil membuang
// data kind-spesifik. Satu picker Target polimorfik (nilai "type:id") saat create;
// target immutable saat edit (read-only). Native POST → 303 (gotcha #16). Reuse
// field/selectField/textareaField/memberSelect/formCard (accounts_form.go) & showWhen
// (sales_deals_detail.go).

// ActivityTargetOption = satu opsi picker target polimorfik. Value = "type:id"
// (mis. "deal:1"); Label = "Deal · Nama".
type ActivityTargetOption struct {
	Value string
	Label string
}

// ActivityFormView = data halaman form. Action = URL POST. IsEdit mengubah judul &
// mengunci target. Field per-kind diisi sesuai Kind; opsi enum dari handler.
type ActivityFormView struct {
	Base   string
	Action string
	IsEdit bool
	Err    string
	Kind   string   // kind aktif: nilai awal signal (create) / kind terkunci (edit)
	Kinds  []string // opsi dropdown "Jenis" (create-only); berurut, dari handler

	// Target (create): pilihan; (edit): TargetLabel read-only, TargetValue tetap.
	Targets     []ActivityTargetOption
	TargetValue string
	TargetLabel string

	Subject string

	// Task
	DueDate  string
	Priority string
	Status   string

	// Call / Chat
	ContactID  string
	Contacts   []AccountMemberOption
	Direction  string
	ActivityAt string
	Duration   string
	CallResult string
	Channel    string

	// Meeting
	StartAt     string
	EndAt       string
	Location    string
	MeetingType string

	// Note
	Body string

	// Bersama (task/call/meeting/chat)
	Notes string

	// Opsi enum (berurut) untuk dropdown
	Priorities      []string
	Statuses        []string
	MeetingStatuses []string
	MeetingTypes    []string
	Directions      []string
	CallResults     []string
	Channels        []string
}

// ActivityForm merender halaman form: buat (dropdown Jenis + field per-kind
// ter-toggle Datastar) atau sunting (kind terkunci, satu grup field).
func ActivityForm(v ActivityFormView) g.Node {
	title, submit := "Tambah Aktivitas", "Simpan"
	if v.IsEdit {
		title = "Sunting " + activityKindLabel(v.Kind)
		submit = "Simpan Perubahan"
	}

	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text(title)),
			h.A(h.Href(v.Base+"/activities"), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke daftar aktivitas")),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "activity-form-err", g.Text(v.Err)))
	}

	form := []g.Node{
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),
	}
	// Create: signal $kind menggerakkan show/hide grup field per-kind. Edit: tak
	// ada signal (kind terkunci), form ramping satu grup.
	if !v.IsEdit {
		form = append(form, data.Signals(map[string]any{"kind": v.Kind}))
	}
	form = append(form,
		formCard("Aktivitas",
			activityKindField(v),
			activityTargetField(v),
			field("Subjek", "subject", v.Subject, true, "text"),
		),
		activityKindFields(v),
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text(submit)),
			h.A(h.Href(v.Base+"/activities"), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	)
	body = append(body, h.FormEl(form...))

	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// activityKindField = pemilih "Jenis". Create: <select name="kind"> di-bind signal
// $kind (menggerakkan toggle field). Edit: teks read-only (input disabled → tak
// terkirim; ActivityUpdate memakai a.Kind, bukan form) → kind immutable. Label dari
// activityKindLabel agar tampilan Indonesia.
func activityKindField(v ActivityFormView) g.Node {
	if v.IsEdit {
		return h.Div(
			h.Class("grid gap-1 min-w-0"),
			labelFor("Jenis", "f-kind_ro", false),
			ui.Input(
				h.ID("f-kind_ro"), h.Type("text"), h.Value(activityKindLabel(v.Kind)),
				h.Disabled(), h.Class("input text-base w-full"),
			),
			h.P(h.Class("text-xs text-base-content/60"),
				g.Text("Jenis tak dapat diubah setelah aktivitas dibuat.")),
		)
	}
	nodes := make([]g.Node, 0, len(v.Kinds))
	for _, k := range v.Kinds {
		attrs := []g.Node{h.Value(k)}
		if k == v.Kind {
			attrs = append(attrs, h.Selected())
		}
		nodes = append(nodes, h.Option(append(attrs, g.Text(activityKindLabel(k)))...))
	}
	sel := []g.Node{
		h.ID("f-kind"), h.Name("kind"), h.Required(),
		data.Bind("kind"),
		h.Class("select text-base w-full"),
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelFor("Jenis", "f-kind", true),
		h.Select(append(sel, g.Group(nodes))...),
	)
}

// activityTargetField = picker target. Create: dropdown "type:id" wajib. Edit:
// target immutable → teks read-only (tanpa name → tak terkirim; handler juga
// tak membacanya saat update).
func activityTargetField(v ActivityFormView) g.Node {
	if v.IsEdit {
		return h.Div(
			h.Class("grid gap-1 min-w-0"),
			labelFor("Target", "f-target_ro", false),
			ui.Input(
				h.ID("f-target_ro"), h.Type("text"), h.Value(v.TargetLabel),
				h.Disabled(), h.Class("input text-base w-full"),
			),
			h.P(h.Class("text-xs text-base-content/60"),
				g.Text("Target tak dapat diubah setelah aktivitas dibuat.")),
		)
	}
	return activityTargetSelect("Target", "target", v.TargetValue, v.Targets)
}

// activityTargetSelect = dropdown target polimorfik. Nilai opsi = "type:id"
// (bukan numerik) → tak bisa pakai memberSelect. Wajib (opsi kosong penuntun,
// backend menolak bila tak sah).
func activityTargetSelect(label, name, current string, opts []ActivityTargetOption) g.Node {
	nodes := []g.Node{h.Option(h.Value(""), g.Text("— Pilih target —"))}
	for _, o := range opts {
		attrs := []g.Node{h.Value(o.Value)}
		if o.Value == current {
			attrs = append(attrs, h.Selected())
		}
		nodes = append(nodes, h.Option(append(attrs, g.Text(o.Label))...))
	}
	sel := []g.Node{
		h.ID("f-" + name), h.Name(name), h.Required(),
		h.Class("select text-base w-full"),
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelFor(label, "f-"+name, true),
		h.Select(append(sel, g.Group(nodes))...),
	)
}

// activityKindFields merender kartu field spesifik-kind. Edit: satu kartu (kind
// terkunci). Create: SEMUA kartu di DOM, tiap kartu di-toggle Datastar mengikuti
// $kind (showWhen) — field tersembunyi tetap terkirim tapi diabaikan backend untuk
// kind lain. Tak ada field per-kind yang `required` → HTML5 tak memblokir submit
// karena field tak terlihat.
func activityKindFields(v ActivityFormView) g.Node {
	if v.IsEdit {
		return activityKindFormCard(v, v.Kind)
	}
	return g.Group([]g.Node{
		showWhen("$kind == 'task'", "min-w-0", activityKindFormCard(v, "task")),
		showWhen("$kind == 'meeting'", "min-w-0", activityKindFormCard(v, "meeting")),
		showWhen("$kind == 'call'", "min-w-0", activityKindFormCard(v, "call")),
		showWhen("$kind == 'chat'", "min-w-0", activityKindFormCard(v, "chat")),
		showWhen("$kind == 'note'", "min-w-0", activityKindFormCard(v, "note")),
	})
}

// activityKindCard = kartu field untuk satu kind.
func activityKindFormCard(v ActivityFormView, kind string) g.Node {
	switch kind {
	case "task":
		return formCard("Detail Tugas",
			field("Jatuh Tempo", "due_date", v.DueDate, false, "date"),
			selectField("Prioritas", "priority", v.Priority, v.Priorities, false),
			selectField("Status", "status", v.Status, v.Statuses, false),
			textareaField("Catatan", "notes", v.Notes),
		)
	case "meeting":
		return formCard("Detail Pertemuan",
			selectField("Tipe", "meeting_type", v.MeetingType, v.MeetingTypes, false),
			field("Waktu Mulai", "start_at", v.StartAt, false, "datetime-local"),
			field("Waktu Selesai", "end_at", v.EndAt, false, "datetime-local"),
			field("Lokasi", "location", v.Location, false, "text"),
			selectField("Status", "status", v.Status, v.MeetingStatuses, false),
			textareaField("Catatan", "notes", v.Notes),
		)
	case "call":
		return formCard("Detail Panggilan",
			memberSelect("Kontak", "contact_id", v.ContactID, v.Contacts),
			selectField("Arah", "direction", v.Direction, v.Directions, false),
			field("Waktu", "activity_at", v.ActivityAt, false, "datetime-local"),
			field("Durasi (menit)", "duration_min", v.Duration, false, "number"),
			selectField("Hasil", "call_result", v.CallResult, v.CallResults, false),
			textareaField("Catatan", "notes", v.Notes),
		)
	case "chat":
		return formCard("Detail Chat",
			memberSelect("Kontak", "contact_id", v.ContactID, v.Contacts),
			selectField("Arah", "direction", v.Direction, v.Directions, false),
			selectField("Kanal", "channel", v.Channel, v.Channels, false),
			field("Waktu", "activity_at", v.ActivityAt, false, "datetime-local"),
			textareaField("Catatan", "notes", v.Notes),
		)
	case "note":
		return formCard("Catatan",
			textareaField("Isi", "body", v.Body),
		)
	default:
		return g.Text("")
	}
}
