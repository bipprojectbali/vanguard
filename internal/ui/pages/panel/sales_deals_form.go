package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_deals_form.go — form buat/sunting deal. Form NATIVE POST → 303 (gotcha
// #16). Validasi sesungguhnya di backend (parseDealForm); atribut di sini hanya
// jaring klien. Enum (type/term) & pilihan desa dioper handler. Stage TAK ada di
// form: deal baru lahir 'Prospecting', perpindahan lewat aksi stage di detail.
// Reuse helper formCard/field/selectField/textareaField dari accounts_form.go.

// DealFormFields = nilai prefill form (edit) atau kosong (buat). Semua string agar
// view netral terhadap tipe DB. AccountID = id desa terpilih (string).
type DealFormFields struct {
	DealName          string
	AccountID         string
	DealType          string
	Amount            string
	Probability       string
	ExpectedCloseDate string
	ForecastCategory  string
	NextStep          string
	SubscriptionTerm  string
	Competitor        string
}

// DealFormView = data halaman form. Action = URL POST tujuan. IsEdit mengubah
// judul/label. Types/Terms = opsi enum; Accounts = kandidat desa (F3-scoped di handler).
type DealFormView struct {
	Base     string
	Action   string
	IsEdit   bool
	Err      string
	Fields   DealFormFields
	Types    []string
	Terms    []string
	Accounts []AccountMemberOption
}

// DealForm merender halaman form lengkap.
func DealForm(v DealFormView) g.Node {
	title := "Tambah Deal"
	submit := "Simpan Deal"
	if v.IsEdit {
		title = "Sunting Deal"
		submit = "Simpan Perubahan"
	}

	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text(title)),
			h.A(h.Href(v.Base+"/deals"), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke daftar deal")),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "deal-form-err", g.Text(v.Err)))
	}

	body = append(body, h.FormEl(
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),

		formCard("Identitas Deal",
			field("Nama Deal", "deal_name", v.Fields.DealName, true, "text"),
			accountSelectField("Desa", "account_id", v.Fields.AccountID, v.Accounts),
			selectField("Tipe Deal", "deal_type", v.Fields.DealType, v.Types, false),
			selectField("Termin Langganan", "subscription_term", v.Fields.SubscriptionTerm, v.Terms, false),
		),
		formCard("Nilai & Peluang",
			moneyField("Nilai (Rp)", "amount", v.Fields.Amount),
			field("Probabilitas (%)", "probability", v.Fields.Probability, false, "number"),
			field("Perkiraan Tutup", "expected_close_date", v.Fields.ExpectedCloseDate, false, "date"),
			field("Kategori Forecast", "forecast_category", v.Fields.ForecastCategory, false, "text"),
		),
		formCard("Catatan",
			field("Kompetitor", "competitor", v.Fields.Competitor, false, "text"),
			textareaField("Langkah Berikutnya", "next_step", v.Fields.NextStep),
		),

		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text(submit)),
			h.A(h.Href(v.Base+"/deals"), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))
	// Pengelompokan ribuan utk Nilai (Rp) (data-numgroup di moneyField): memformat
	// tampilan & menormalkan jadi digit polos saat submit. Same-origin CSP-safe
	// (gotcha #16). Sejajar sales_leads_form (BL-2/BL-8).
	body = append(body, h.Script(h.Src("/static/numgroup.js"), h.Defer()))

	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// accountSelectField = dropdown WAJIB pilih desa (deal selalu menempel ke satu
// desa). Berbeda dari memberSelect (opsional): opsi pertama = placeholder disabled
// agar submit kosong tertangkap jaring klien, backend tetap penjaga sesungguhnya.
func accountSelectField(label, name, current string, accounts []AccountMemberOption) g.Node {
	placeholder := []g.Node{h.Value(""), h.Disabled(), g.Text("— Pilih desa —")}
	if current == "" {
		placeholder = append(placeholder, h.Selected())
	}
	nodes := []g.Node{h.Option(placeholder...)}
	for _, a := range accounts {
		id := strconv.FormatInt(a.ID, 10)
		attrs := []g.Node{h.Value(id)}
		if id == current {
			attrs = append(attrs, h.Selected())
		}
		nodes = append(nodes, h.Option(append(attrs, g.Text(a.Label))...))
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelFor(label, "f-"+name, true),
		h.Select(
			append([]g.Node{
				h.ID("f-" + name), h.Name(name), h.Required(),
				h.Class("select text-base w-full"),
			}, g.Group(nodes))...,
		),
	)
}
