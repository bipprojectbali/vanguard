package handler

import (
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_convert_form.go — parse & validasi form REVIEW konversi (desa + kontak
// utama + deal, satu submit). Dipisah dari sales_convert.go (halaman) &
// sales_convert_action.go (aksi atomik) semata untuk file health.

// convertForm = nilai review konversi yang SUDAH divalidasi. Menggabungkan tiga
// entitas hasil (desa + kontak utama + deal) menjadi satu submit.
type convertForm struct {
	// Desa (Account)
	VillageName string
	AccountType string
	DistrictID  *int64
	// Kontak utama (Contact)
	FirstName   string
	LastName    *string
	JobTitle    *string
	MobilePhone *string
	Whatsapp    *string
	Email       *string
	// Deal
	DealName string
	Amount   pgtype.Numeric
}

// parseConvertForm membaca & memvalidasi form review. (form, "") bila sah, atau
// (zero, kode) yang dipetakan wsErrMsg. Enum & panjang dicermin dari form desa/
// deal (sumber sama: validAccountTypes, maxVillageNameLen, maxDealNameLen,
// maxContactNameLen) agar konversi tak menerima nilai yang ditolak create biasa.
func parseConvertForm(fv func(string) string) (convertForm, string) {
	var f convertForm

	f.VillageName = strings.TrimSpace(fv("village_name"))
	if f.VillageName == "" || len(f.VillageName) > maxVillageNameLen {
		return convertForm{}, "village_name"
	}
	f.AccountType = strings.TrimSpace(fv("account_type"))
	if _, ok := validAccountTypes[f.AccountType]; !ok {
		return convertForm{}, "account_type"
	}

	f.FirstName = strings.TrimSpace(fv("first_name"))
	if f.FirstName == "" || len(f.FirstName) > maxContactNameLen {
		return convertForm{}, "first_name"
	}

	f.DealName = strings.TrimSpace(fv("deal_name"))
	if f.DealName == "" || len(f.DealName) > maxDealNameLen {
		return convertForm{}, "deal_name"
	}
	amt, code := optNumeric(fv("amount"), "amount")
	if code != "" {
		return convertForm{}, code
	}
	f.Amount = amt

	// district_id: sama pola parseAccountForm/parseLeadForm — cukup parse & pastikan
	// bentuknya ID sah; keberadaannya di DB dijaga FK (pelanggaran → SQLSTATE 23503,
	// ditangani accountWriteErr di sales_convert_action.go).
	did, code := optInt64(fv("district_id"))
	if code != "" {
		return convertForm{}, "district_id"
	}
	f.DistrictID = did

	// Teks bebas opsional: trim, kosong → NULL.
	f.LastName = optTrim(fv("last_name"))
	f.JobTitle = optTrim(fv("job_title"))
	f.MobilePhone = optTrim(fv("mobile_phone"))
	f.Whatsapp = optTrim(fv("whatsapp_number"))
	f.Email = optTrim(fv("email"))

	return f, ""
}
