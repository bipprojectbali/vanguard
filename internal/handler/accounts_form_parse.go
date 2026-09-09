package handler

import "strings"

// normalizeWebsite mengurai field Website OPSIONAL: trim, kosong → nil. Bila
// terisi tanpa skema (tak memuat "://"), prepend "https://" agar domain telanjang
// (mis. "www.facebook.com") tersimpan sebagai URL sah (BL-110). Nilai yang sudah
// berskema (http/https/ftp/…) dibiarkan apa adanya. Bukan validator URL penuh —
// cukup menutup kasus umum "domain tanpa skema" yang ditolak input type=url.
func normalizeWebsite(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	return &s
}

// accounts_form_parse.go — parseAccountForm: baca & validasi field form Account
// (Desa) menjadi accountForm tervalidasi. Dipisah dari accounts_form.go (const
// batas, enum, tipe accountForm) agar tiap file di bawah ambang Route/Handler
// (150). Batas panjang & enum tetap punya SATU sumber di accounts_form.go.
// parseAccountForm membaca & memvalidasi form. Mengembalikan (form, "") bila
// sah, atau (zero, kode) yang dipetakan wsErrMsg. Nilai user-controlled →
// validasi backend adalah penjaga sesungguhnya; atribut form hanya jaring klien.
func parseAccountForm(fv func(string) string) (accountForm, string) {
	var f accountForm

	// BL-66: Nama Desa & Kecamatan TAK lagi diketik/dipilih langsung — diturunkan
	// handler dari Desa (VillageID) via master regions. Di sini cukup parse
	// village_id sbg ID sah; keberadaannya (level 4) diverifikasi handler
	// (GetVillageRegion). Kosong = biarkan handler memutuskan (wajib saat create,
	// pertahankan lama saat update).

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

	// Desa/Kelurahan (BL-66): <select name="village_id"> level 4 dari cascading
	// (static/regions.js lazy-fetch). Cukup parse bentuk ID sah di sini; handler
	// resolve ke master (GetVillageRegion) utk village_code/nama/district. Kosong
	// → nil (handler: wajib saat create, pertahankan desa lama saat update).
	vid, code := optInt64(fv("village_id"))
	if code != "" {
		return accountForm{}, "village_id"
	}
	f.VillageID = vid

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

	// HP Kontak & Telepon Kantor: nomor OPSIONAL yang WAJIB berupa angka bila diisi
	// (BL-106 — input jadi bertema angka; phonenum.js jaring klien, optPhone penjaga
	// backend). Kosong → nil. Pola sama HP/WhatsApp Lead.
	phone, code := optPhone(fv("contact_phone"), "contact_phone")
	if code != "" {
		return accountForm{}, code
	}
	f.ContactPhone = phone
	office, code := optPhone(fv("office_phone"), "office_phone")
	if code != "" {
		return accountForm{}, code
	}
	f.OfficePhone = office

	// Teks bebas opsional: trim, kosong → NULL.
	// BL-110: Website menerima domain telanjang (www.facebook.com) — normalkan ke
	// URL berskema agar tersimpan valid tanpa memaksa operator mengetik "https://".
	f.Website = normalizeWebsite(fv("website"))
	f.Description = optTrim(fv("description"))
	f.VillageAddress = optTrim(fv("village_address"))
	f.PostalCode = optTrim(fv("postal_code"))
	f.Territory = optTrim(fv("territory"))
	f.OfficeEmail = optTrim(fv("office_email"))

	return f, ""
}
