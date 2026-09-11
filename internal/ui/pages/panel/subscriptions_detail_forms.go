package panel

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// subscriptions_detail_forms.go — kartu aksi & form langganan (approve, renew,
// activate, churn) beserta pemilih generik. Dipisah dari subscriptions_detail.go
// agar file induk di bawah ambang tipe View/Component (300). Satu paket panel —
// tampilan identik.

// subAction = satu aksi langganan (BL-125): pemicu (tombol) + isi modal. Daftar
// dirakit sekali di subActions() agar pemicu (di header) & dialog (di body) tak
// bisa lepas sinkron.
type subAction struct {
	id           string
	triggerLabel string
	triggerClass string
	modalTitle   string
	body         g.Node
}

// subActions = daftar aksi yang boleh tampil untuk langganan ini. Tiap aksi hanya
// masuk bila handler mengizinkan (flag) DAN status memungkinkan (view murni-data;
// status dicek ulang agar tak menawarkan aksi yang pasti ditolak backend).
func subActions(v SubDetailView) []subAction {
	var a []subAction
	if v.CanApprove && v.Status == "PendingApproval" {
		a = append(a, subAction{"sub-approve", "Tinjau Persetujuan",
			"btn btn-primary min-h-11", "Persetujuan Renewal", subApproveBody(v)})
	}
	if v.CanRenew && v.Status == "Active" {
		a = append(a, subAction{"sub-renew", "Perpanjang",
			"btn btn-primary min-h-11", "Perpanjang Langganan", subRenewBody(v)})
	}
	if v.CanActivate && v.Status == "Trial" {
		a = append(a, subAction{"sub-activate", "Aktifkan Langganan",
			"btn btn-primary min-h-11", "Aktifkan Langganan", subActivateBody(v)})
	}
	if v.CanChurn && (v.Status == "Active" || v.Status == "Trial") {
		a = append(a, subAction{"sub-churn", "Tandai Churn",
			"btn btn-error btn-outline min-h-11", "Tandai Churn", subChurnBody(v)})
	}
	return a
}

// subActionTriggers = tombol pemicu tiap aksi, untuk dipasang di HEADER (kanan-atas,
// BL-125) — bukan lagi kartu "Tindakan" sendiri. Kosong bila tak ada aksi.
func subActionTriggers(v SubDetailView) []g.Node {
	acts := subActions(v)
	nodes := make([]g.Node, 0, len(acts))
	for _, a := range acts {
		nodes = append(nodes, modalTrigger(a.id, a.triggerLabel, a.triggerClass))
	}
	return nodes
}

// subActionDialogs = modal (checkbox-toggle CSP-safe, modal.go, gotcha #1/#16) tiap
// aksi; form di dalamnya tetap NATIVE POST → 303. Disisipkan di body halaman (posisi
// DOM bebas — visibilitas via checkbox tersembunyi, bukan aliran dokumen).
func subActionDialogs(v SubDetailView) []g.Node {
	acts := subActions(v)
	nodes := make([]g.Node, 0, len(acts))
	for _, a := range acts {
		nodes = append(nodes, modalDialog(a.id, a.modalTitle, a.body))
	}
	return nodes
}

// subApproveBody = isi modal persetujuan: keputusan Setujui/Tolak renewal Upsell
// (dua form POST terpisah).
func subApproveBody(v SubDetailView) g.Node {
	base := v.Base + "/subscriptions/" + strconv.FormatInt(v.ID, 10)
	return h.Div(
		h.Class("grid gap-3"),
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

// subRenewBody = isi modal perpanjang: MRR baru opsional (kosong = sama). MRR baru
// lebih besar → jalur Upsell (butuh persetujuan) diputuskan backend.
func subRenewBody(v SubDetailView) g.Node {
	base := v.Base + "/subscriptions/" + strconv.FormatInt(v.ID, 10)
	return h.FormEl(
		h.Method("post"), h.Action(base+"/renew"),
		h.Class("grid gap-3"),
		h.Label(h.Class("form-control w-full"),
			h.Span(h.Class("label-text text-sm mb-1"),
				g.Text("MRR baru (kosongkan bila sama)")),
			h.Input(h.Type("text"), h.Name("new_mrr"),
				g.Attr("inputmode", "numeric"), g.Attr("pattern", "[0-9.]*"),
				g.Attr("data-numgroup", ""),
				h.Class("input input-bordered text-base w-full"),
				h.Placeholder("mis. 750.000")),
		),
		h.Div(h.Class("flex flex-wrap gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
				g.Text("Perpanjang"))),
	)
}

// subActivateBody = isi modal aktivasi (Trial → Active, BL-73). Satu tombol POST —
// tak ada input (transisi status murni); menutup jalan buntu Trial (tak diakui
// pendapatan, tak bisa di-renew).
func subActivateBody(v SubDetailView) g.Node {
	base := v.Base + "/subscriptions/" + strconv.FormatInt(v.ID, 10)
	return h.FormEl(
		h.Method("post"), h.Action(base+"/activate"),
		h.Class("grid gap-3"),
		h.P(h.Class("text-sm text-base-content/70"),
			g.Text("Naikkan langganan Trial ini ke Active agar pendapatan diakui dan langganan bisa diperpanjang.")),
		h.Div(h.Class("flex flex-wrap gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"),
				g.Text("Aktifkan Langganan"))),
	)
}

// churnReasonLegend = makna ringkas tiap opsi alasan churn (BL-153), cermin label
// ID singkat churnReasonLabelID (handler/reports_subscriptions_panels.go) — disalin
// literal krn panel tak boleh depend handler.
var churnReasonLegend = [][2]string{
	{"Budget", "Anggaran tidak lanjut"},
	{"No Adoption", "Adopsi rendah"},
	{"Change of Leadership", "Pergantian pimpinan"},
	{"Competitor", "Pindah vendor"},
	{"Dissatisfaction", "Ketidakpuasan"},
	{"Feature Gap", "Fitur kurang"},
}

// churnTypeLegend = makna 2 opsi tipe churn (BL-153).
var churnTypeLegend = [][2]string{
	{"Voluntary", "Pelanggan berhenti atas keputusan sendiri."},
	{"Involuntary", "Berhenti bukan atas kehendak pelanggan (mis. gagal bayar)."},
}

// subChurnBody = isi modal churn: alasan & tipe (dropdown domain), catatan, layak
// win-back. lost_value_mrr dihitung backend (MRR saat ini), bukan input.
// BL-153: alasan/tipe di-stack 1 kolom penuh (bukan sm:grid-cols-2 — opsi cukup
// pendek tapi makna tak jelas tanpa penjelasan, jadi ruang dipakai tap-info ⓘ
// (labelWithLegend) ketimbang 2 kolom sempit).
func subChurnBody(v SubDetailView) g.Node {
	base := v.Base + "/subscriptions/" + strconv.FormatInt(v.ID, 10)
	return h.FormEl(
		h.Method("post"), h.Action(base+"/churn"),
		h.Class("grid gap-3"),
		subSelect("churn_reason", "Alasan churn", v.ChurnReasons, churnReasonLegend),
		subSelect("churn_type", "Tipe churn", v.ChurnTypes, churnTypeLegend),
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
// datang dari handler (satu sumber dgn validasi backend). legend (BL-153) → label
// via labelWithLegend (tap-info ⓘ); kosong → label biasa (labelFor, di dalamnya).
func subSelect(name, label string, opts []string, legend [][2]string) g.Node {
	nodes := []g.Node{h.Option(h.Value(""), g.Text("—"))}
	for _, o := range opts {
		nodes = append(nodes, h.Option(h.Value(o), g.Text(o)))
	}
	return h.Div(h.Class("grid gap-1 min-w-0"),
		labelWithLegend(label, "f-"+name, false, legend),
		h.Select(append([]g.Node{h.ID("f-" + name), h.Name(name),
			h.Class("select select-bordered text-base w-full")}, nodes...)...),
	)
}
