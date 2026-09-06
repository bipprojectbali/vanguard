package handler

import "strings"

// accounts_form_parse.go — parseAccountForm: baca & validasi field form Account
// (Desa) menjadi accountForm tervalidasi. Dipisah dari accounts_form.go (const
// batas, enum, tipe accountForm) agar tiap file di bawah ambang Route/Handler
// (150). Batas panjang & enum tetap punya SATU sumber di accounts_form.go.
// parseAccountForm membaca & memvalidasi form. Mengembalikan (form, "") bila
// sah, atau (zero, kode) yang dipetakan wsErrMsg. Nilai user-controlled →
// validasi backend adalah penjaga sesungguhnya; atribut form hanya jaring klien.
func parseAccountForm(fv func(string) string) (accountForm, string) {
	var f accountForm

	f.VillageName = strings.TrimSpace(fv("village_name"))
	if f.VillageName == "" || len(f.VillageName) > maxVillageNameLen {
		return accountForm{}, "village_name"
	}

	f.AccountType = strings.TrimSpace(fv("account_type"))
	if _, ok := validAccountTypes[f.AccountType]; !ok {
		return accountForm{}, "account_type"
	}

	// Status & klasifikasi opsional: kosong = NULL (belum diisi); terisi wajib
	// salah satu nilai enum.
	if s := strings.TrimSpace(fv("village_status")); s != "" {
		if _, ok := validVillageStatuses[s]; !ok {
			return accountForm{}, "village_status"
		}
		f.VillageStatus = &s
	}
	if s := strings.TrimSpace(fv("village_classification")); s != "" {
		if _, ok := validClassifications[s]; !ok {
			return accountForm{}, "classification"
		}
		f.VillageClassification = &s
	}

	// Angka opsional: kosong = NULL; terisi wajib bilangan bulat non-negatif
	// (jumlah penduduk/dusun tak bisa negatif).
	pop, code := optInt32(fv("population"))
	if code != "" {
		return accountForm{}, code
	}
	f.Population = pop
	ham, code := optInt32(fv("hamlets_count"))
	if code != "" {
		return accountForm{}, code
	}
	f.HamletsCount = ham

	// Anggaran (APBDes): kosong = NULL; terisi wajib angka sah. BL-60: input jadi
	// UANG bulat → buang pemisah ribuan (cleanThousands) sebelum parse, agar
	// "5.000.000" (dari numgroup.js maupun ketikan manual) & digit polos sama
	// sahnya, dan tanpa JS pun form tetap jalan.
	budget, code := optNumeric(cleanThousands(fv("village_budget")), "budget")
	if code != "" {
		return accountForm{}, code
	}
	f.VillageBudget = budget

	// Kecamatan: FK ke master regions (0009), bukan lagi teks bebas. <select>
	// bernilai ID dipopulasikan cascading di client (static/regions.js) — di sini
	// cukup parse & pastikan bentuknya ID sah; keberadaannya di DB dijaga FK
	// (pelanggaran → SQLSTATE 23503, ditangani createAccount/updateAccount).
	did, code := optInt64(fv("district_id"))
	if code != "" {
		return accountForm{}, "district_id"
	}
	f.DistrictID = did

	// entity_code (kode sistem) OPSIONAL: kosong → nil (dibuat otomatis di
	// create); terisi → override manual, dibatasi panjangnya (keunikan dijaga
	// index DB, dicek saat INSERT). Hanya bermakna di jalur create — update tak
	// menyentuh entity_code. BL-60: input override DILEPAS dari UI (form tak lagi
	// mengirim entity_code → selalu nil → selalu otomatis); jalur parse+override
	// ini SENGAJA dipertahankan (masih teruji, dipakai seed/test) — penghapusan
	// bersifat UI-only.
	if s := strings.TrimSpace(fv("entity_code")); s != "" {
		if len(s) > maxEntityCodeLen {
			return accountForm{}, "entity_code"
		}
		f.EntityCode = &s
	}

	// Teks bebas opsional: trim, kosong → NULL.
	f.Website = optTrim(fv("website"))
	f.Description = optTrim(fv("description"))
	f.VillageAddress = optTrim(fv("village_address"))
	f.PostalCode = optTrim(fv("postal_code"))
	f.Territory = optTrim(fv("territory"))
	f.ContactPhone = optTrim(fv("contact_phone"))
	f.OfficePhone = optTrim(fv("office_phone"))
	f.OfficeEmail = optTrim(fv("office_email"))

	return f, ""
}
