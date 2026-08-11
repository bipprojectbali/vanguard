// Package codes merumuskan kode-unik lookup entitas CRM (mis. DESA-001,
// DEAL-042) — bentuk & default-nya, tanpa menyentuh DB.
//
// Kenapa paket terpisah & MURNI (tanpa pgx): perakitan kode ("prefix + separator
// + angka ber-padding") adalah aturan tampilan yang harus SAMA di titik
// pembuatan (db) dan titik tampilan/validasi (handler). Menaruhnya di satu tempat
// bebas-DB membuatnya bisa diuji tanpa database dan mustahil bercabang jadi dua
// versi kebenaran — pola yang sama dengan internal/settings (nilai) vs pemanggil
// (DB-nya).
//
// Kode BEDA dari kode resmi eksternal (mis. accounts.village_code = Kode
// Kemendagri): yang ini diberikan sistem untuk lookup internal, satu lagi datang
// dari pemerintah. Jangan campur keduanya.
package codes

import "strconv"

// Entity = jenis objek yang diberi kode. Konstanta, bukan string bebas: nilainya
// juga jadi CHECK di DB (migrasi 00006), jadi menambah entitas = ubah DUA tempat
// dengan sengaja (di sini + CHECK), bukan menyelinap lewat typo yang senyap.
type Entity string

const (
	EntityAccount      Entity = "account"      // Desa (HUB) — DESA-xxx
	EntityLead         Entity = "lead"         // LEAD-xxx (M4)
	EntityDeal         Entity = "deal"         // DEAL-xxx (M4)
	EntityQuote        Entity = "quote"        // QUO-xxx  (M4)
	EntityTicket       Entity = "ticket"       // TIKET-xxx (M6)
	EntitySubscription Entity = "subscription" // SUB-xxx  (M5)
)

// allEntities = sumber tunggal daftar entitas valid. Dipakai Valid() dan bisa
// dipakai UI untuk merender daftar format. Urutannya = urutan tampil.
var allEntities = []Entity{
	EntityAccount, EntityLead, EntityDeal,
	EntityQuote, EntityTicket, EntitySubscription,
}

// AllEntities mengembalikan salinan daftar entitas berkode (untuk UI pengaturan).
func AllEntities() []Entity {
	out := make([]Entity, len(allEntities))
	copy(out, allEntities)
	return out
}

// Valid melaporkan apakah e termasuk entitas berkode yang dikenal. Divalidasi di
// Go SEBELUM DB (CHECK constraint adalah jaring terakhir, bukan yang pertama).
func Valid(e Entity) bool {
	for _, x := range allEntities {
		if x == e {
			return true
		}
	}
	return false
}

// Batas bentuk format — CERMIN CHECK di migrasi 00006. Divalidasi di sini supaya
// input operator ditolak dengan pesan jelas sebelum menyentuh DB, bukan meledak
// sebagai galat constraint yang tak ramah.
const (
	MaxPrefixLen    = 16
	MaxSeparatorLen = 3
	MinPadding      = 0
	MaxPadding      = 12
)

// Format = aturan perakitan kode untuk satu entitas di satu workspace.
// Prefix + Separator + angka (di-pad kiri dengan '0' hingga selebar Padding).
// Padding 0 = tanpa padding (angka apa adanya).
type Format struct {
	Prefix    string
	Separator string
	Padding   int
}

// defaults = format bawaan per entitas, dipakai bila workspace belum menyetel
// (baris code_formats absen). Angkanya konservatif: prefix pendek yang lazim
// diucapkan, padding 3 (001..999 lalu melebar sendiri — Render tak memotong).
var defaults = map[Entity]Format{
	EntityAccount:      {Prefix: "DESA", Separator: "-", Padding: 3},
	EntityLead:         {Prefix: "LEAD", Separator: "-", Padding: 3},
	EntityDeal:         {Prefix: "DEAL", Separator: "-", Padding: 3},
	EntityQuote:        {Prefix: "QUO", Separator: "-", Padding: 3},
	EntityTicket:       {Prefix: "TIKET", Separator: "-", Padding: 4},
	EntitySubscription: {Prefix: "SUB", Separator: "-", Padding: 4},
}

// DefaultFormat mengembalikan format bawaan untuk e. Entitas tak dikenal jatuh ke
// bentuk netral tapi tetap berguna (prefix "REC") alih-alih panik: pemanggil di
// jalur pembuatan tak boleh gagal total hanya karena entitas belum punya default
// — lebih baik kode "jelek tapi ada" daripada create yang batal.
func DefaultFormat(e Entity) Format {
	if f, ok := defaults[e]; ok {
		return f
	}
	return Format{Prefix: "REC", Separator: "-", Padding: 3}
}

// Render merakit kode akhir dari format + nomor urut (seq ≥ 1).
//
// Angka di-pad kiri dengan '0' sampai selebar Padding, TAPI tak pernah DIPOTONG:
// begitu seq melebihi lebar padding (mis. 1000 dengan padding 3) kodenya melebar
// jadi DESA-1000, bukan terpotong jadi DESA-000. Memotong = dua entitas berbeda
// berbagi kode, kegagalan terburuk untuk sebuah penanda unik.
func (f Format) Render(seq int64) string {
	num := strconv.FormatInt(seq, 10)
	for len(num) < f.Padding {
		num = "0" + num
	}
	return f.Prefix + f.Separator + num
}

// ValidFormat melaporkan apakah f masuk akal untuk disimpan (cermin CHECK 00006).
// Prefix wajib ada — kode tanpa prefix cuma angka, tak bisa dibedakan antar
// entitas saat di-lookup, yang justru satu-satunya gunanya.
func ValidFormat(f Format) bool {
	if l := len(f.Prefix); l < 1 || l > MaxPrefixLen {
		return false
	}
	if len(f.Separator) > MaxSeparatorLen {
		return false
	}
	if f.Padding < MinPadding || f.Padding > MaxPadding {
		return false
	}
	return true
}
