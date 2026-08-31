package panel

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// subscriptions_detail_forms.go — kartu aksi & form langganan (approve, renew,
// churn) beserta pemilih generik. Dipisah dari subscriptions_detail.go agar
// file induk di bawah ambang tipe View/Component (300). Satu paket panel —
// tampilan identik.

// subActionCard = kartu aksi langganan (M5-3c): approve/reject (saat menunggu
// persetujuan), perpanjang, dan churn. Tiap form hanya tampil bila handler
// mengizinkan (flag) DAN status memungkinkan. Kosong → tak dirender sama sekali.
func subActionCard(v SubDetailView) g.Node {
	var forms []g.Node
	if v.CanApprove && v.Status == "PendingApproval" {
		forms = append(forms, subApproveForms(v))
	}
	if v.CanRenew && v.Status == "Active" {
		forms = append(forms, subRenewForm(v))
	}
	if v.CanChurn && (v.Status == "Active" || v.Status == "Trial") {
		forms = append(forms, subChurnForm(v))
	}
	if len(forms) == 0 {
		return g.Text("")
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0 grid gap-4"),
			h.H2(h.Class("font-semibold"), g.Text("Tindakan")),
			g.Group(forms),
		),
	)
}

// subApproveForms = tombol Setujui/Tolak renewal Upsell (dua form POST terpisah).
func subApproveForms(v SubDetailView) g.Node {
	base := v.Base + "/subscriptions/" + strconv.FormatInt(v.ID, 10)
	return h.Div(
		h.Class("grid gap-2"),
		h.P(h.Class("text-sm text-base-content/70"),
			g.Text("Renewal upsell ini menunggu keputusan Anda.")),
		h.Div(
			h.Class("flex flex-wrap gap-2"),
			h.FormEl(h.Method("post"), h.Action(base+"/approve"),
				h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text("Setujui"))),
			h.FormEl(h.Method("post"), h.Action(base+"/reject"),
				h.Button(h.Type("submit"), h.Class("btn btn-error btn-outline min-h-11"), g.Text("Tolak"))),
		),
	)
}

// subRenewForm = form perpanjang: MRR baru opsional (kosong = sama). MRR baru lebih
// besar → jalur Upsell (butuh persetujuan) diputuskan backend.
func subRenewForm(v SubDetailView) g.Node {
	base := v.Base + "/subscriptions/" + strconv.FormatInt(v.ID, 10)
	return h.FormEl(
		h.Method("post"), h.Action(base+"/renew"),
		h.Class("grid gap-2"),
		h.H3(h.Class("font-medium text-sm"), g.Text("Perpanjang Langganan")),
		h.Label(h.Class("form-control w-full max-w-xs"),
			h.Span(h.Class("label-text text-sm mb-1"),
				g.Text("MRR baru (kosongkan bila sama)")),
			h.Input(h.Type("text"), g.Attr("inputmode", "numeric"), h.Name("new_mrr"),
				h.Class("input input-bordered text-base w-full"),
				h.Placeholder("mis. 750000")),
		),
		h.Div(h.Class("flex flex-wrap gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
				g.Text("Perpanjang"))),
	)
}

// subChurnForm = form churn: alasan & tipe (dropdown domain), catatan, layak
// win-back. lost_value_mrr dihitung backend (MRR saat ini), bukan input.
func subChurnForm(v SubDetailView) g.Node {
	base := v.Base + "/subscriptions/" + strconv.FormatInt(v.ID, 10)
	return h.FormEl(
		h.Method("post"), h.Action(base+"/churn"),
		h.Class("grid gap-2 border-t border-base-300 pt-4"),
		h.H3(h.Class("font-medium text-sm"), g.Text("Tandai Churn")),
		h.Div(
			h.Class("grid gap-2 sm:grid-cols-2"),
			subSelect("churn_reason", "Alasan churn", v.ChurnReasons),
			subSelect("churn_type", "Tipe churn", v.ChurnTypes),
		),
		h.Label(h.Class("form-control w-full"),
			h.Span(h.Class("label-text text-sm mb-1"), g.Text("Catatan (opsional)")),
			h.Textarea(h.Name("churn_notes"), h.Class("textarea textarea-bordered text-base w-full"),
				h.Rows("2")),
		),
		h.Label(h.Class("flex items-center gap-2 cursor-pointer min-h-11"),
			h.Input(h.Type("checkbox"), h.Name("win_back_eligible"), h.Value("1"),
				h.Class("checkbox")),
			h.Span(h.Class("text-sm"), g.Text("Layak win-back")),
		),
		h.Div(h.Class("flex flex-wrap gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-error min-h-11"),
				g.Text("Tandai Churn"))),
	)
}

// subSelect = dropdown domain sederhana dgn opsi kosong "—" (nilai NULL). Opsi
// datang dari handler (satu sumber dgn validasi backend).
func subSelect(name, label string, opts []string) g.Node {
	nodes := []g.Node{h.Option(h.Value(""), g.Text("—"))}
	for _, o := range opts {
		nodes = append(nodes, h.Option(h.Value(o), g.Text(o)))
	}
	return h.Label(h.Class("form-control w-full"),
		h.Span(h.Class("label-text text-sm mb-1"), g.Text(label)),
		h.Select(append([]g.Node{h.Name(name),
			h.Class("select select-bordered text-base w-full")}, nodes...)...),
	)
}
