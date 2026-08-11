package handler

import (
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_leads_form.go — parsing & validasi form Lead, dipakai bersama create &
// update. Dipisah dari handler aksi agar aturan validasi (enum, panjang, angka)
// punya SATU tempat: create & edit tak boleh menerima nilai yang berbeda sahnya
// untuk kolom yang sama. Meniru accounts_form.go.
//
// Enum di sini = CERMIN CHECK constraint migrasi 00009 (leads_status_chk,
// leads_rating_chk) — penolakan terjadi di sini SEBELUM DB; CHECK jaring
// terakhir. 'Converted' SENGAJA tak ditawarkan di form: itu transisi sistem
// (efek konversi lead), bukan status yang di-set manual.

const maxLeadNameLen = 200

var (
	// validLeadStatuses = status yang boleh di-set MANUAL lewat form. 'Converted'
	// tak di sini walau CHECK mengizinkannya — hanya jalur konversi (MarkLeadConverted)
	// yang mencapainya, agar "sudah dikonversi" tak bisa dipalsukan dengan edit.
	validLeadStatuses = map[string]struct{}{
		"New": {}, "Contacted": {}, "Qualified": {}, "Unqualified": {},
	}
	validLeadRatings = map[string]struct{}{
		"Hot": {}, "Warm": {}, "Cold": {},
	}
)

// leadForm = nilai form Lead yang SUDAH divalidasi & siap dipetakan ke
// Create/UpdateLeadParams. Kolom opsional pointer (nil = NULL); estimated_value
// pgtype.Numeric (nil-valid = NULL).
type leadForm struct {
	LeadName          string
	ContactPerson     *string
	JobTitle          *string
	LeadSource        *string
	LeadStatus        string
	Rating            *string
	UnqualifiedReason *string
	EstimatedValue    pgtype.Numeric
	Province          *string
	Regency           *string
	District          *string
	MobilePhone       *string
	Whatsapp          *string
	Email             *string
}

// parseLeadForm membaca & memvalidasi form. (form, "") bila sah, atau (zero,
// kode) yang dipetakan wsErrMsg. Nilai user-controlled → validasi backend adalah
// penjaga sesungguhnya.
func parseLeadForm(fv func(string) string) (leadForm, string) {
	var f leadForm

	f.LeadName = strings.TrimSpace(fv("lead_name"))
	if f.LeadName == "" || len(f.LeadName) > maxLeadNameLen {
		return leadForm{}, "lead_name"
	}

	f.LeadStatus = strings.TrimSpace(fv("lead_status"))
	if f.LeadStatus == "" {
		f.LeadStatus = "New" // default = New (cermin DEFAULT kolom).
	}
	if _, ok := validLeadStatuses[f.LeadStatus]; !ok {
		return leadForm{}, "lead_status"
	}

	// Rating opsional: kosong = NULL; terisi wajib enum.
	if s := strings.TrimSpace(fv("rating")); s != "" {
		if _, ok := validLeadRatings[s]; !ok {
			return leadForm{}, "lead_rating"
		}
		f.Rating = &s
	}

	// Nilai estimasi (ARR prospektif): kosong = NULL; terisi wajib desimal sah.
	ev, code := optNumeric(fv("estimated_value"), "estimated")
	if code != "" {
		return leadForm{}, code
	}
	f.EstimatedValue = ev

	// Teks bebas opsional: trim, kosong → NULL.
	f.ContactPerson = optTrim(fv("contact_person"))
	f.JobTitle = optTrim(fv("job_title"))
	f.LeadSource = optTrim(fv("lead_source"))
	f.UnqualifiedReason = optTrim(fv("unqualified_reason"))
	f.Province = optTrim(fv("province"))
	f.Regency = optTrim(fv("regency"))
	f.District = optTrim(fv("district"))
	f.MobilePhone = optTrim(fv("mobile_phone"))
	f.Whatsapp = optTrim(fv("whatsapp"))
	f.Email = optTrim(fv("email"))

	return f, ""
}

// Opsi enum untuk dropdown — slice BERURUT (map validasi tak berurutan). Nilai
// HARUS himpunan yang sama dengan map validasi & CHECK 00009; urutan untuk tampilan.
var (
	leadStatusOptions = []string{"New", "Contacted", "Qualified", "Unqualified"}
	leadRatingOptions = []string{"Hot", "Warm", "Cold"}
)

// compile-time: opsi & map validasi sepakat (panjang sama). Berbeda = dropdown
// menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(leadStatusOptions) != len(validLeadStatuses) ||
		len(leadRatingOptions) != len(validLeadRatings) {
		panic("leads: opsi enum tak sinkron dengan map validasi")
	}
	return struct{}{}
}()
