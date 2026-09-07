package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	data "maragu.dev/gomponents-datastar"
	h "maragu.dev/gomponents/html"
)

// leadUnqualified = nilai status yang mengaktifkan field "Alasan Unqualified"
// (BL-80). Cermin salah satu validLeadStatuses (handler); dipakai merakit
// ekspresi data-show agar satu perubahan nilai enum tak menyisakan ekspresi basi.
const leadUnqualified = "Unqualified"

// unqualifiedReasonShowExpr = ekspresi data-show field "Alasan Unqualified":
// tampil hanya saat $leadstatus == "Unqualified". Dirakit dari const (bukan
// literal terpisah) — sejajar pola onboardingProgressShowExpr (CS form).
const unqualifiedReasonShowExpr = "$leadstatus == '" + leadUnqualified + "'"

// sales_leads_form.go — form buat/sunting lead. Form NATIVE POST → 303 (gotcha
// #16). Validasi sesungguhnya di backend (parseLeadForm); atribut di sini hanya
// jaring klien. Enum (status/rating) dioper handler. Reuse helper formCard/
// field/selectField/textareaField dari accounts_form.go (satu paket panel).

// LeadFormFields = nilai prefill form (edit) atau kosong (buat). Semua string
// agar view netral terhadap tipe DB.
type LeadFormFields struct {
	LeadName          string
	ContactPerson     string
	JobTitle          string
	LeadSource        string
	LeadStatus        string
	Rating            string
	UnqualifiedReason string
	EstimatedValue    string
	DistrictID        string
	MobilePhone       string
	Whatsapp          string
	Email             string
}

// LeadFormView = data halaman form. Action = URL POST tujuan. IsEdit mengubah
// judul/label. Statuses/Ratings = opsi enum dropdown (dari handler).
type LeadFormView struct {
	Base     string
	Action   string
	IsEdit   bool
	Err      string
	Fields   LeadFormFields
	Statuses []string
	Ratings  []string

	// RegionsJSON = dataset penuh master wilayah (h.regionsJSON), diembed sekali
	// utk cascading dropdown Provinsi/Kabupaten-Kota/Kecamatan (ADR 0009).
	RegionsJSON string
}

// LeadForm merender halaman form lengkap.
func LeadForm(v LeadFormView) g.Node {
	title := "Tambah Lead"
	submit := "Simpan Lead"
	if v.IsEdit {
		title = "Sunting Lead"
		submit = "Simpan Perubahan"
	}

	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text(title)),
			h.A(h.Href(v.Base+"/leads"), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke daftar lead")),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "lead-form-err", g.Text(v.Err)))
	}

	body = append(body, h.FormEl(
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),
		// BL-80: signal $leadstatus menggerakkan tampil/sembunyi "Alasan
		// Unqualified" (data-show). Diinisialisasi dari nilai TERSIMPAN agar
		// no-FOUC saat prefill (edit lead Unqualified → field langsung tampak).
		// Efemeral (state form), bukan data dikirim ke server.
		data.Signals(map[string]any{"leadstatus": v.Fields.LeadStatus}),

		formCard("Identitas Lead",
			field("Nama Lead", "lead_name", v.Fields.LeadName, true, "text"),
			field("Kontak (nama orang)", "contact_person", v.Fields.ContactPerson, false, "text"),
			field("Jabatan", "job_title", v.Fields.JobTitle, false, "text"),
			field("Sumber Lead", "lead_source", v.Fields.LeadSource, false, "text"),
		),
		formCard("Kualifikasi",
			// BL-69: legenda makna Status/Rating pindah ke balik ikon ⓘ tap-friendly
			// di label (enumFieldHinted, pola BL-65) — bukan baris statis di bawah
			// select yang memaksa field melebar. Form Tambah Lead kini seragam dgn
			// form Tambah Desa.
			//
			// BL-80: Status pakai leadStatusSelect (varian enumFieldHinted yang
			// di-bind ke signal $leadstatus) agar field "Alasan Unqualified" bisa
			// muncul/lenyap mengikuti pilihan status tanpa round-trip.
			leadStatusSelect(v.Fields.LeadStatus, v.Statuses),
			enumFieldHinted("Rating", "rating", v.Fields.Rating, v.Ratings, false, leadRatingLegend),
			moneyField("Nilai Estimasi (Rp)", "estimated_value", v.Fields.EstimatedValue),
			// BL-80: alasan unqualified HANYA bermakna saat Status = Unqualified —
			// disembunyikan (data-show) untuk status lain agar tak menyesatkan.
			// data-show = jaring UX klien (display:none, field TETAP terkirim);
			// backend (parseLeadForm) tetap penegak: nilai basi dibuang saat status
			// ≠ Unqualified.
			showWhen(unqualifiedReasonShowExpr, "sm:col-span-2 min-w-0",
				textareaField("Alasan Unqualified", "unqualified_reason", v.Fields.UnqualifiedReason)),
		),
		formCard("Lokasi & Kontak",
			regionSelect("lead", v.RegionsJSON, v.Fields.DistrictID, false),
			phoneNumField("HP", "mobile_phone", v.Fields.MobilePhone),
			phoneNumField("WhatsApp", "whatsapp", v.Fields.Whatsapp),
			field("Email", "email", v.Fields.Email, false, "email"),
		),

		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text(submit)),
			h.A(h.Href(v.Base+"/leads"), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	))
	// Cascading dropdown wilayah (regionSelect di atas cuma menanam data + markup;
	// interaksi berjenjangnya di sini, same-origin CSP-safe, gotcha #12).
	body = append(body, h.Script(h.Src("/static/regions.js"), h.Defer()))
	// Pengelompokan ribuan utk input uang (data-numgroup): memformat tampilan &
	// menormalkan jadi digit polos saat submit. Same-origin CSP-safe (gotcha #16).
	body = append(body, h.Script(h.Src("/static/numgroup.js"), h.Defer()))

	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// leadStatusSelect — dropdown "Status" lead yang di-bind ke signal $leadstatus
// (data.Bind) sehingga memilih nilai men-toggle field "Alasan Unqualified" tanpa
// round-trip (BL-80). Selain binding, identik enumFieldHinted("Status", …,
// required=true, leadStatusLegend): label + legenda makna di balik ikon ⓘ (BL-69),
// opsi dari enumOptions (satu sumber, tanpa blank karena wajib). Dibuat manual
// karena enumFieldHinted tak menyuntikkan atribut Datastar — cermin
// onboardingStatusSelect (customer_success_form.go).
func leadStatusSelect(current string, opts []string) g.Node {
	sel := []g.Node{
		h.ID("f-lead_status"), h.Name("lead_status"),
		data.Bind("leadstatus"),
		h.Class("select text-base w-full"),
		h.Required(),
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelWithLegend("Status", "f-lead_status", true, leadStatusLegend),
		h.Select(append(sel, g.Group(enumOptions(current, opts, false)))...),
	)
}

// leadStatusLegend = makna tiap status lead (BL-3). Urut = alur kualifikasi
// (New → Contacted → Qualified, atau bercabang ke Unqualified). 'Converted' tak
// di sini: itu status sistem hasil konversi, tak bisa dipilih manual (lihat
// parseLeadForm). HARUS himpunan yang sama dengan leadStatusOptions.
var leadStatusLegend = [][2]string{
	{"New", "Baru masuk, belum dihubungi."},
	{"Contacted", "Sudah dihubungi, belum dikualifikasi."},
	{"Qualified", "Cocok dan siap dikonversi jadi Deal."},
	{"Unqualified", "Tak cocok atau tak berminat (isi alasannya)."},
}

// leadRatingLegend = makna tiap rating minat (BL-3). Urut dari paling panas.
// HARUS himpunan yang sama dengan leadRatingOptions.
var leadRatingLegend = [][2]string{
	{"Hot", "Minat tinggi, siap closing."},
	{"Warm", "Tertarik, perlu tindak lanjut."},
	{"Cold", "Belum tertarik atau prioritas rendah."},
}
