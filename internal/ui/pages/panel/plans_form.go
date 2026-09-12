package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// plans_form.go — form buat/sunting plan (Modul 5). Tipe & tabel katalog di
// plans.go.

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
		body = append(body, ui.Toast(ui.VariantDestructive, "plan-form-err", g.Text(v.Err)))
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
