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
//
// Field-level helper (picker Jenis/Target, toggle kartu per-kind, field Kontak
// polimorfik, kartu field per-kind) di sales_activities_form_fields.go —
// dipisah krn ambang File Health (View/Component 300 baris).

// ActivityTargetOption = satu opsi picker target polimorfik. Value = "type:id"
// (mis. "deal:1"); Label = "Deal · Nama".
type ActivityTargetOption struct {
	Value string
	Label string
}

// LeadContactInfoView = info kontak mentah Lead (BL-164), ditampilkan read-only
// menggantikan dropdown Kontak saat Target = Lead. Field kosong ("") = data
// tak diisi di Lead — ditampilkan sebagai "—" (lihat contactFieldNode).
type LeadContactInfoView struct {
	Name     string
	Phone    string
	WhatsApp string
	Email    string
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

	// LeadContactInfo (BL-164): terisi HANYA saat Target = Lead — Lead tak
	// punya relasi ke tabel contacts, jadi field Kontak diganti info mentah
	// lead ini (read-only) alih-alih dropdown. nil = Target bukan Lead → field
	// Kontak dropdown biasa dirender (lihat contactFieldNode).
	LeadContactInfo *LeadContactInfoView

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
		body = append(body, ui.Toast(ui.VariantDestructive, "activity-form-err", g.Text(v.Err)))
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

	body = append(body, h.Script(h.Src("/static/accountpicker.js"), h.Defer()))
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}
