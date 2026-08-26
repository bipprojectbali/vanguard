package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// engagements_form.go — form buat engagement baru (Modul 6 Customer Success,
// slice 6.5). Murni-data: opsi dropdown (akun, anggota) sudah dirakit handler.
// Native POST → 303 (gotcha #16). Meniru tickets_form.go.

// EngagementFormView = data halaman form buat engagement baru. Action = URL POST
// tujuan. Accounts/Members/Types/Statuses/Channels dioper handler.
type EngagementFormView struct {
	Base     string
	Action   string
	Err      string
	Accounts []EngagementAccountOption
	Members  []EngagementMemberOption
	Types    []string // engagement_type values (kode DB)
	Statuses []string // status values (kode DB) — v1 hanya 'planned' di create
	Channels []string // channel values (kode DB)
}

// EngagementForm merender form buat engagement baru.
func EngagementForm(v EngagementFormView) g.Node {
	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text("Buat Engagement Baru")),
			h.A(h.Href(v.Base+"/engagements"), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke daftar engagement")),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "engagement-form-err", g.Text(v.Err)))
	}
	body = append(body, h.FormEl(
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),
		formCard("Informasi Engagement",
			engagementAccountSelect(v.Accounts),
			field("Subjek / Topik", "subject", "", true, "text"),
			engagementTypeSelect(v.Types),
			engagementChannelSelect(v.Channels),
		),
		formCard("Jadwal",
			engagementDateTimeField("Jadwal Interaksi", "scheduled_at", ""),
			engagementFrequencySelect(),
			engagementDateField("Next Due Date (opsional)", "next_due_date", ""),
		),
		formCard("Catatan & Penugasan",
			textareaField("Outcome / Ringkasan (opsional)", "outcome", ""),
			engagementMemberSelect(v.Members),
		),
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text("Buat Engagement")),
			h.A(h.Href(v.Base+"/engagements"), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// engagementAccountSelect = dropdown pilih desa (wajib). Scope sudah difilter
// handler (CSM melihat desanya; Admin/Manager melihat semua).
func engagementAccountSelect(accounts []EngagementAccountOption) g.Node {
	opts := make([]g.Node, 0, len(accounts)+1)
	opts = append(opts, h.Option(h.Value(""), g.Text("— Pilih desa —")))
	for _, a := range accounts {
		opts = append(opts,
			h.Option(h.Value(strconv.FormatInt(a.ID, 10)), g.Text(a.Name)),
		)
	}
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		ui.Label("Desa", h.For("account_id")),
		h.Select(
			h.ID("account_id"), h.Name("account_id"), h.Required(),
			h.Class("select text-base w-full"),
			g.Group(opts),
		),
	)
}

// engagementTypeSelect = dropdown tipe engagement (wajib). Label UI dihasilkan
// dari kode DB (touch_point → "Touch Point", dst.).
func engagementTypeSelect(types []string) g.Node {
	opts := make([]g.Node, 0, len(types))
	for _, t := range types {
		opts = append(opts,
			h.Option(h.Value(t), g.Text(engagementTypeLabelUI(t))),
		)
	}
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		ui.Label("Tipe Engagement", h.For("engagement_type")),
		h.Select(
			h.ID("engagement_type"), h.Name("engagement_type"), h.Required(),
			h.Class("select text-base w-full"),
			g.Group(opts),
		),
	)
}

// engagementChannelSelect = dropdown channel komunikasi (opsional).
func engagementChannelSelect(channels []string) g.Node {
	opts := make([]g.Node, 0, len(channels)+1)
	opts = append(opts, h.Option(h.Value(""), g.Text("— Pilih channel (opsional) —")))
	for _, c := range channels {
		opts = append(opts,
			h.Option(h.Value(c), g.Text(engagementChannelLabelUI(c))),
		)
	}
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		ui.Label("Channel", h.For("channel")),
		h.Select(
			h.ID("channel"), h.Name("channel"),
			h.Class("select text-base w-full"),
			g.Group(opts),
		),
	)
}

// engagementFrequencySelect = dropdown frekuensi engagement (opsional).
func engagementFrequencySelect() g.Node {
	freqs := []struct{ val, label string }{
		{"weekly", "Weekly"},
		{"monthly", "Monthly"},
		{"quarterly", "Quarterly"},
		{"ad_hoc", "Ad-hoc"},
	}
	opts := make([]g.Node, 0, len(freqs)+1)
	opts = append(opts, h.Option(h.Value(""), g.Text("— Pilih frekuensi (opsional) —")))
	for _, f := range freqs {
		opts = append(opts, h.Option(h.Value(f.val), g.Text(f.label)))
	}
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		ui.Label("Frekuensi", h.For("frequency")),
		h.Select(
			h.ID("frequency"), h.Name("frequency"),
			h.Class("select text-base w-full"),
			g.Group(opts),
		),
	)
}

// engagementDateTimeField = <input type="datetime-local"> untuk scheduled_at.
func engagementDateTimeField(label, name, val string) g.Node {
	attrs := []g.Node{
		h.Type("datetime-local"), h.ID(name), h.Name(name),
		h.Class("input w-full text-base"),
		h.Required(),
	}
	if val != "" {
		attrs = append(attrs, h.Value(val))
	}
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		ui.Label(label, h.For(name)),
		h.Input(attrs...),
	)
}

// engagementDateField = <input type="date"> untuk next_due_date (opsional).
func engagementDateField(label, name, val string) g.Node {
	attrs := []g.Node{
		h.Type("date"), h.ID(name), h.Name(name),
		h.Class("input w-full text-base"),
	}
	if val != "" {
		attrs = append(attrs, h.Value(val))
	}
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		ui.Label(label, h.For(name)),
		h.Input(attrs...),
	)
}

// engagementMemberSelect = dropdown owner / CSM pelaksana (opsional).
func engagementMemberSelect(members []EngagementMemberOption) g.Node {
	opts := make([]g.Node, 0, len(members)+1)
	opts = append(opts, h.Option(h.Value(""), g.Text("— Belum ditugaskan —")))
	for _, m := range members {
		opts = append(opts,
			h.Option(h.Value(strconv.FormatInt(m.ID, 10)), g.Text(m.Name)),
		)
	}
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		ui.Label("Owner / CS Pelaksana", h.For("owner_id")),
		h.Select(
			h.ID("owner_id"), h.Name("owner_id"),
			h.Class("select text-base w-full"),
			g.Group(opts),
		),
	)
}

// engagementTypeLabelUI memetakan kode DB → label tampilan. Didefinisikan di
// sini agar view tidak bergantung ke paket handler. Sinkron dengan
// engagementTypeLabel di engagements_helpers.go (handler).
func engagementTypeLabelUI(t string) string {
	switch t {
	case "touch_point":
		return "Touch Point"
	case "qbr":
		return "QBR"
	case "onboarding_call":
		return "Onboarding Call"
	case "escalation":
		return "Escalation"
	case "check_in":
		return "Check-in"
	default:
		return t
	}
}

// engagementChannelLabelUI memetakan kode DB → label tampilan.
func engagementChannelLabelUI(c string) string {
	switch c {
	case "whatsapp":
		return "WhatsApp"
	case "call":
		return "Call"
	case "video":
		return "Video"
	case "site_visit":
		return "Site Visit"
	default:
		return c
	}
}
