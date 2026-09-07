package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// tickets_form.go — form buat tiket baru (Modul 6 Customer Success, slice B2).
// Murni-data: opsi dropdown (akun, SLA policy, agen) sudah dirakit handler.
// Native POST → 303 (gotcha #16). Meniru kb_articles.go.

// TicketAccountOption = satu pilihan dropdown desa.
type TicketAccountOption struct {
	ID   int64
	Name string
}

// TicketMemberOption = satu pilihan dropdown agen (anggota workspace).
type TicketMemberOption struct {
	ID   int64
	Name string
}

// TicketSLAOption = satu pilihan dropdown SLA policy.
type TicketSLAOption struct {
	ID   int64
	Name string
}

// TicketFormView = data halaman form buat tiket baru. Action = URL POST tujuan.
// Accounts/Members/SLAs dioper handler (view tak query DB). Priorities dioper
// handler (ticketPriorityValues dari tickets_helpers.go).
type TicketFormView struct {
	Base        string
	Action      string
	Err         string
	Accounts    []TicketAccountOption
	Members     []TicketMemberOption
	SLAPolicies []TicketSLAOption
	Priorities  []string
}

// TicketForm merender form buat tiket baru.
func TicketForm(v TicketFormView) g.Node {
	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text("Buat Tiket Baru")),
			h.A(h.Href(v.Base+"/tickets"), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke daftar tiket")),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "ticket-form-err", g.Text(v.Err)))
	}
	body = append(body, h.FormEl(
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),
		formCard("Informasi Tiket",
			ticketAccountSelect(v.Accounts),
			field("Subjek", "subject", "", true, "text"),
			ticketSelectField("Prioritas", "priority", "sedang", v.Priorities, true),
			textareaField("Deskripsi (opsional)", "description", ""),
		),
		formCard("Penugasan & SLA",
			ticketMemberSelect(v.Members),
			ticketSLASelect(v.SLAPolicies),
		),
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text("Buat Tiket")),
			h.A(h.Href(v.Base+"/tickets"), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))
	body = append(body, h.Script(h.Src("/static/accountpicker.js"), h.Defer()))
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// ticketAccountSelect = dropdown pilih desa (wajib). Semua desa aktif dioper
// handler tanpa filter scope (ticket writer bisa membuat tiket untuk desa apapun).
func ticketAccountSelect(accounts []TicketAccountOption) g.Node {
	opts := make([]TypeaheadOption, 0, len(accounts))
	for _, a := range accounts {
		opts = append(opts, TypeaheadOption{
			Value: strconv.FormatInt(a.ID, 10),
			Label: a.Name,
		})
	}
	return accountTypeaheadField("account_id", "Desa", opts, true, false)
}

// ticketMemberSelect = dropdown pilih agen (opsional). "" = belum ditugaskan.
func ticketMemberSelect(members []TicketMemberOption) g.Node {
	opts := make([]g.Node, 0, len(members)+1)
	opts = append(opts, h.Option(h.Value(""), g.Text("— Belum ditugaskan —")))
	for _, m := range members {
		opts = append(opts,
			h.Option(h.Value(strconv.FormatInt(m.ID, 10)), g.Text(m.Name)),
		)
	}
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		ui.Label("Ditugaskan Kepada", h.For("assigned_to")),
		h.Select(
			h.ID("assigned_to"), h.Name("assigned_to"),
			h.Class("select w-full"),
			g.Group(opts),
		),
	)
}

// ticketSLASelect = dropdown pilih SLA policy (opsional). "" = tanpa SLA.
func ticketSLASelect(policies []TicketSLAOption) g.Node {
	opts := make([]g.Node, 0, len(policies)+1)
	opts = append(opts, h.Option(h.Value(""), g.Text("— Tanpa SLA —")))
	for _, p := range policies {
		opts = append(opts,
			h.Option(h.Value(strconv.FormatInt(p.ID, 10)), g.Text(p.Name)),
		)
	}
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		ui.Label("Kebijakan SLA", h.For("sla_policy_id")),
		h.Select(
			h.ID("sla_policy_id"), h.Name("sla_policy_id"),
			h.Class("select w-full"),
			g.Group(opts),
		),
	)
}

// ticketSelectField = dropdown pilih prioritas (punya opsi default terpilih).
func ticketSelectField(label, name, current string, opts []string, required bool) g.Node {
	options := make([]g.Node, 0, len(opts))
	for _, o := range opts {
		attrs := []g.Node{h.Value(o), g.Text(ticketPriorityLabel(o))}
		if o == current {
			attrs = append(attrs, h.Selected())
		}
		options = append(options, h.Option(attrs...))
	}
	sel := []g.Node{
		h.ID(name), h.Name(name), h.Class("select w-full"),
		g.Group(options),
	}
	if required {
		sel = append(sel, h.Required())
	}
	return h.Div(
		h.Class("grid gap-2 min-w-0"),
		ui.Label(label, h.For(name)),
		h.Select(sel...),
	)
}

// ticketPriorityLabel = label Bahasa Indonesia untuk nilai enum priority.
func ticketPriorityLabel(priority string) string {
	switch priority {
	case "tinggi":
		return "Tinggi"
	case "sedang":
		return "Sedang"
	case "rendah":
		return "Rendah"
	default:
		return priority
	}
}
