package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// sales_activities_form_fields.go — field-level helper untuk ActivityForm
// (sales_activities_form.go: picker Jenis/Target, toggle kartu per-kind,
// field Kontak polimorfik, kartu field per-kind). Dipisah krn ambang File
// Health (View/Component 300 baris); murni pindah, tak ada perubahan logika.

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
	opts := make([]TypeaheadOption, 0, len(v.Targets))
	for _, t := range v.Targets {
		opts = append(opts, TypeaheadOption{Value: t.Value, Label: t.Label})
	}
	return typeaheadPickerField(typeaheadPickerConfig{
		Name:        "target",
		Label:       "Target",
		ListID:      "target-options",
		Options:     opts,
		Current:     v.TargetValue,
		Required:    true,
		Placeholder: "Ketik untuk mencari deal, desa, kontak, atau lead (nama/kode)…",
		InvalidMsg:  "Pilih target dari daftar.",
		// BL-164: reload SSE opsi Kontak saat Target berubah (create-mode saja —
		// edit-mode target immutable, cabang di atas tak pernah sampai sini).
		TriggerURL: v.Base + "/activities/contact-options",
	})
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

// contactFieldNode (BL-164) merender field "Kontak" pada kartu Call/Chat:
// dropdown biasa (memberSelect, accounts_form.go) bila v.LeadContactInfo nil,
// atau blok info Lead read-only bila Target = Lead (Lead tak punya relasi ke
// tabel contacts). Dibungkus <div id=wrapID> agar SSE PatchElements (reload
// dinamis saat Target berubah) bisa menarget kartu Call & Chat SECARA TERPISAH
// — wrapID WAJIB unik per kartu (keduanya dirender bersamaan di mode create).
//
// sm:col-span-2 KHUSUS varian info Lead (dilaporkan user: field "Arah" di
// sebelahnya melar/misalign): wrapper ini duduk sebagai ITEM LANGSUNG grid
// 2-kolom formCard (grid gap-3 sm:grid-cols-2), dan blok info Lead jauh lebih
// tinggi dari field biasa — align-items:stretch bawaan grid memaksa sel
// tetangga (mis. "Arah") ikut setinggi itu. Melebarkan blok Lead ke 2 kolom
// membuatnya jadi satu-satunya penghuni barisnya → tak ada lagi sel tetangga
// yang ikut melar. Varian dropdown (memberSelect) TETAP 1 kolom seperti field
// lain — tak berubah.
func contactFieldNode(wrapID string, v ActivityFormView) g.Node {
	attrs := []g.Node{h.ID(wrapID)}
	var inner g.Node
	if v.LeadContactInfo != nil {
		inner = leadContactInfoBlock(*v.LeadContactInfo)
		attrs = append(attrs, h.Class("sm:col-span-2"))
	} else {
		inner = memberSelect("Kontak", "contact_id", v.ContactID, v.Contacts)
	}
	return h.Div(append(attrs, inner)...)
}

// ContactFieldFragment = versi diekspor contactFieldNode, dipanggil lintas-paket
// dari internal/handler (sales_activities_contact_options.go) untuk merender
// fragmen SSE saat Target berubah.
func ContactFieldFragment(wrapID string, v ActivityFormView) g.Node {
	return contactFieldNode(wrapID, v)
}

// leadContactInfoBlock merender info kontak mentah Lead sebagai kartu
// read-only, menggantikan dropdown Kontak. Field kosong → "—".
func leadContactInfoBlock(info LeadContactInfoView) g.Node {
	row := func(label, value string) g.Node {
		if value == "" {
			value = "—"
		}
		return h.Div(
			h.Class("flex justify-between gap-2 text-sm min-w-0"),
			h.Span(h.Class("text-base-content/60"), g.Text(label)),
			h.Span(h.Class("font-medium text-right break-words"), g.Text(value)),
		)
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		h.Div(h.Class("text-sm text-base-content/70"), g.Text("Kontak")),
		h.Div(
			h.Class("card bg-base-200 p-3 grid gap-1 min-w-0"),
			row("Nama", info.Name),
			row("HP", info.Phone),
			row("WhatsApp", info.WhatsApp),
			row("Email", info.Email),
		),
		h.P(h.Class("text-xs text-base-content/60"),
			g.Text("Lead belum terhubung ke kontak sistem — info di atas diambil langsung dari data lead.")),
	)
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
			contactFieldNode("contact-field-call", v),
			selectField("Arah", "direction", v.Direction, v.Directions, false),
			field("Waktu", "activity_at", v.ActivityAt, false, "datetime-local"),
			field("Durasi (menit)", "duration_min", v.Duration, false, "number"),
			selectField("Hasil", "call_result", v.CallResult, v.CallResults, false),
			textareaField("Catatan", "notes", v.Notes),
		)
	case "chat":
		return formCard("Detail Chat",
			contactFieldNode("contact-field-chat", v),
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
