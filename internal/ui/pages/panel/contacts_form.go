package panel

import (
	"strconv"

	"go_starter/internal/ui"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// contacts_form.go — form buat/sunting kontak. Form NATIVE POST → 303 (gotcha
// #16). Validasi sesungguhnya di backend (parseContactForm); atribut di sini
// hanya jaring klien. Nilai enum dioper handler (Positions/Roles/Channels).
//
// F4: PhoneEditable=false (bukan Sales) → field HP & WhatsApp dikunci: nilainya
// tersamar & TANPA name agar tak terkirim — handler juga mempertahankan nomor
// asli, jadi mask tak pernah menimpa data. Telepon kantor tak pernah dikunci.

// ContactFormFields = nilai prefill form (edit) atau kosong (buat). Semua string
// agar view netral terhadap tipe DB.
type ContactFormFields struct {
	FirstName          string
	LastName           string
	Salutation         string
	JobTitle           string
	PositionCategory   string
	ContactRole        string
	IsPrimaryContact   bool
	IsTechnicalContact bool
	TermPeriod         string
	MobilePhone        string
	WhatsappNumber     string
	OfficePhone        string
	Email              string
	PreferredChannel   string
	MailingAddress     string
	City               string
	PostalCode         string
	EmailOptOut        bool
	DoNotContact       bool
}

// AccountOption = satu <option> desa untuk pemilih desa induk di form global.
// Value = id desa (dikirim sebagai account_id), Label = nama desa.
type AccountOption struct {
	ID   int64
	Name string
}

// ContactFormView = data halaman form kontak. Action = URL POST tujuan.
// AccountBase = URL desa induk (tautan batal/kembali). IsEdit mengubah
// judul/label. PhoneEditable menentukan apakah field HP/WhatsApp terkunci.
//
// DUA jalur pemakaian:
//   - NESTED (dari detail desa): AccountBase & AccountName terisi, Accounts nil →
//     desa induk sudah pasti, tak ada pemilih; tautan kembali ke desa itu.
//   - GLOBAL (dari daftar kontak): AccountBase kosong, Accounts terisi → render
//     dropdown <select name="account_id">; ListHref = tautan kembali ke daftar
//     kontak global (AccountBase kosong tak bisa merakit tautan desa).
type ContactFormView struct {
	AccountBase string
	AccountName string
	Action      string
	IsEdit      bool
	Err         string
	Fields      ContactFormFields

	PhoneEditable bool

	// Global-only: pemilih desa induk. Accounts non-kosong → mode global.
	Accounts []AccountOption
	ListHref string

	Positions []string
	Roles     []string
	Channels  []string
}

// ContactForm merender halaman form lengkap.
func ContactForm(v ContactFormView) g.Node {
	title := "Tambah Kontak"
	submit := "Simpan Kontak"
	if v.IsEdit {
		title = "Sunting Kontak"
		submit = "Simpan Perubahan"
	}

	// Mode global (dari daftar kontak): desa induk BELUM dipilih → dropdown +
	// tautan kembali ke daftar kontak global. Mode nested (dari detail desa):
	// desa sudah pasti → subjudul menyebut namanya, tautan kembali ke desa itu.
	isGlobal := len(v.Accounts) > 0
	subtitle := "Kontak untuk " + v.AccountName + "."
	backHref := v.AccountBase + "/contacts"
	if isGlobal {
		subtitle = "Pilih desa induk lalu isi data kontak."
		backHref = v.ListHref
	}

	body := []g.Node{
		h.Div(
			h.H1(h.Class("text-xl font-semibold"), g.Text(title)),
			h.P(h.Class("text-sm text-base-content/60"), g.Text(subtitle)),
			h.A(h.Href(backHref), h.Class("text-sm text-base-content/60"),
				g.Text("« Kembali ke daftar kontak")),
		),
	}
	if v.Err != "" {
		body = append(body, ui.Alert(ui.VariantDestructive, "contact-form-err", g.Text(v.Err)))
	}

	form := []g.Node{
		h.Method("post"), h.Action(v.Action),
		h.Class("grid gap-4 min-w-0"),
	}
	// Selektor desa induk HANYA di mode global — di atas kartu Identitas agar
	// "kontak ini milik desa mana" diputuskan lebih dulu. account_id divalidasi
	// ulang backend (loadOwnedAccount → 404 bila di luar cakupan).
	if isGlobal {
		form = append(form, formCard("Desa Induk",
			contactAccountSelectField(v.Accounts),
		))
	}
	form = append(form,
		formCard("Identitas",
			field("Nama Depan", "first_name", v.Fields.FirstName, true, "text"),
			field("Nama Belakang", "last_name", v.Fields.LastName, false, "text"),
			field("Sapaan", "salutation", v.Fields.Salutation, false, "text"),
			field("Jabatan", "job_title", v.Fields.JobTitle, false, "text"),
			selectField("Jabatan (Kategori)", "position_category", v.Fields.PositionCategory, v.Positions, false),
			selectField("Peran", "contact_role", v.Fields.ContactRole, v.Roles, false),
			field("Periode Menjabat", "term_period", v.Fields.TermPeriod, false, "text"),
		),
		formCard("Kontak",
			contactPhoneField(v.PhoneEditable, "HP (Pribadi)", "mobile_phone", v.Fields.MobilePhone),
			contactPhoneField(v.PhoneEditable, "WhatsApp", "whatsapp_number", v.Fields.WhatsappNumber),
			field("Telepon Kantor", "office_phone", v.Fields.OfficePhone, false, "tel"),
			field("Email", "email", v.Fields.Email, false, "email"),
			selectField("Kanal Pilihan", "preferred_channel", v.Fields.PreferredChannel, v.Channels, false),
		),
		formCard("Alamat",
			field("Alamat Surat", "mailing_address", v.Fields.MailingAddress, false, "text"),
			field("Kota", "city", v.Fields.City, false, "text"),
			field("Kode Pos", "postal_code", v.Fields.PostalCode, false, "text"),
		),
		formCard("Penanda & Kepatuhan",
			checkboxField("Kontak Utama desa", "is_primary_contact", v.Fields.IsPrimaryContact),
			checkboxField("Kontak Teknis", "is_technical_contact", v.Fields.IsTechnicalContact),
			checkboxField("Opt-out email (jangan kirim email)", "email_opt_out", v.Fields.EmailOptOut),
			checkboxField("Jangan hubungi", "do_not_contact", v.Fields.DoNotContact),
		),

		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text(submit)),
			h.A(h.Href(backHref), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	)
	body = append(body, h.FormEl(form...))

	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// contactAccountSelectField merender dropdown desa induk (mode global). required: satu
// desa WAJIB dipilih (kontak selalu milik sebuah desa). Placeholder disabled
// mencegah submit tanpa memilih; backend tetap memvalidasi (loadOwnedAccount).
func contactAccountSelectField(accounts []AccountOption) g.Node {
	nodes := []g.Node{
		h.Option(h.Value(""), h.Disabled(), h.Selected(), g.Text("— Pilih desa —")),
	}
	for _, a := range accounts {
		nodes = append(nodes, h.Option(
			h.Value(strconv.FormatInt(a.ID, 10)), g.Text(a.Name),
		))
	}
	return h.Div(
		h.Class("grid gap-1 min-w-0 sm:col-span-2"),
		labelFor("Desa", "f-account_id", true),
		h.Select(
			append([]g.Node{
				h.ID("f-account_id"), h.Name("account_id"),
				h.Class("select text-base w-full"), h.Required(),
			}, g.Group(nodes))...,
		),
	)
}

// contactPhoneField = nomor pribadi (HP/WhatsApp). Bila tak boleh disunting (bukan
// Sales), field dikunci menampilkan nilai tersamar + keterangan, dan TANPA name
// agar tak terkirim — handler juga mempertahankan nomor asli, jadi mask tak pernah
// menimpa data. Sejajar phoneField di accounts_form.go, tapi label & name variabel
// (dua nomor pribadi di form kontak).
func contactPhoneField(editable bool, label, name, val string) g.Node {
	if editable {
		return field(label, name, val, false, "tel")
	}
	roID := "f-" + name + "_ro"
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		labelFor(label, roID, false),
		ui.Input(
			h.ID(roID), h.Type("tel"), h.Value(val),
			h.Disabled(), h.Class("input text-base w-full"),
		),
		h.P(h.Class("text-xs text-base-content/60"),
			g.Text("Nomor disamarkan & hanya bisa disunting oleh Sales.")),
	)
}

// checkboxField = satu boolean. Checkbox HTML tak mengirim apa pun saat tak
// dicentang, jadi absen = false secara alami (parseContactForm.optBool). Tap
// target ≥44px lewat padding label. Nilai "1" hanya penanda kehadiran.
func checkboxField(label, name string, checked bool) g.Node {
	attrs := []g.Node{
		h.ID("f-" + name), h.Name(name), h.Type("checkbox"),
		h.Value("1"), h.Class("checkbox"),
	}
	if checked {
		attrs = append(attrs, h.Checked())
	}
	return h.Label(
		h.For("f-"+name),
		h.Class("flex items-center gap-2 min-h-11 cursor-pointer sm:col-span-2"),
		h.Input(attrs...),
		h.Span(h.Class("text-sm"), g.Text(label)),
	)
}
