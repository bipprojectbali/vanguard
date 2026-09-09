package panel

import (
	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// sales_leads_form.go — form buat/sunting PROFIL lead. Form NATIVE POST → 303
// (gotcha #16). Validasi sesungguhnya di backend (parseLeadForm); atribut di sini
// hanya jaring klien. Enum (rating) dioper handler. Reuse helper formCard/field/
// selectField/textareaField dari accounts_form.go (satu paket panel).
//
// BL-83: STATUS lead TAK lagi di form ini — transisi status = aksi tersendiri
// (kartu "Ubah Status" di detail lead, sales_leads_status_control.go). Form profil
// fokus ke identitas/kualifikasi non-transisi (nama/kontak/sumber/rating/nilai/
// lokasi). "Alasan Unqualified" (BL-80) ikut pindah ke kontrol status, karena
// terkopel ke pilihan status.

// LeadFormFields = nilai prefill form (edit) atau kosong (buat). Semua string
// agar view netral terhadap tipe DB. Status & alasan Unqualified TIDAK di sini
// (BL-83: dikelola kontrol status di detail, bukan form profil).
type LeadFormFields struct {
	LeadName       string
	ContactPerson  string
	JobTitle       string
	LeadSource     string
	Rating         string
	EstimatedValue string
	DistrictID     string
	MobilePhone    string
	Email          string
}

// LeadFormView = data halaman form profil. Action = URL POST tujuan. IsEdit
// mengubah judul/label. Ratings = opsi enum rating; Sources = opsi "Sumber Lead"
// (BL-82). Statuses TAK di sini (BL-83: status dikelola kontrol terpisah).
type LeadFormView struct {
	Base    string
	Action  string
	IsEdit  bool
	Err     string
	Fields  LeadFormFields
	Ratings []string
	// Sources = opsi dropdown "Sumber Lead" (BL-82, dari handler). Enum terkunci
	// menggantikan input teks bebas.
	Sources []string

	// PhoneEditable (BL-84): role boleh MENYUNTING nomor HP/WhatsApp (canEditPhone,
	// Sales saja). false → field HP/WA dikunci (disabled, tak ter-submit) menampilkan
	// mask; mencegah bug submit mask "•••" gagal validasi. Simetris AccountFormView.
	PhoneEditable bool

	// RegionsJSON = dataset penuh master wilayah (h.regionsJSON), diembed sekali
	// utk cascading dropdown Provinsi/Kabupaten-Kota/Kecamatan (ADR 0009).
	RegionsJSON string
}

// LeadForm merender halaman form profil lengkap.
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
			// BL-82: Sumber Lead = dropdown enum terkunci (bukan teks bebas).
			// Opsional (required=false → ada opsi kosong "—"); nilai lama di luar
			// himpunan tampil tak-terpilih. Backend (parseLeadForm) menegakkan.
			selectField("Sumber Lead", "lead_source", v.Fields.LeadSource, v.Sources, false),
		),
		formCard("Kualifikasi",
			// BL-69: legenda makna Rating di balik ikon ⓘ tap-friendly (enumFieldHinted).
			// BL-83: "Status" & "Alasan Unqualified" dipindah ke kontrol status di detail
			// lead — kualifikasi non-transisi (rating minat + nilai estimasi) tetap di sini.
			enumFieldHinted("Rating", "rating", v.Fields.Rating, v.Ratings, false, leadRatingLegend),
			moneyField("Nilai Estimasi (Rp)", "estimated_value", v.Fields.EstimatedValue),
		),
		formCard("Lokasi & Kontak",
			regionSelect("lead", v.RegionsJSON, v.Fields.DistrictID, false),
			// HP & WhatsApp digabung jadi satu field (nomor yang sama dipakai untuk
			// keduanya) — kolom whatsapp lama dipertahankan di DB, tak lagi disunting.
			phoneNumField("HP / WhatsApp", "mobile_phone", v.Fields.MobilePhone, v.PhoneEditable),
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
	// BL-81: penyaring live nomor telepon (data-phonenum, HP/WhatsApp) — buang
	// karakter tak-diizinkan saat diketik. Same-origin CSP-safe (gotcha #16).
	body = append(body, h.Script(h.Src("/static/phonenum.js"), h.Defer()))

	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// leadRatingLegend = makna tiap rating minat (BL-3). Urut dari paling panas.
// HARUS himpunan yang sama dengan leadRatingOptions.
var leadRatingLegend = [][2]string{
	{"Hot", "Minat tinggi, siap closing."},
	{"Warm", "Tertarik, perlu tindak lanjut."},
	{"Cold", "Belum tertarik atau prioritas rendah."},
}
