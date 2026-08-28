package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

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

		formCard("Identitas Lead",
			field("Nama Lead", "lead_name", v.Fields.LeadName, true, "text"),
			field("Kontak (nama orang)", "contact_person", v.Fields.ContactPerson, false, "text"),
			field("Jabatan", "job_title", v.Fields.JobTitle, false, "text"),
			field("Sumber Lead", "lead_source", v.Fields.LeadSource, false, "text"),
		),
		formCard("Kualifikasi",
			selectField("Status", "lead_status", v.Fields.LeadStatus, v.Statuses, true),
			selectField("Rating", "rating", v.Fields.Rating, v.Ratings, false),
			field("Nilai Estimasi (Rp)", "estimated_value", v.Fields.EstimatedValue, false, "text"),
			textareaField("Alasan Unqualified", "unqualified_reason", v.Fields.UnqualifiedReason),
		),
		formCard("Lokasi & Kontak",
			regionSelect("lead", v.RegionsJSON, v.Fields.DistrictID, false),
			field("HP", "mobile_phone", v.Fields.MobilePhone, false, "tel"),
			field("WhatsApp", "whatsapp", v.Fields.Whatsapp, false, "tel"),
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

	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}
