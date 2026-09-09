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
	Competitor           string
}

// DealFormView = data halaman form. Action = URL POST tujuan. IsEdit mengubah
// judul/label. Types = opsi enum tipe deal; Accounts = kandidat desa (F3-scoped di
// handler). BL-88: Termin Langganan pindah ke form quote (quote otoritatif) → tak
// ada opsi termin / preview MRR di form deal.
type DealFormView struct {
	Base     string
	Action   string
	IsEdit   bool
	Err    string
	Fields DealFormFields
	Types  []string
	// ForecastCategories = opsi enum Kategori Forecast (BL-124). Dioper handler
	// (forecastCategoryOptions) agar view tetap murni-data.
	ForecastCategories []string
	Accounts           []AccountMemberOption
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
		),
		formCard("Nilai & Peluang",
			moneyField("Nilai perkiraan (Rp)", "amount", v.Fields.Amount),
			field("Probabilitas (%)", "probability", v.Fields.Probability, false, "number"),
			field("Perkiraan Tutup", "expected_close_date", v.Fields.ExpectedCloseDate, false, "date"),
			selectField("Kategori Forecast", "forecast_category", v.Fields.ForecastCategory, v.ForecastCategories, false),
			dealEstimateHint(),
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

// dealEstimateHint = keterangan makna "Nilai perkiraan" (BL-88). Nilai deal kini
// SEMATA forecast pipeline tahap awal (nilai × probabilitas); nilai yang DIAKUI
// (MRR/ARR) diturunkan dari grand_total quote saat quote di-Accept, bukan diketik di
// sini. Menggantikan preview MRR/ARR lama (yang mengasumsikan nilai per-termin +
// Termin di deal — keduanya kini milik quote). Span penuh-lebar di dalam formCard.
func dealEstimateHint() g.Node {
	return h.Div(
		h.Class("sm:col-span-2 min-w-0"),
		h.P(
			h.Class("rounded-box bg-base-200 border border-base-300 p-3 text-xs text-base-content/70 break-words"),
			g.Text("Nilai perkiraan = forecast pipeline tahap awal. Nilai yang diakui "+
				"(MRR/ARR) diturunkan otomatis dari total quote saat quote di-Accept."),
		),
	)
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
	opts := make([]TypeaheadOption, 0, len(accounts))
	for _, a := range accounts {
		opts = append(opts, TypeaheadOption{
			Value: strconv.FormatInt(a.ID, 10),
			Label: a.Label,
		})
	}
	return typeaheadPickerField(typeaheadPickerConfig{
		Name:         "account_id",
		Label:        "Desa",
		ListID:       "account-options",
		Options:      opts,
		Current:      current,
		CurrentLabel: currentLabel,
		Required:     true,
		Placeholder:  "Ketik kode Kemendagri atau nama desa…",
	})
}
