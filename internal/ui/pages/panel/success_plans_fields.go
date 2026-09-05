package panel

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// success_plans_fields.go — pembangun field individual form Success Plan,
// dipisah dari success_plans_form.go (ukuran file). Murni-data; render identik.

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

// successPlanTwoCol membungkus dua field dalam grid responsif (1 kolom di mobile, 2 di ≥sm).
func successPlanTwoCol(a, b g.Node) g.Node {
	return h.Div(h.Class("grid grid-cols-1 sm:grid-cols-2 gap-4"), a, b)
}

// successPlanTextareaField membangun field textarea generik (objektif, metrik sukses).
func successPlanTextareaField(label, name, placeholder, val string) g.Node {
	return h.Div(h.Class("form-control gap-1"),
		h.Label(h.Class("label pb-0"), h.Span(h.Class("label-text"), g.Text(label))),
		h.Textarea(
			h.Name(name),
			h.Class("textarea textarea-bordered w-full min-h-[80px]"),
			h.Placeholder(placeholder),
			g.Text(val),
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
