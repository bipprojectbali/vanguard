package panel

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// success_plans_form.go — form buat/edit Success Plan (Modul 6 slice 6.3).
//
// View murni-data: tidak ada import db/authz/session.
// Handler memetakan db.GetSuccessPlanRow + db.Account → SuccessPlanFormView.

// SuccessPlanAccountOption — satu pilihan dropdown Desa (untuk form create).
type SuccessPlanAccountOption struct {
	ID   int64
	Name string
}

// SuccessPlanMemberOption — satu pilihan dropdown Owner CSM.
type SuccessPlanMemberOption struct {
	ID   int64
	Name string
}

// SuccessPlanFormView — data lengkap form buat/edit success plan.
// Mode create: Action = POST "/w/{slug}/success-plans", Accounts terisi, AccountName kosong.
// Mode edit:   Action = POST "/w/{slug}/success-plans/{id}", Accounts kosong, AccountName terisi.
type SuccessPlanFormView struct {
	Base   string // "/w/{slug}"
	Action string // POST target
	Err    string // pesan galat ?err=

	// Mode edit: nama desa read-only (tidak tampilkan dropdown).
	AccountName string

	// Nilai saat ini (diisi handler pada mode edit).
	CurrentPlanName      string
	CurrentObjective     string
	CurrentSuccessMetric string
	CurrentTargetDate    string // YYYY-MM-DD
	CurrentStatus        string
	CurrentProgress      int // 0–100
	CurrentOwnerCsmID    *int64

	// Pilihan dropdown.
	Accounts []SuccessPlanAccountOption // diisi mode create; kosong mode edit
	Members  []SuccessPlanMemberOption
	Statuses []string // ["Draft","Active","Achieved","At-Risk","Cancelled"]
}

// SuccessPlanForm merender form buat atau edit success plan.
func SuccessPlanForm(v SuccessPlanFormView) g.Node {
	isEdit := v.AccountName != "" // mode edit jika akun sudah terpilih
	return h.Div(h.Class("space-y-4"),
		successPlanFormHeader(v, isEdit),
		successPlanFormAlert(v.Err),
		successPlanFormBody(v, isEdit),
	)
}

func successPlanFormHeader(v SuccessPlanFormView, isEdit bool) g.Node {
	title := "Buat Success Plan"
	if isEdit {
		title = "Edit Success Plan"
	}
	return h.Div(h.Class("flex flex-wrap items-center gap-2"),
		h.A(h.Href(v.Base+"/success-plans"),
			h.Class("btn btn-ghost btn-sm min-h-11"),
			g.Text("← Kembali"),
		),
		h.H1(h.Class("text-xl font-bold"), g.Text(title)),
	)
}

func successPlanFormAlert(errMsg string) g.Node {
	if errMsg == "" {
		return nil
	}
	return h.Div(h.Class("alert alert-error"), g.Text(errMsg))
}

func successPlanFormBody(v SuccessPlanFormView, isEdit bool) g.Node {
	return h.Form(h.Method("post"), h.Action(v.Action),
		h.Class("space-y-4"),
		successPlanAccountCard(v, isEdit),
		successPlanDetailCard(v),
		h.Div(h.Class("flex flex-wrap gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
				g.Text("Simpan"),
			),
			h.A(h.Href(v.Base+"/success-plans"),
				h.Class("btn btn-ghost min-h-11"),
				g.Text("Batal"),
			),
		),
	)
}

// successPlanAccountCard — desa: dropdown pada mode create; read-only mode edit.
func successPlanAccountCard(v SuccessPlanFormView, isEdit bool) g.Node {
	var accountField g.Node
	if isEdit {
		// Edit: nama desa statis, tanpa dropdown.
		accountField = csFormFieldReadOnly("Desa", orDash(v.AccountName))
	} else {
		// Create: dropdown semua desa dalam cakupan.
		opts := make([]g.Node, 0, len(v.Accounts)+1)
		opts = append(opts, h.Option(h.Value(""), g.Text("— Pilih Desa —"),
			h.Selected()))
		for _, a := range v.Accounts {
			opts = append(opts, h.Option(
				h.Value(strconv.FormatInt(a.ID, 10)),
				g.Text(a.Name),
			))
		}
		accountField = h.Div(h.Class("form-control gap-1"),
			h.Label(h.Class("label pb-0"),
				h.Span(h.Class("label-text"), g.Text("Desa")),
				h.Span(h.Class("label-text-alt text-error"), g.Text("*wajib")),
			),
			h.Select(
				h.Name("account_id"),
				h.Class("select select-bordered w-full"),
				h.Required(),
				g.Group(opts),
			),
		)
	}
	return h.Div(h.Class("card bg-base-100 shadow-sm"),
		h.Div(h.Class("card-body gap-4"),
			h.H2(h.Class("card-title text-base"), g.Text("Desa")),
			accountField,
		),
	)
}

// successPlanDetailCard — detail plan: nama, objektif, metrik, target, status, progres, owner.
func successPlanDetailCard(v SuccessPlanFormView) g.Node {
	return h.Div(h.Class("card bg-base-100 shadow-sm"),
		h.Div(h.Class("card-body gap-4"),
			h.H2(h.Class("card-title text-base"), g.Text("Detail Plan")),
			successPlanNameField(v),
			h.Div(h.Class("grid grid-cols-1 sm:grid-cols-2 gap-4"),
				successPlanObjectiveField(v),
				successPlanSuccessMetricField(v),
			),
			h.Div(h.Class("grid grid-cols-1 sm:grid-cols-2 gap-4"),
				successPlanTargetDateField(v),
				successPlanProgressField(v),
			),
			h.Div(h.Class("grid grid-cols-1 sm:grid-cols-2 gap-4"),
				successPlanStatusField(v),
				successPlanOwnerCsmField(v),
			),
		),
	)
}

func successPlanNameField(v SuccessPlanFormView) g.Node {
	return h.Div(h.Class("form-control gap-1"),
		h.Label(h.Class("label pb-0"),
			h.Span(h.Class("label-text"), g.Text("Nama Plan")),
			h.Span(h.Class("label-text-alt text-error"), g.Text("*wajib")),
		),
		h.Input(
			h.Type("text"),
			h.Name("plan_name"),
			h.Class("input input-bordered w-full"),
			h.Placeholder("Contoh: Q3 Digitalisasi Desa Sukamaju"),
			h.Required(),
			g.If(v.CurrentPlanName != "", h.Value(v.CurrentPlanName)),
		),
	)
}

func successPlanObjectiveField(v SuccessPlanFormView) g.Node {
	return h.Div(h.Class("form-control gap-1"),
		h.Label(h.Class("label pb-0"), h.Span(h.Class("label-text"), g.Text("Objektif"))),
		h.Textarea(
			h.Name("objective"),
			h.Class("textarea textarea-bordered w-full min-h-[80px]"),
			h.Placeholder("Tujuan utama yang ingin dicapai…"),
			g.Text(v.CurrentObjective),
		),
	)
}

func successPlanSuccessMetricField(v SuccessPlanFormView) g.Node {
	return h.Div(h.Class("form-control gap-1"),
		h.Label(h.Class("label pb-0"), h.Span(h.Class("label-text"), g.Text("Metrik Sukses"))),
		h.Textarea(
			h.Name("success_metric"),
			h.Class("textarea textarea-bordered w-full min-h-[80px]"),
			h.Placeholder("Cara mengukur keberhasilan plan ini…"),
			g.Text(v.CurrentSuccessMetric),
		),
	)
}

func successPlanTargetDateField(v SuccessPlanFormView) g.Node {
	return h.Div(h.Class("form-control gap-1"),
		h.Label(h.Class("label pb-0"), h.Span(h.Class("label-text"), g.Text("Target Tanggal"))),
		h.Input(
			h.Type("date"),
			h.Name("target_date"),
			h.Class("input input-bordered w-full"),
			g.If(v.CurrentTargetDate != "", h.Value(v.CurrentTargetDate)),
		),
	)
}

func successPlanProgressField(v SuccessPlanFormView) g.Node {
	progVal := strconv.Itoa(v.CurrentProgress)
	return h.Div(h.Class("form-control gap-1"),
		h.Label(h.Class("label pb-0"),
			h.Span(h.Class("label-text"), g.Text("Progres (%)")),
			h.Span(h.Class("label-text-alt text-base-content/60"), g.Text("0–100")),
		),
		h.Input(
			h.Type("number"),
			h.Name("progress"),
			h.Class("input input-bordered w-full"),
			g.Attr("min", "0"),
			g.Attr("max", "100"),
			h.Value(progVal),
		),
	)
}

func successPlanStatusField(v SuccessPlanFormView) g.Node {
	opts := make([]g.Node, 0, len(v.Statuses)+1)
	opts = append(opts, h.Option(h.Value(""), g.Text("— Pilih Status —"),
		g.If(v.CurrentStatus == "", h.Selected())))
	for _, s := range v.Statuses {
		opts = append(opts, h.Option(
			h.Value(s),
			g.If(v.CurrentStatus == s, h.Selected()),
			g.Text(s),
		))
	}
	return h.Div(h.Class("form-control gap-1"),
		h.Label(h.Class("label pb-0"),
			h.Span(h.Class("label-text"), g.Text("Status")),
			h.Span(h.Class("label-text-alt text-error"), g.Text("*wajib")),
		),
		h.Select(h.Name("plan_status"), h.Class("select select-bordered w-full"),
			h.Required(),
			g.Group(opts),
		),
	)
}

func successPlanOwnerCsmField(v SuccessPlanFormView) g.Node {
	currentID := int64(0)
	if v.CurrentOwnerCsmID != nil {
		currentID = *v.CurrentOwnerCsmID
	}
	opts := make([]g.Node, 0, len(v.Members)+1)
	opts = append(opts, h.Option(h.Value(""), g.Text("— Pilih Owner CS —"),
		g.If(currentID == 0, h.Selected())))
	for _, m := range v.Members {
		opts = append(opts, h.Option(
			h.Value(strconv.FormatInt(m.ID, 10)),
			g.If(currentID == m.ID, h.Selected()),
			g.Text(m.Name),
		))
	}
	return h.Div(h.Class("form-control gap-1"),
		h.Label(h.Class("label pb-0"), h.Span(h.Class("label-text"), g.Text("Owner CS"))),
		h.Select(h.Name("owner_csm"), h.Class("select select-bordered w-full"),
			g.Group(opts),
		),
	)
}
