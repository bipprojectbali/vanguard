package handler

import (
	"strings"
)

// contacts_form.go — parsing & validasi form kontak (orang di dalam sebuah desa),
// dipakai bersama jalur create & update. Dipisah dari handler aksi agar aturan
// validasi (enum wajib, panjang) punya SATU tempat: create & edit tak boleh
// menerima nilai yang berbeda sahnya untuk kolom yang sama.
//
// Enum di sini = CERMIN CHECK constraint migrasi 00005 (contacts_position_chk,
// contacts_role_chk, contacts_channel_chk) — penolakan terjadi di sini SEBELUM
// menyentuh DB; CHECK adalah jaring terakhir, bukan yang pertama. Nilai berbeda
// antara Go dan DB = baris valid yang ditolak DB dengan galat yang menyesatkan.

const maxContactNameLen = 200

// Nilai enum sah — set (map→struct kosong), cek keanggotaan O(1). Urutan untuk
// dropdown ada di slice terpisah (contactPositionOptions dll., map tak berurutan).
var (
	validContactPositions = map[string]struct{}{
		"Kepala Desa": {}, "Sekdes": {}, "Kaur": {}, "Kasi": {},
		"Operator": {}, "Bendahara": {}, "BPD": {}, "Lainnya": {},
	}
	validContactRoles = map[string]struct{}{
		"Decision Maker": {}, "Influencer": {}, "User": {},
		"Finance": {}, "Gatekeeper": {},
	}
	validContactChannels = map[string]struct{}{
		"WhatsApp": {}, "Telepon": {}, "Email": {}, "Kunjungan": {},
	}
)

// contactForm = nilai form kontak yang SUDAH divalidasi & siap dipetakan ke
// Create/UpdateContactParams. Kolom opsional bertipe pointer (nil = NULL).
// account_id TIDAK di sini — ia datang dari URL (desa induk), bukan form.
type contactForm struct {
	FirstName          string
	LastName           *string
	Salutation         *string
	JobTitle           *string
	PositionCategory   *string
	ContactRole        *string
	IsPrimaryContact   bool
	IsTechnicalContact bool
	TermPeriod         *string
	MobilePhone        *string
	WhatsappNumber     *string
	OfficePhone        *string
	Email              *string
	PreferredChannel   *string
	MailingAddress     *string
	City               *string
	PostalCode         *string
	EmailOptOut        bool
	DoNotContact       bool
}

// parseContactForm membaca & memvalidasi form. Mengembalikan (form, "") bila sah,
// atau (zero, kode) yang dipetakan wsErrMsg. Nilai user-controlled → validasi
// backend adalah penjaga sesungguhnya; atribut form hanya jaring klien.
func parseContactForm(fv func(string) string) (contactForm, string) {
	var f contactForm

	f.FirstName = strings.TrimSpace(fv("first_name"))
	if f.FirstName == "" || len(f.FirstName) > maxContactNameLen {
		return contactForm{}, "first_name"
	}

	// Enum opsional: kosong = NULL (belum diisi); terisi wajib salah satu nilai.
	if s := strings.TrimSpace(fv("position_category")); s != "" {
		if _, ok := validContactPositions[s]; !ok {
			return contactForm{}, "contact_position"
		}
		f.PositionCategory = &s
	}
	if s := strings.TrimSpace(fv("contact_role")); s != "" {
		if _, ok := validContactRoles[s]; !ok {
			return contactForm{}, "contact_role"
		}
		f.ContactRole = &s
	}
	if s := strings.TrimSpace(fv("preferred_channel")); s != "" {
		if _, ok := validContactChannels[s]; !ok {
			return contactForm{}, "contact_channel"
		}
		f.PreferredChannel = &s
	}

	// Teks bebas opsional: trim, kosong → NULL.
	f.LastName = optTrim(fv("last_name"))
	f.Salutation = optTrim(fv("salutation"))
	f.JobTitle = optTrim(fv("job_title"))
	f.TermPeriod = optTrim(fv("term_period"))
	f.MobilePhone = optTrim(fv("mobile_phone"))
	f.WhatsappNumber = optTrim(fv("whatsapp_number"))
	f.OfficePhone = optTrim(fv("office_phone"))
	f.Email = optTrim(fv("email"))
	f.MailingAddress = optTrim(fv("mailing_address"))
	f.City = optTrim(fv("city"))
	f.PostalCode = optTrim(fv("postal_code"))

	// Boolean checkbox: hadir (bernilai apa pun) = true, absen = false. opt-out &
	// do-not-contact writable (kontrol manual), sama seperti penanda primary/teknis.
	f.IsPrimaryContact = optBool(fv("is_primary_contact"))
	f.IsTechnicalContact = optBool(fv("is_technical_contact"))
	f.EmailOptOut = optBool(fv("email_opt_out"))
	f.DoNotContact = optBool(fv("do_not_contact"))

	return f, ""
}

// optBool membaca nilai checkbox: hadir & tak kosong → true. Checkbox HTML tak
// mengirim apa pun saat tak dicentang, jadi absen = false secara alami.
func optBool(s string) bool {
	return strings.TrimSpace(s) != ""
}
