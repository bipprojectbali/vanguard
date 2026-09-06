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
	// Desa (Account). BL-67: desa dipilih dari master Kemendagri (regions level
	// 4) lewat VillageID — SAMA seperti AccountCreate langsung — bukan lagi nama
	// teks bebas + Kecamatan. village_code/village_name/district_id DITURUNKAN
	// handler dari VillageID (GetVillageRegion), tak diketik operator.
	VillageID   *int64
	AccountType string
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
// deal (sumber sama: validAccountTypes, maxDealNameLen, maxContactNameLen) agar
// konversi tak menerima nilai yang ditolak create biasa.
func parseConvertForm(fv func(string) string) (convertForm, string) {
	var f convertForm

	// Desa/Kelurahan (BL-67): <select name="village_id"> level 4 dari cascading
	// (static/regions.js lazy-fetch), SAMA pola parseAccountForm. Di sini cukup
	// pastikan bentuk ID sah; WAJIB-nya & keberadaan level 4 diverifikasi handler
	// (LeadConvert) via GetVillageRegion — sejajar AccountCreate.
	vid, code := optInt64(fv("village_id"))
	if code != "" {
		return convertForm{}, "village_id"
	}
	f.VillageID = vid

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

	// Teks bebas opsional: trim, kosong → NULL.
	f.LastName = optTrim(fv("last_name"))
	f.JobTitle = optTrim(fv("job_title"))
	f.MobilePhone = optTrim(fv("mobile_phone"))
	f.Whatsapp = optTrim(fv("whatsapp_number"))
	f.Email = optTrim(fv("email"))

	return f, ""
}
