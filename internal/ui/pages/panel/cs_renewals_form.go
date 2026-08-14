package panel

import (
	"fmt"
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// cs_renewals_form.go — form edit AKSI CS pada satu langganan (Modul 6 slice 6.6).
//
// View murni-data: tidak ada import db/authz/session.
// Handler memetakan db.Subscription + db.Account → CSRenewalFormView.

// CSRenewalMemberOption — satu pilihan dropdown Owner CSM.
type CSRenewalMemberOption struct {
	ID   int64
	Name string
}

// CSRenewalFormView — data lengkap form edit renewal CS.
type CSRenewalFormView struct {
	Base   string // "/w/{slug}"
	Action string // POST target "/w/{slug}/renewal-management/{id}"
	Err    string // pesan galat ?err=

	// Field read-only dari subscription / account.
	VillageName   string
	PlanName      string
	RenewalDate   string // end_date — read-only
	RenewalStatus string // renewal_status — read-only

	// Nilai aksi CS saat ini (dari DB).
	CurrentStage      string
	CurrentRisk       string
	CurrentActionPlan string
	CurrentNextAction string // tanggal tindakan berikutnya
	CurrentOwnerID    *int64

	// Pilihan dropdown.
	Members []CSRenewalMemberOption
	Stages  []string // ["Not Started","Outreach","Negotiation","Won","Lost"]
	Risks   []string // ["Low","Medium","High"]
}

// CSRenewalForm merender form edit aksi CS renewal.
func CSRenewalForm(v CSRenewalFormView) g.Node {
	return h.Div(h.Class("space-y-4"),
		csRenewalFormHeader(v),
		csRenewalFormAlert(v.Err),
		csRenewalFormBody(v),
	)
}

func csRenewalFormHeader(v CSRenewalFormView) g.Node {
	return h.Div(h.Class("flex flex-wrap items-center gap-2"),
		h.A(h.Href(v.Base+"/renewal-management"),
			h.Class("btn btn-ghost btn-sm min-h-11"),
			g.Text("← Kembali"),
		),
		h.H1(h.Class("text-xl font-bold"), g.Text("Edit Aksi Renewal")),
	)
}

func csRenewalFormAlert(errMsg string) g.Node {
	if errMsg == "" {
		return nil
	}
	return h.Div(h.Class("alert alert-error"), g.Text(errMsg))
}

func csRenewalFormBody(v CSRenewalFormView) g.Node {
	return h.Form(h.Method("post"), h.Action(v.Action),
		h.Class("space-y-4"),
		csRenewalReadOnlyCard(v),
		csRenewalActionCard(v),
		h.Div(h.Class("flex flex-wrap gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
				g.Text("Simpan"),
			),
			h.A(h.Href(v.Base+"/renewal-management"),
				h.Class("btn btn-ghost min-h-11"),
				g.Text("Batal"),
			),
		),
	)
}

// csRenewalReadOnlyCard — bagian atas: info subscription (read-only).
func csRenewalReadOnlyCard(v CSRenewalFormView) g.Node {
	return h.Div(h.Class("card bg-base-100 shadow-sm"),
		h.Div(h.Class("card-body gap-3"),
			h.H2(h.Class("card-title text-base"), g.Text("Informasi Langganan")),
			h.P(h.Class("text-sm text-base-content/60"),
				g.Text("Data di bawah diambil dari Modul Subscriptions dan tidak dapat diubah di sini."),
			),
			h.Div(h.Class("grid grid-cols-1 sm:grid-cols-2 gap-4"),
				csFormFieldReadOnly("Desa", orDash(v.VillageName)),
				csFormFieldReadOnly("Plan", orDash(v.PlanName)),
				csFormFieldReadOnly("Tanggal Renewal", orDash(v.RenewalDate)),
				csFormFieldReadOnly("Status Renewal", orDash(v.RenewalStatus)),
			),
		),
	)
}

// csRenewalActionCard — bagian bawah: field aksi CS yang bisa diubah.
func csRenewalActionCard(v CSRenewalFormView) g.Node {
	return h.Div(h.Class("card bg-base-100 shadow-sm"),
		h.Div(h.Class("card-body gap-4"),
			h.H2(h.Class("card-title text-base"), g.Text("Aksi Customer Success")),
			h.Div(h.Class("grid grid-cols-1 sm:grid-cols-2 gap-4"),
				csRenewalStageField(v),
				csRenewalRiskField(v),
			),
			csRenewalActionPlanField(v),
			h.Div(h.Class("grid grid-cols-1 sm:grid-cols-2 gap-4"),
				csRenewalNextActionField(v),
				csRenewalOwnerField(v),
			),
		),
	)
}

func csRenewalStageField(v CSRenewalFormView) g.Node {
	opts := make([]g.Node, 0, len(v.Stages)+1)
	opts = append(opts, h.Option(h.Value(""), g.Text("— Pilih Stage —"),
		g.If(v.CurrentStage == "", h.Selected())))
	for _, s := range v.Stages {
		opts = append(opts, h.Option(h.Value(s),
			g.If(v.CurrentStage == s, h.Selected()),
			g.Text(s),
		))
	}
	return h.Div(h.Class("form-control gap-1"),
		h.Label(h.Class("label pb-0"), h.Span(h.Class("label-text"), g.Text("Renewal Stage"))),
		h.Select(h.Name("renewal_stage"), h.Class("select select-bordered w-full"),
			g.Group(opts),
		),
	)
}

func csRenewalRiskField(v CSRenewalFormView) g.Node {
	opts := make([]g.Node, 0, len(v.Risks)+1)
	opts = append(opts, h.Option(h.Value(""), g.Text("— Pilih Risiko —"),
		g.If(v.CurrentRisk == "", h.Selected())))
	for _, r := range v.Risks {
		opts = append(opts, h.Option(h.Value(r),
			g.If(v.CurrentRisk == r, h.Selected()),
			g.Text(r),
		))
	}
	return h.Div(h.Class("form-control gap-1"),
		h.Label(h.Class("label pb-0"), h.Span(h.Class("label-text"), g.Text("Renewal Risk"))),
		h.Select(h.Name("renewal_risk"), h.Class("select select-bordered w-full"),
			g.Group(opts),
		),
	)
}

func csRenewalActionPlanField(v CSRenewalFormView) g.Node {
	return h.Div(h.Class("form-control gap-1"),
		h.Label(h.Class("label pb-0"), h.Span(h.Class("label-text"), g.Text("Action Plan"))),
		h.Textarea(
			h.Name("renewal_action_plan"),
			h.Class("textarea textarea-bordered w-full min-h-[100px]"),
			h.Placeholder("Rencana aksi detail untuk renewal ini…"),
			g.Text(v.CurrentActionPlan),
		),
	)
}

func csRenewalNextActionField(v CSRenewalFormView) g.Node {
	return h.Div(h.Class("form-control gap-1"),
		h.Label(h.Class("label pb-0"), h.Span(h.Class("label-text"), g.Text("Tanggal Tindakan Berikutnya"))),
		h.Input(
			h.Type("date"),
			h.Name("renewal_next_action_date"),
			h.Class("input input-bordered w-full"),
			g.If(v.CurrentNextAction != "", h.Value(v.CurrentNextAction)),
		),
	)
}

func csRenewalOwnerField(v CSRenewalFormView) g.Node {
	currentID := int64(0)
	if v.CurrentOwnerID != nil {
		currentID = *v.CurrentOwnerID
	}
	opts := make([]g.Node, 0, len(v.Members)+1)
	opts = append(opts, h.Option(h.Value(""), g.Text("— Pilih Owner CSM —"),
		g.If(currentID == 0, h.Selected())))
	for _, m := range v.Members {
		label := fmt.Sprintf("%s", m.Name)
		opts = append(opts, h.Option(
			h.Value(strconv.FormatInt(m.ID, 10)),
			g.If(currentID == m.ID, h.Selected()),
			g.Text(label),
		))
	}
	return h.Div(h.Class("form-control gap-1"),
		h.Label(h.Class("label pb-0"), h.Span(h.Class("label-text"), g.Text("Owner CSM"))),
		h.Select(h.Name("renewal_owner"), h.Class("select select-bordered w-full"),
			g.Group(opts),
		),
	)
}

// csFormFieldReadOnly — field tampil (tidak dapat diubah).
func csFormFieldReadOnly(label, value string) g.Node {
	return h.Div(h.Class("form-control gap-1"),
		h.Label(h.Class("label pb-0"), h.Span(h.Class("label-text"), g.Text(label))),
		h.Div(h.Class("input input-bordered bg-base-200 flex items-center text-base-content/70 text-sm"),
			g.Text(value),
		),
	)
}
