package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sla_policies.go — view katalog SLA Policies (Modul 6 Customer Success, slice
// A1). Murni-data: target sudah diformat handler (int32Str), status sudah
// bool. Meniru plans.go. sla_policies TANPA soft-delete: is_active=false =
// pensiun — baris tetap tampil di kelola, ditandai badge. Retire/aktifkan =
// native POST (gotcha #16), aksi tersendiri per baris.
//
// KPI kepatuhan SLA & kolom "Kepatuhan" wireframe 6.11 SENGAJA tak ada di
// slice ini — keduanya butuh data tiket yang belum ada (D1/D2). Nama Kebijakan
// ditambah sebagai kolom (bukan di wireframe, tapi sla_name kolom wajib skema
// sejak 00005 — satu prioritas bisa punya lebih dari satu kebijakan).

// SLAPolicyRow = satu kebijakan untuk baris tabel kelola. Target respon/
// selesai SUDAH diformat handler (int32Str, satuan menit; "" = tak diset).
type SLAPolicyRow struct {
	ID               int64
	SLAName          string
	Priority         string
	FirstResponseMin string
	ResolutionMin    string
	BusinessHours    string
	Active           bool
}

// SLAPolicyListView = data halaman /sla-policies. CanWrite (manager/admin)
// memunculkan tombol tulis & aksi baris. Keyset lewat NextCursor (BL-6):
// "" = ujung daftar.
type SLAPolicyListView struct {
	Base       string
	CanWrite   bool
	Err        string
	Msg        string
	Items      []SLAPolicyRow
	NextCursor string
	After      string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail      string // BL-7: jejak cursor halaman sebelumnya (?trail=)
}

// SLAPolicyList merender halaman katalog: header + alert + tabel.
func SLAPolicyList(v SLAPolicyListView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("SLA Management")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Kebijakan SLA per prioritas — target respon & penyelesaian, dasar janji layanan ke desa.")),
			),
			ui.When(v.CanWrite, h.A(
				h.Href(v.Base+"/sla-policies/new"), h.Class("btn btn-primary min-h-11"),
				g.Text("Kebijakan SLA Baru"),
			)),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "sla-policies-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "sla-policies-ok", g.Text(v.Msg)))
	}
	if len(v.Items) == 0 {
		body = append(body, emptySLAPolicies())
	} else {
		body = append(body, slaPoliciesTable(v), slaPoliciesPager(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// slaPoliciesPager = tautan keyset "Berikutnya »" (native <a>, lolos gotcha #16).
// NextCursor kosong = ujung daftar. flex-wrap agar tak mendorong lebar di 375px.
func slaPoliciesPager(v SLAPolicyListView) g.Node {
	base := v.Base + "/sla-policies"
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}

func emptySLAPolicies() g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body"),
			h.P(h.Class("text-base-content/70"), g.Text("Belum ada kebijakan SLA di katalog."))),
	)
}

// slaPoliciesTable = tabel katalog, dibungkus ui.TableScroll (scroll
// terkurung, tak meluberkan viewport 375px).
func slaPoliciesTable(v SLAPolicyListView) g.Node {
	head := []g.Node{
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Nama Kebijakan")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Prioritas")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Target Respon")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Target Selesai")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Jam Berlaku")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
	}
	if v.CanWrite {
		head = append(head, h.Th(h.Class("py-2 font-medium"), g.Text("Aksi")))
	}
	rows := make([]g.Node, 0, len(v.Items))
	for _, p := range v.Items {
		rows = append(rows, slaPolicyTableRow(v.Base, p, v.CanWrite))
	}
	return h.Div(
		h.Class("card bg-base-100 border border-base-300 min-w-0"),
		h.Div(
			h.Class("card-body min-w-0"),
			ui.TableScroll(h.Table(
				h.Class("w-full text-sm"),
				h.THead(h.Tr(
					h.Class("border-b border-base-300 text-left text-base-content/70"),
					g.Group(head),
				)),
				h.TBody(g.Group(rows)),
			)),
		),
	)
}

func slaPolicyTableRow(base string, p SLAPolicyRow, canWrite bool) g.Node {
	cells := []g.Node{
		h.Td(h.Class("py-2 pr-4 font-medium"), g.Text(orDash(p.SLAName))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(p.Priority))),
		h.Td(h.Class("py-2 pr-4"), g.Text(minutesLabel(p.FirstResponseMin))),
		h.Td(h.Class("py-2 pr-4"), g.Text(minutesLabel(p.ResolutionMin))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(p.BusinessHours))),
		h.Td(h.Class("py-2 pr-4"), slaPolicyStatusBadge(p.Active)),
	}
	if canWrite {
		cells = append(cells, h.Td(h.Class("py-2"), slaPolicyRowActions(base, p)))
	}
	return h.Tr(h.Class("border-b border-base-300/50 hover:bg-base-200/50"), g.Group(cells))
}

// minutesLabel memformat target menit untuk tampilan ("15 menit"); kosong →
// "—". Sengaja tak dikonversi ke jam/hari (mis. "2 jam") — angka mentah
// menit yang jadi satuan penyimpanan lebih tak ambigu daripada pembulatan.
func minutesLabel(min string) string {
	if min == "" {
		return "—"
	}
	return min + " menit"
}

// slaPolicyStatusBadge = badge status pakai token semantik daisyUI (bukan
// absolut).
func slaPolicyStatusBadge(active bool) g.Node {
	if active {
		return h.Span(h.Class("badge badge-success"), g.Text("Aktif"))
	}
	return h.Span(h.Class("badge badge-ghost"), g.Text("Pensiun"))
}

// slaPolicyRowActions = Sunting + Pensiunkan/Aktifkan. Pensiun/aktifkan = FORM
// native POST tersendiri (aksi, bukan navigasi) → 303 PRG di handler.
// flex-wrap agar tak mendorong lebar tabel di mobile.
func slaPolicyRowActions(base string, p SLAPolicyRow) g.Node {
	id := strconv.FormatInt(p.ID, 10)
	toggle := slaPolicyRetireForm(base, id)
	if !p.Active {
		toggle = slaPolicyActivateForm(base, id)
	}
	return h.Div(
		h.Class("flex flex-wrap items-center gap-2"),
		h.A(h.Href(base+"/sla-policies/"+id+"/edit"), h.Class("btn btn-ghost btn-xs min-h-11"),
			g.Text("Sunting")),
		toggle,
	)
}

func slaPolicyRetireForm(base, id string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/sla-policies/"+id+"/retire"),
		h.Button(h.Type("submit"), h.Class("btn btn-ghost btn-xs text-warning min-h-11"),
			g.Text("Pensiunkan")),
	)
}

func slaPolicyActivateForm(base, id string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/sla-policies/"+id+"/activate"),
		h.Button(h.Type("submit"), h.Class("btn btn-ghost btn-xs text-success min-h-11"),
			g.Text("Aktifkan")),
	)
}

// ── Form buat/sunting kebijakan ──────────────────────────────────────────────

// SLAPolicyFormFields = nilai prefill form (semua string agar view netral
// terhadap tipe DB). Kosong (buat) atau terisi (sunting).
type SLAPolicyFormFields struct {
	SLAName                    string
	AppliesToPriority          string
	FirstResponseTargetMinutes string
	ResolutionTargetMinutes    string
	BusinessHours              string
	EscalationRule             string
}

// SLAPolicyFormView = data halaman form. Action = URL POST tujuan. Priorities
// & BusinessHoursOp dioper handler (view tak memutuskan enum).
type SLAPolicyFormView struct {
	Base            string
	Action          string
	IsEdit          bool
	Err             string
	Fields          SLAPolicyFormFields
	Priorities      []string
	BusinessHoursOp []string
}

// SLAPolicyForm merender halaman form lengkap (native POST → 303, gotcha #16).
func SLAPolicyForm(v SLAPolicyFormView) g.Node {
	title, submit := "Tambah Kebijakan SLA", "Simpan Kebijakan"
	if v.IsEdit {
		title, submit = "Sunting Kebijakan SLA", "Simpan Perubahan"
	}
	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text(title)),
			h.A(h.Href(v.Base+"/sla-policies"), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke katalog")),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "sla-policy-form-err", g.Text(v.Err)))
	}
	body = append(body, h.FormEl(
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),
		formCard("Identitas",
			field("Nama Kebijakan", "sla_name", v.Fields.SLAName, true, "text"),
			selectField("Prioritas", "applies_to_priority", v.Fields.AppliesToPriority, v.Priorities, false),
			selectField("Jam Berlaku", "business_hours", v.Fields.BusinessHours, v.BusinessHoursOp, false),
		),
		formCard("Target SLA (menit)",
			field("Target Respon (menit)", "first_response_target_minutes", v.Fields.FirstResponseTargetMinutes, false, "text"),
			field("Target Selesai (menit)", "resolution_target_minutes", v.Fields.ResolutionTargetMinutes, false, "text"),
		),
		formCard("Eskalasi",
			textareaField("Aturan Eskalasi", "escalation_rule", v.Fields.EscalationRule),
		),
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text(submit)),
			h.A(h.Href(v.Base+"/sla-policies"), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}
