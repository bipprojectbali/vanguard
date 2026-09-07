package panel

import (
	"encoding/json"
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
	// TermMonths = Termin Langganan → jumlah bulan-kontrak (sumber:
	// handler.dealTermMonths, cermin termContractMonths). Ditanam sebagai JSON
	// untuk preview MRR/ARR di klien (BL-87 opsi c). Kosong/nil = preview mati
	// (fallback aman: form tetap berfungsi tanpa preview).
	TermMonths map[string]int
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
			moneyField("Nilai per periode termin (Rp)", "amount", v.Fields.Amount),
			field("Probabilitas (%)", "probability", v.Fields.Probability, false, "number"),
			field("Perkiraan Tutup", "expected_close_date", v.Fields.ExpectedCloseDate, false, "date"),
			field("Kategori Forecast", "forecast_category", v.Fields.ForecastCategory, false, "text"),
			dealValuePreview(),
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
	// Preview MRR/ARR (BL-87 opsi c): peta Termin→bulan-kontrak ditanam sebagai
	// JSON (CSP-safe, gotcha #15 — nilai internal, bukan input user; json.Marshal
	// meng-escape <>&). dealpreview.js membacanya lalu menghitung MRR = nilai ÷
	// bulan, ARR = MRR × 12 (cermin sales_deals_won_subscription). Enhancement
	// murni: tanpa JS / tanpa TermMonths, form tetap jalan (preview diam "—").
	termJSON, _ := json.Marshal(v.TermMonths)
	body = append(body,
		h.Script(h.Type("application/json"), h.ID("deal-term-months"), g.Raw(string(termJSON))),
		h.Script(h.Src("/static/dealpreview.js"), h.Defer()),
	)
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

// dealValuePreview = kotak preview MRR/ARR terhitung (BL-87 opsi c) untuk
// menutup ketaksinkronan makna field Nilai: nilai deal diperlakukan PER-TERMIN
// saat Won→Langganan (MRR = nilai ÷ bulan-kontrak, ARR = MRR × 12), sehingga
// tanpa preview user mudah salah input (mis. niat ARR + Termin Monthly → ARR
// 12× lipat). Span penuh-lebar di dalam formCard (sm:col-span-2). Kontrak markup
// dibaca static/dealpreview.js: [data-deal-preview] wadah, [data-deal-mrr]/
// [data-deal-arr] slot nilai, [data-deal-note] baris keterangan. data-amount-sel/
// data-term-sel = selector input sumber (hindari hardcode id di JS). Fallback
// no-JS aman: nilai diam "—" + note instruksi (preview = enhancement, bukan
// syarat submit).
func dealValuePreview() g.Node {
	valueSlot := func(label, hook string) g.Node {
		return h.Div(
			h.Class("min-w-0"),
			h.Div(h.Class("text-xs text-base-content/60"), g.Text(label)),
			h.Div(h.Class("font-semibold break-words"), g.Attr(hook, ""), g.Text("—")),
		)
	}
	return h.Div(
		h.Class("sm:col-span-2 min-w-0"),
		g.Attr("data-deal-preview", ""),
		g.Attr("data-amount-sel", "#f-amount"),
		g.Attr("data-term-sel", "#f-subscription_term"),
		h.Div(
			h.Class("rounded-box bg-base-200 border border-base-300 p-3 min-w-0"),
			h.Div(
				h.Class("text-xs font-medium text-base-content/70 mb-2"),
				g.Text("Perkiraan langganan saat Closed Won"),
			),
			h.Div(
				h.Class("flex flex-wrap gap-6"),
				valueSlot("MRR", "data-deal-mrr"),
				valueSlot("ARR", "data-deal-arr"),
			),
			h.P(
				h.Class("mt-2 text-xs text-base-content/60 break-words"),
				g.Attr("data-deal-note", ""),
				g.Text("Isi Nilai & pilih Termin Langganan untuk melihat perkiraan MRR/ARR."),
			),
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
