package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// plans.go — view katalog Plans & Pricing (Modul 5). Murni-data: harga SUDAH
// diformat di handler (formatRupiah), status sudah bool. Meniru accounts.go/
// sales_deals.go. Plans TANPA soft-delete: is_active=false = pensiun — baris
// tetap tampil di kelola (beda dari picker quote yang hanya aktif), ditandai
// badge. Retire/aktifkan = native POST (gotcha #16), aksi tersendiri per baris.

// PlanRow = satu plan untuk baris tabel kelola. Price SUDAH diformat handler.
type PlanRow struct {
	ID       int64
	PlanName string
	PlanCode string
	Category string
	Price    string
	Billing  string
	Currency string
	Active   bool
}

// PlanListView = data halaman /plans. CanWrite (admin) memunculkan tombol tulis
// & aksi baris. Keyset lewat NextCursor (BL-6): "" = ujung daftar.
type PlanListView struct {
	Base       string
	CanWrite   bool
	Err        string
	Msg        string
	Items      []PlanRow
	NextCursor string
	After      string // BL-7: cursor pembuka halaman ini (kosong = hal 1)
	Trail      string // BL-7: jejak cursor halaman sebelumnya (?trail=)
}

// PlanList merender halaman katalog: header + alert + tabel.
func PlanList(v PlanListView) g.Node {
	body := []g.Node{
		h.Div(
			h.Class("flex flex-wrap items-center justify-between gap-2 mb-2"),
			h.Div(
				h.H1(h.Class("text-xl font-semibold"), g.Text("Plans & Pricing")),
				h.P(h.Class("text-base-content/70"),
					g.Text("Katalog paket langganan — harga dasar, siklus tagih, dan status.")),
			),
			ui.When(v.CanWrite, h.A(
				h.Href(v.Base+"/plans/new"), h.Class("btn btn-primary min-h-11"),
				g.Text("Plan Baru"),
			)),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "plans-err", g.Text(v.Err)))
	}
	if v.Msg != "" {
		body = append(body, ui.Alert(ui.VariantDefault, "plans-ok", g.Text(v.Msg)))
	}
	if len(v.Items) == 0 {
		body = append(body, emptyPlans())
	} else {
		body = append(body, plansTable(v), plansPager(v))
	}
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// plansPager = tautan keyset "Berikutnya »" (native <a>, lolos gotcha #16).
// NextCursor kosong = ujung daftar. flex-wrap agar tak mendorong lebar di 375px.
func plansPager(v PlanListView) g.Node {
	base := v.Base + "/plans"
	return ui.KeysetPager(base, v.After, v.Trail, v.NextCursor)
}

func emptyPlans() g.Node {
	return h.Div(
		h.Class("card bg-base-100 border border-base-300"),
		h.Div(h.Class("card-body"),
			h.P(h.Class("text-base-content/70"), g.Text("Belum ada plan di katalog."))),
	)
}

// plansTable = tabel katalog, dibungkus ui.TableScroll (scroll terkurung, tak
// meluberkan viewport 375px).
func plansTable(v PlanListView) g.Node {
	head := []g.Node{
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Kode")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Nama")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Kategori")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Harga Dasar")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Siklus")),
		h.Th(h.Class("py-2 pr-4 font-medium"), g.Text("Status")),
	}
	if v.CanWrite {
		head = append(head, h.Th(h.Class("py-2 font-medium"), g.Text("Aksi")))
	}
	rows := make([]g.Node, 0, len(v.Items))
	for _, p := range v.Items {
		rows = append(rows, planTableRow(v.Base, p, v.CanWrite))
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

func planTableRow(base string, p PlanRow, canWrite bool) g.Node {
	cells := []g.Node{
		h.Td(h.Class("py-2 pr-4 font-mono text-xs"), g.Text(orDash(p.PlanCode))),
		h.Td(h.Class("py-2 pr-4 font-medium"), g.Text(orDash(p.PlanName))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(p.Category))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(p.Price))),
		h.Td(h.Class("py-2 pr-4"), g.Text(orDash(p.Billing))),
		h.Td(h.Class("py-2 pr-4"), planStatusBadge(p.Active)),
	}
	if canWrite {
		cells = append(cells, h.Td(h.Class("py-2"), planRowActions(base, p)))
	}
	return h.Tr(h.Class("border-b border-base-300/50 hover:bg-base-200/50"), g.Group(cells))
}

// planStatusBadge = badge status pakai token semantik daisyUI (bukan absolut).
func planStatusBadge(active bool) g.Node {
	if active {
		return h.Span(h.Class("badge badge-success"), g.Text("Aktif"))
	}
	return h.Span(h.Class("badge badge-ghost"), g.Text("Pensiun"))
}

// planRowActions = Sunting + Pensiunkan/Aktifkan. Pensiun/aktifkan = FORM native
// POST tersendiri (aksi, bukan navigasi) → 303 PRG di handler. flex-wrap agar tak
// mendorong lebar tabel di mobile.
func planRowActions(base string, p PlanRow) g.Node {
	id := strconv.FormatInt(p.ID, 10)
	toggle := planRetireForm(base, id)
	if !p.Active {
		toggle = planActivateForm(base, id)
	}
	return h.Div(
		h.Class("flex flex-wrap items-center gap-2"),
		h.A(h.Href(base+"/plans/"+id+"/edit"), h.Class("btn btn-ghost btn-xs min-h-11"),
			g.Text("Sunting")),
		toggle,
	)
}

func planRetireForm(base, id string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/plans/"+id+"/retire"),
		h.Button(h.Type("submit"), h.Class("btn btn-ghost btn-xs text-warning min-h-11"),
			g.Text("Pensiunkan")),
	)
}

func planActivateForm(base, id string) g.Node {
	return h.FormEl(
		h.Method("post"), h.Action(base+"/plans/"+id+"/activate"),
		h.Button(h.Type("submit"), h.Class("btn btn-ghost btn-xs text-success min-h-11"),
			g.Text("Aktifkan")),
	)
}

// ── Form buat/sunting plan ───────────────────────────────────────────────────

// PlanFormFields = nilai prefill form (semua string agar view netral terhadap
// tipe DB). Kosong (buat) atau terisi (sunting).
type PlanFormFields struct {
	PlanName         string
	PlanCode         string
	PlanCategory     string
	Description      string
	BasePrice        string
	BillingFrequency string
	SetupFee         string
	// Currency TAK di sini (BL-90): tak lagi field form; handler menetapkan
	// IDR konstan. Kolom DB plans.currency tetap ada (lihat PlanRow.Currency
	// untuk tampilan katalog).
	IncludedFeatures string
}

// PlanFormView = data halaman form. Action = URL POST tujuan. Categories &
// BillingOptions dioper handler (view tak memutuskan enum).
type PlanFormView struct {
	Base           string
	Action         string
	IsEdit         bool
	Err            string
	Fields         PlanFormFields
	Categories     []string
	BillingOptions []string
}

// PlanForm merender halaman form lengkap (native POST → 303, gotcha #16).
func PlanForm(v PlanFormView) g.Node {
	title, submit := "Tambah Plan", "Simpan Plan"
	if v.IsEdit {
		title, submit = "Sunting Plan", "Simpan Perubahan"
	}
	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text(title)),
			h.A(h.Href(v.Base+"/plans"), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke katalog")),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "plan-form-err", g.Text(v.Err)))
	}
	body = append(body, h.FormEl(
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),
		formCard("Identitas",
			field("Nama Plan", "plan_name", v.Fields.PlanName, true, "text"),
			field("Kode (SKU)", "plan_code", v.Fields.PlanCode, true, "text"),
			selectField("Kategori", "plan_category", v.Fields.PlanCategory, v.Categories, true),
			textareaField("Deskripsi", "description", v.Fields.Description),
		),
		formCard("Harga",
			moneyField("Harga Dasar", "base_price", v.Fields.BasePrice),
			moneyField("Biaya Setup", "setup_fee", v.Fields.SetupFee),
			selectField("Siklus Tagih", "billing_frequency", v.Fields.BillingFrequency, v.BillingOptions, false),
			// Mata Uang TAK dirender (BL-90): selalu IDR, ditetapkan handler
			// (defaultPlanCurrency). Kolom plans.currency dipertahankan untuk
			// kesiapan multi-currency, hanya tak lagi bisa disunting user.
		),
		formCard("Fitur",
			textareaField("Fitur Termasuk", "included_features", v.Fields.IncludedFeatures),
		),
		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text(submit)),
			h.A(h.Href(v.Base+"/plans"), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))
	// BL-89: Harga Dasar & Biaya Setup (moneyField, data-numgroup) memuat
	// numgroup.js — reformat() membuang non-digit pada TIAP input (bukan sekadar
	// format saat submit), jadi huruf tak pernah mengendap di desktop. Same-origin
	// CSP-safe (gotcha #16). Backend cleanThousands/optNumeric tetap penjaga tanpa
	// JS. Sebelumnya form ini alpa memuat skrip → field terasa menerima non-angka.
	body = append(body, h.Script(h.Src("/static/numgroup.js"), h.Defer()))
	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}
