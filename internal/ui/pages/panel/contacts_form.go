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
			contactAccountPickerField(v.Accounts),
		))
	}
	form = append(form,
		formCard("Identitas",
			field("Nama Depan", "first_name", v.Fields.FirstName, true, "text"),
			field("Nama Belakang", "last_name", v.Fields.LastName, false, "text"),
			field("Sapaan", "salutation", v.Fields.Salutation, false, "text"),
			field("Jabatan", "job_title", v.Fields.JobTitle, false, "text"),
			enumFieldHinted("Kategori Perangkat Desa", "position_category", v.Fields.PositionCategory, v.Positions, false, contactPositionLegend),
			enumFieldHinted("Peran", "contact_role", v.Fields.ContactRole, v.Roles, false, contactRoleLegend),
			field("Periode Menjabat", "term_period", v.Fields.TermPeriod, false, "text"),
		),
		formCard("Kontak",
			contactPhoneField(v.PhoneEditable, "HP (Pribadi)", "mobile_phone", v.Fields.MobilePhone),
			contactPhoneField(v.PhoneEditable, "WhatsApp", "whatsapp_number", v.Fields.WhatsappNumber),
			field("Telepon Kantor", "office_phone", v.Fields.OfficePhone, false, "tel"),
			field("Email", "email", v.Fields.Email, false, "email"),
			enumFieldHinted("Kanal Pilihan", "preferred_channel", v.Fields.PreferredChannel, v.Channels, false, contactChannelLegend),
		),
		formCard("Alamat",
			field("Alamat", "mailing_address", v.Fields.MailingAddress, false, "text"),
			field("Kota", "city", v.Fields.City, false, "text"),
			field("Kode Pos", "postal_code", v.Fields.PostalCode, false, "text"),
		),
		formCard("Penanda & Kepatuhan",
			checkboxHintField("Kontak Utama desa", "is_primary_contact", v.Fields.IsPrimaryContact,
				"Penghubung utama desa (maks. satu per desa). Dipakai sebagai kontak default saat lead dikonversi jadi Deal."),
			checkboxHintField("Kontak Teknis", "is_technical_contact", v.Fields.IsTechnicalContact,
				"Narahubung untuk urusan teknis/implementasi produk."),
			checkboxHintField("Opt-out email", "email_opt_out", v.Fields.EmailOptOut,
				"Tandai bila kontak menolak email. Jadi acuan untuk tak mengirim email ke kontak ini."),
			checkboxHintField("Jangan hubungi", "do_not_contact", v.Fields.DoNotContact,
				"Tandai bila kontak minta tak dihubungi sama sekali. Jadi acuan untuk menghentikan semua kontak keluar & tindak lanjut."),
		),

		h.Div(
			h.Class("flex flex-wrap items-center gap-2"),
			h.Button(h.Type("submit"), h.Class("btn btn-primary min-h-11"), g.Text(submit)),
			h.A(h.Href(backHref), h.Class("btn btn-ghost min-h-11"), g.Text("Batal")),
		),
	)
	body = append(body, h.FormEl(form...))

	// Pemilih desa induk bisa diketik/dicari (BL-5) — interaksi typeahead di file
	// same-origin (CSP script-src 'self', pola regions.js). HANYA mode global yang
	// merender pemilih itu, jadi skrip pun hanya dimuat di sana.
	if isGlobal {
		body = append(body, h.Script(h.Src("/static/accountpicker.js"), h.Defer()))
	}

	return h.Div(h.Class("grid gap-4 min-w-0"), g.Group(body))
}

// contactAccountPickerField merender pemilih desa induk yang BISA DIKETIK/DICARI
// (BL-5), menggantikan <select> polos yang sulit dinavigasi saat desa banyak.
// CSP-safe tanpa lib pihak-ketiga: native <datalist> + input teks, dipandu
// static/accountpicker.js (same-origin, script-src 'self').
//
// Dua kontrol, satu tampak satu tersembunyi:
//   - input teks tampak (data-account-search) TAK bernama → tak ter-submit
//     sendiri; hanya alat ketik/cari yang memfilter <datalist>. required =
//     jaring klien agar tak submit kosong.
//   - input hidden name="account_id" (data-account-value) = nilai SEBENARNYA
//     yang ter-submit; diisi accountpicker.js dari data-account-id opsi yang
//     namanya cocok persis. Ketikan yang tak cocok opsi → hidden kosong +
//     setCustomValidity → submit ditahan (backend tetap penjaga: loadOwnedAccount
//     → 404 di luar cakupan, jadi memalsu id tak menembus F3).
func contactAccountPickerField(accounts []AccountOption) g.Node {
	opts := make([]TypeaheadOption, 0, len(accounts))
	for _, a := range accounts {
		opts = append(opts, TypeaheadOption{
			Value: strconv.FormatInt(a.ID, 10),
			Label: a.Name,
		})
	}
	return accountTypeaheadField("account_id", "Desa", opts, true, true)
}

// contactPhoneField = nomor pribadi (HP/WhatsApp). Bila tak boleh disunting (bukan
// Sales), field dikunci menampilkan nilai tersamar + keterangan, dan TANPA name
// agar tak terkirim — handler juga mempertahankan nomor asli, jadi mask tak pernah
// menimpa data. (HP Kontak modul Account sendiri sudah dilepas dari FLS di BL-106;
// mask per-peran kini khusus Kontak/Lead.) label & name variabel (dua nomor di form).
func contactPhoneField(editable bool, label, name, val string) g.Node {
	if editable {
		return field(label, name, val, false, "tel")
	}
	roID := "f-" + name + "_ro"
	return h.Div(
		h.Class("grid gap-1 min-w-0"),
		// Keterangan "nomor tersamar" dipindah ke balik ikon ⓘ tap (BL-101, pola
		// BL-65) agar tak jadi baris teks statis yang menuhi form — konsisten
		// dgn legenda enum di form ini.
		labelWithHint(label, roID, false,
			[]string{"Nomor disamarkan & hanya bisa disunting oleh Sales."}),
		ui.Input(
			h.ID(roID), h.Type("tel"), h.Value(val),
			h.Disabled(), h.Class("input text-base w-full"),
		),
	)
}

// Legenda makna opsi enum kontak (BL-4; disajikan lewat ikon ⓘ tap sejak BL-101
// via enumFieldHinted). Tiap pasangan
// {nilai, makna} HARUS himpunan yang sama dengan opsi enum di
// contacts_helpers.go (contactPositionOptions/RoleOptions/ChannelOptions);
// dijaga uji render TestContactForm_LegendaEnum.

// contactPositionLegend = makna kategori jabatan pemerintah desa.
var contactPositionLegend = [][2]string{
	{"Kepala Desa", "Pemimpin desa; pengambil keputusan tertinggi."},
	{"Sekdes", "Sekretaris Desa; koordinator administrasi."},
	{"Kaur", "Kepala Urusan (tata usaha, keuangan, perencanaan)."},
	{"Kasi", "Kepala Seksi (pemerintahan, kesejahteraan, pelayanan)."},
	{"Operator", "Pengelola sistem/aplikasi desa sehari-hari."},
	{"Bendahara", "Pemegang kas; urusan pembayaran."},
	{"BPD", "Badan Permusyawaratan Desa; unsur pengawas."},
	{"Lainnya", "Jabatan lain di luar daftar."},
}

// contactRoleLegend = peran kontak dalam keputusan pembelian (buying role).
var contactRoleLegend = [][2]string{
	{"Decision Maker", "Pemegang keputusan akhir pembelian."},
	{"Influencer", "Memengaruhi keputusan, tapi bukan penentu."},
	{"User", "Pengguna langsung produk sehari-hari."},
	{"Finance", "Mengurus anggaran & pembayaran."},
	{"Gatekeeper", "Penjaga akses ke pengambil keputusan."},
}

// contactChannelLegend = kanal yang diutamakan untuk menghubungi kontak.
var contactChannelLegend = [][2]string{
	{"WhatsApp", "Utamakan menghubungi lewat WhatsApp."},
	{"Telepon", "Utamakan menghubungi lewat telepon."},
	{"Email", "Utamakan menghubungi lewat email."},
	{"Kunjungan", "Utamakan menemui langsung (kunjungan)."},
}
