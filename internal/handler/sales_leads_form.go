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
//
// Opsi dropdown (leadStatusOptions/dll.) di sales_leads_form_options.go.

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
	// validLeadSources = sumber lead yang boleh di-set lewat form (BL-82). Enum
	// dikunci di UI (dropdown) & ditegakkan di sini — TANPA CHECK constraint DB
	// (keputusan user: nilai teks-bebas lama tetap di DB apa adanya, hanya
	// disaring saat form disubmit). Nilai disimpan verbatim seperti opsi.
	validLeadSources = map[string]struct{}{
		"Referral": {}, "Event": {}, "Website": {}, "Cold Call": {},
		"Tender": {}, "Dinas PMD": {}, "Lainnya": {},
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
	DistrictID        *int64
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

	// Nilai estimasi (ARR prospektif): kosong = NULL; terisi wajib angka. Pemisah
	// ribuan dibuang dulu (cleanThousands) agar input terkelompok "5.000.000" dari
	// numgroup.js — atau ketikan manual tanpa JS — sama-sama sah (BL-2).
	ev, code := optNumeric(cleanThousands(fv("estimated_value")), "estimated")
	if code != "" {
		return leadForm{}, code
	}
	f.EstimatedValue = ev

	// Kecamatan: FK ke master regions (0009) — lihat parseAccountForm, pola sama.
	did, code := optInt64(fv("district_id"))
	if code != "" {
		return leadForm{}, "district_id"
	}
	f.DistrictID = did

	// Teks bebas opsional: trim, kosong → NULL.
	f.ContactPerson = optTrim(fv("contact_person"))
	f.JobTitle = optTrim(fv("job_title"))
	// Sumber Lead (BL-82): enum terkunci di UI (dropdown), ditegakkan di sini.
	// Kosong = NULL; terisi wajib ∈ himpunan. TANPA CHECK DB — nilai teks-bebas
	// lama dibiarkan tersimpan; penegakan hanya saat form disubmit.
	if s := strings.TrimSpace(fv("lead_source")); s != "" {
		if _, ok := validLeadSources[s]; !ok {
			return leadForm{}, "lead_source"
		}
		f.LeadSource = &s
	}
	f.UnqualifiedReason = optTrim(fv("unqualified_reason"))
	// BL-80: "Alasan Unqualified" hanya bermakna saat status Unqualified. Field
	// yang disembunyikan klien (data-show) untuk status lain TETAP terkirim
	// (data-show = display:none, bukan lepas-DOM) — backend penegak: buang nilai
	// basi agar tak ada "alasan yatim" tersimpan pada lead non-Unqualified.
	if f.LeadStatus != "Unqualified" {
		f.UnqualifiedReason = nil
	}
	f.Email = optTrim(fv("email"))

	// HP/WhatsApp opsional (BL-2): kosong = NULL; terisi wajib "berupa nomor"
	// (digit + opsional '+' prefix + pemisah, 6–20 digit). Format asli
	// dipertahankan (leading zero & +62 utuh); validasi, bukan normalisasi.
	mob, code := optPhone(fv("mobile_phone"), "mobile_phone")
	if code != "" {
		return leadForm{}, code
	}
	f.MobilePhone = mob
	wa, code := optPhone(fv("whatsapp"), "whatsapp")
	if code != "" {
		return leadForm{}, code
	}
	f.Whatsapp = wa

	return f, ""
}
