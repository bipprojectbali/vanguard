package handler

import (
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_format.go — parse & format nilai khas modul Sales yang dipakai bersama
// oleh leads & deals (tanggal, probabilitas, mata uang). Dipisah dari gate
// (sales_view.go) & form (sales_*_form.go) agar aturan "kosong = NULL / terisi =
// wajib sah" punya SATU tempat: create & edit leads/deals tak boleh menerima
// nilai yang berbeda sahnya untuk kolom yang sama. Meniru optInt32/optTrim di
// accounts_form.go.

const dateLayout = "2006-01-02" // <input type=date> HTML mengirim ISO ini.

// optDate mengurai tanggal opsional dari form: kosong → (NULL, ""); terisi &
// sah → (Date valid, ""); tak terurai → (zero, "date"). Kolom expected_close /
// closed pada deals nullable — kosong berarti "belum dijadwalkan", bukan galat.
func optDate(s string) (pgtype.Date, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Date{Valid: false}, ""
	}
	t, err := time.Parse(dateLayout, s)
	if err != nil {
		return pgtype.Date{}, "date"
	}
	return pgtype.Date{Time: t, Valid: true}, ""
}

// dateStr memformat pgtype.Date untuk tampilan & pengisian ulang form: NULL →
// "". Format ISO yang sama dengan input agar nilai bisa bolak-balik tanpa
// pergeseran zona (Date tak berzona — murni kalender).
func dateStr(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	return d.Time.Format(dateLayout)
}

// dateTimeLayout = format <input type="datetime-local"> HTML (tanpa detik & zona).
// Diperlakukan sebagai UTC saat disimpan (gotcha #14: simpan UTC) — cukup untuk
// stempel waktu aktivitas v1; agregasi berzona belum diperlukan di sini.
const dateTimeLayout = "2006-01-02T15:04"

// todayInAppTZ = "hari ini" menurut zona aplikasi (appTZ), untuk perbandingan
// date-only kedaluwarsa quote (BL-17). Tak di-truncate ke midnight: pembanding
// dateBefore hanya melihat Y/M/D, komponen jam diabaikan — menghindari skew batas
// hari saat zona non-UTC (bertetangga gotcha #14: simpan UTC, banding berzona).
func todayInAppTZ() time.Time { return time.Now().In(appTZ) }

// dateBefore membandingkan DUA tanggal secara date-only (Y/M/D) di zona masing-
// masing: true bila a jatuh pada kalender SEBELUM b. Dipakai enforce expiration
// quote (BL-17): expiration_date (midnight UTC dari optDate) vs todayInAppTZ.
// Perbandingan kalender (bukan instant) agar "== hari ini" tak terhitung lampau
// walau jam berbeda; hanya STRICTLY-before yang true.
func dateBefore(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	if ay != by {
		return ay < by
	}
	if am != bm {
		return am < bm
	}
	return ad < bd
}

// optDateTime mengurai stempel waktu opsional (mis. activity_at pada Call):
// kosong → (NULL, ""); terisi & sah → (Timestamptz valid, ""); tak terurai →
// (zero, "datetime"). Menerima varian berdetik dari beberapa browser.
func optDateTime(s string) (pgtype.Timestamptz, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Timestamptz{Valid: false}, ""
	}
	t, err := time.Parse(dateTimeLayout, s)
	if err != nil {
		if t, err = time.Parse("2006-01-02T15:04:05", s); err != nil {
			return pgtype.Timestamptz{}, "datetime"
		}
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}, ""
}

// dateTimeStr memformat Timestamptz untuk pengisian ulang <input datetime-local>:
// NULL → "". Format sama dengan input agar nilai bolak-balik tanpa pergeseran.
func dateTimeStr(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.UTC().Format(dateTimeLayout)
}
