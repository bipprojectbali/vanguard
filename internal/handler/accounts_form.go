package handler

import (
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// accounts_form.go — parsing & validasi form desa (Account), dipakai bersama
// oleh jalur create dan update. Dipisah dari handler aksi agar aturan validasi
// (enum, panjang, angka) punya SATU tempat: create & edit tak boleh menerima
// nilai yang berbeda sahnya untuk kolom yang sama.
//
// Enum di sini adalah CERMIN CHECK constraint migrasi 00005 (accounts_type_chk,
// village_status_chk, classification_chk) — penolakan terjadi di sini SEBELUM
// menyentuh DB; CHECK adalah jaring terakhir, bukan yang pertama. Berbeda nilai
// antara Go dan DB = baris valid yang ditolak DB dengan galat yang menyesatkan.

const maxVillageNameLen = 200

// maxEntityCodeLen = batas panjang override manual kode sistem (entity_code).
// Kolomnya TEXT (tak dibatasi DB), tapi kode adalah label pendek yang dikutip
// manusia ("cek DESA-014") — 32 karakter memberi ruang cukup tanpa mengundang
// "kode" sepanjang paragraf.
const maxEntityCodeLen = 32

// Nilai enum sah — set (map→struct kosong) agar cek keanggotaan O(1) & niatnya
// terbaca. Urutan untuk dropdown ada di accountEnumOptions (slice terpisah,
// karena map tak berurutan).
var (
	validAccountTypes = map[string]struct{}{
		"prospect": {}, "customer": {}, "former_customer": {},
	}
	validVillageStatuses = map[string]struct{}{
		"Desa": {}, "Kelurahan": {}, "Nagari": {}, "Gampong": {},
	}
	validClassifications = map[string]struct{}{
		"Mandiri": {}, "Maju": {}, "Berkembang": {},
		"Tertinggal": {}, "Sangat Tertinggal": {},
	}
)

// accountForm = nilai form desa yang SUDAH divalidasi & siap dipetakan ke
// Create/UpdateAccountParams. Kolom opsional bertipe pointer (nil = NULL);
// village_budget pgtype.Numeric (nil-valid = NULL).
type accountForm struct {
	VillageName string
	// EntityCode = override kode sistem OPSIONAL (nil = auto). village_code
	// (Kemendagri) TAK lagi field form — dibuat otomatis dari Kecamatan di jalur
	// create (accounts_codes.go).
	EntityCode            *string
	AccountType           string
	Website               *string
	Description           *string
	DistrictID            *int64
	VillageAddress        *string
	PostalCode            *string
	Territory             *string
	VillageStatus         *string
	VillageClassification *string
	Population            *int32
	HamletsCount          *int32
	VillageBudget         pgtype.Numeric
	ContactPhone          *string
	OfficePhone           *string
	OfficeEmail           *string
}

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

	// Anggaran (APBDes): kosong = NULL; terisi wajib angka desimal sah.
	if s := strings.TrimSpace(fv("village_budget")); s != "" {
		var n pgtype.Numeric
		if err := n.Scan(s); err != nil {
			return accountForm{}, "budget"
		}
		f.VillageBudget = n
	}

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
	// menyentuh entity_code.
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
