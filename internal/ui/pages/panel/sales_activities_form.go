package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_activities_form.go — form buat/sunting aktivitas. Murni-data. kind TETAP
// per form (immutable): create memilih lewat ?kind=, edit membaca dari DB → tiap
// form ramping & no-JS (tak ada switch field dinamis). Satu picker Target
// polimorfik (nilai "type:id") saat create; target immutable saat edit (ditampilkan
// read-only). Native POST → 303 (gotcha #16). Reuse field/selectField/textareaField/
// memberSelect/formCard (accounts_form.go).

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
	Kind   string

	// Target (create): pilihan; (edit): TargetLabel read-only, TargetValue tetap.
	Targets     []ActivityTargetOption
	TargetValue string
	TargetLabel string

	Subject string

	// Task
	DueDate  string
	Priority string
	Status   string

	// Call
	ContactID string
	Contacts  []AccountMemberOption
	Direction string
	ActivityAt string
	Duration   string
	CallResult string

	// Note
	Body string

	// Bersama (task/call)
	Notes string

	// Opsi enum (berurut) untuk dropdown
	Priorities  []string
	Statuses    []string
	Directions  []string
	CallResults []string
}

// ActivityForm merender halaman form lengkap untuk satu kind.
func ActivityForm(v ActivityFormView) g.Node {
	title := "Catat " + activityKindLabel(v.Kind)
	submit := "Simpan"
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

	body = append(body, h.FormEl(
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),

		// kind dibawa eksplisit agar create tahu cabang (immutable setelahnya).
		h.Input(h.Type("hidden"), h.Name("kind"), h.Value(v.Kind)),

		formCard("Aktivitas",
			activityTargetField(v),
			field("Subjek", "subject", v.Subject, true, "text"),
		),
		activityKindFields(v),

		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text(submit)),
			h.A(h.Href(v.Base+"/activities"), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))

	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
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

// activityKindFields = kartu field spesifik per kind.
func activityKindFields(v ActivityFormView) g.Node {
	switch v.Kind {
	case "task":
		return formCard("Detail Tugas",
			field("Jatuh Tempo", "due_date", v.DueDate, false, "date"),
			selectField("Prioritas", "priority", v.Priority, v.Priorities, false),
			selectField("Status", "status", v.Status, v.Statuses, false),
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
	case "note":
		return formCard("Catatan",
			textareaField("Isi", "body", v.Body),
		)
	default:
		return g.Text("")
	}
}
