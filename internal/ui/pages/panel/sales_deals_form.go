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
	DealName  string
	AccountID string
	// SelectedAccountLabel = nama desa terpilih (prefill edit). Kosong saat buat.
	// Dipakai mengisi input teks tampak pemilih & menjamin opsi desa terpilih
	// hadir di <datalist> walau di luar batas dealAccountPickerLimit.
	SelectedAccountLabel string
	DealType             string
	Amount               string
	Probability          string
	ExpectedCloseDate    string
	ForecastCategory     string
	NextStep             string
	SubscriptionTerm     string
	Competitor           string
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
			dealAccountPickerField(v.Fields.AccountID, v.Fields.SelectedAccountLabel, v.Accounts),
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
	// Pemilih desa bisa diketik/dicari (BL-9) — typeahead di file same-origin
	// (CSP script-src 'self'), pola & skrip yang SAMA dengan BL-5 (kontrak markup
	// [data-account-picker]). Lihat dealAccountPickerField di bawah.
	body = append(body, h.Script(h.Src("/static/accountpicker.js"), h.Defer()))

	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// dealAccountPickerField = pemilih desa WAJIB yang BISA DIKETIK/DICARI (BL-9),
// menggantikan <select> polos yang sulit dinavigasi saat desa banyak. Pola &
// skrip SAMA dengan BL-5 (kontrak markup [data-account-picker], dipandu
// static/accountpicker.js same-origin — CSP-safe tanpa lib pihak-ketiga).
//
// Dua kontrol, satu tampak satu tersembunyi:
//   - input teks tampak (data-account-search) TAK bernama → tak ter-submit
//     sendiri; hanya alat ketik/cari yang memfilter <datalist>. required =
//     jaring klien agar tak submit kosong.
//   - input hidden name="account_id" (data-account-value) = nilai SEBENARNYA
//     yang ter-submit; diisi accountpicker.js dari data-account-id opsi yang
//     LABEL-nya cocok persis dgn ketikan. Ketikan tak cocok → hidden kosong +
//     setCustomValidity → submit ditahan. Backend tetap penjaga: loadOwnedDeal/
//     loadOwnedAccount → 404 di luar cakupan, memalsu id tak menembus F3.
//
// Prefill EDIT: current = id desa terpilih, currentLabel = namanya. Input teks
// diisi label; hidden diisi id. Opsi desa terpilih DISISIPKAN ke <datalist> bila
// belum ada (bisa di luar batas dealAccountPickerLimit) agar sync klien tak
// mengosongkan id yang sah.
func dealAccountPickerField(current, currentLabel string, accounts []AccountMemberOption) g.Node {
	opts := make([]g.Node, 0, len(accounts)+1)
	inList := false
	for _, a := range accounts {
		id := strconv.FormatInt(a.ID, 10)
		if id == current {
			inList = true
		}
		opts = append(opts, h.Option(
			h.Value(a.Label),
			g.Attr("data-account-id", id),
		))
	}
	// Desa terpilih di luar batas picker → sisipkan agar preselect tetap cocok.
	if current != "" && currentLabel != "" && !inList {
		opts = append(opts, h.Option(
			h.Value(currentLabel),
			g.Attr("data-account-id", current),
		))
	}

	search := []g.Node{
		h.ID("f-account_id_search"), h.Type("text"),
		h.List("account-options"), h.Placeholder("Ketik nama desa…"),
		g.Attr("autocomplete", "off"), g.Attr("data-account-search", ""),
		h.Class("input text-base w-full min-h-11"), h.Required(),
	}
	hidden := []g.Node{
		h.Type("hidden"), h.ID("f-account_id"), h.Name("account_id"),
		g.Attr("data-account-value", ""),
	}
	if current != "" {
		hidden = append(hidden, h.Value(current))
	}
	if currentLabel != "" {
		search = append(search, h.Value(currentLabel))
	}

	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		g.Attr("data-account-picker", ""),
		labelFor("Desa", "f-account_id_search", true),
		ui.Input(search...),
		h.Input(hidden...),
		h.DataList(append([]g.Node{h.ID("account-options")}, opts...)...),
		h.P(h.Class("text-xs text-base-content/60"),
			g.Text("Ketik untuk mencari, lalu pilih desa dari daftar yang muncul.")),
	)
}
