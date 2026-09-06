package handler

import (
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
	// VillageName & DistrictID TAK lagi berasal dari form (BL-66) — keduanya
	// DITURUNKAN handler dari VillageID (master regions level 4): nama = regions.name,
	// district = regions.parent_region_id, village_code = regions.code. Field
	// tetap ada di struct sbg wadah nilai turunan yang dioper ke Create/Update.
	VillageName string
	// VillageID = id Desa/Kelurahan (regions level 4) terpilih. Wajib saat create
	// (village_code Kemendagri diturunkan darinya); saat update nil = pertahankan
	// desa lama (form edit desa legacy yang dropdown-nya tak bisa preselect).
	VillageID *int64
	// EntityCode = override kode sistem OPSIONAL (nil = auto). village_code
	// (Kemendagri) TAK lagi field form — diturunkan dari Desa terpilih (VillageID).
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
