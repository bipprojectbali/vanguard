package handler

import (
	"strconv"
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

// optProbability mengurai probabilitas deal (0–100) opsional: kosong → (nil,
// "");  terisi wajib bilangan bulat DALAM 0–100 → (&v, ""); di luar itu → (nil,
// "probability"). Batas dipaksa di sini SEBELUM DB (cermin deals_probability_chk
// migrasi 00009); *int16 karena kolom SMALLINT nullable.
func optProbability(s string) (*int16, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, ""
	}
	n, err := strconv.ParseInt(s, 10, 16)
	if err != nil || n < 0 || n > 100 {
		return nil, "probability"
	}
	v := int16(n)
	return &v, ""
}

// probabilityStr memformat *int16 opsional untuk tampilan/isian ulang: nil → "".
func probabilityStr(p *int16) string {
	if p == nil {
		return ""
	}
	return strconv.FormatInt(int64(*p), 10)
}

// optNumeric mengurai nilai mata uang opsional (estimated_value / amount):
// kosong → (NULL, ""); terisi wajib desimal sah → (Numeric valid, ""); tak
// terurai → (zero, code). code dioper pemanggil ("estimated"/"amount") agar
// pesan galat menyebut field yang benar. Meniru parse village_budget.
func optNumeric(s, code string) (pgtype.Numeric, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Numeric{Valid: false}, ""
	}
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		return pgtype.Numeric{}, code
	}
	return n, ""
}

// formatRupiah memformat nilai NUMERIC → "Rp 5.000.000" (pemisah ribuan titik,
// gaya Indonesia), desimal dibuang untuk tampilan ringkas. NULL/invalid → "".
// Bukan i18n penuh — cukup agar angka pipeline terbaca manusia; masking F4
// (maskARR) memutuskan tampil/sembunyi TERPISAH dari format ini.
func formatRupiah(n pgtype.Numeric) string {
	raw := numericStr(n)
	if raw == "" {
		return ""
	}
	// Buang bagian desimal — tampilan pipeline tak perlu sen.
	intPart := raw
	if i := strings.IndexByte(intPart, '.'); i >= 0 {
		intPart = intPart[:i]
	}
	neg := strings.HasPrefix(intPart, "-")
	intPart = strings.TrimPrefix(intPart, "-")
	if intPart == "" {
		return ""
	}
	grouped := groupThousands(intPart)
	if neg {
		return "Rp -" + grouped
	}
	return "Rp " + grouped
}

// groupThousands menyisipkan titik tiap tiga digit dari kanan ("5000000" →
// "5.000.000"). Input diasumsikan digit murni (dipanggil setelah tanda & desimal
// dilucuti). Manual, bukan paket lokal — 12 baris ketimbang dependency.
func groupThousands(digits string) string {
	n := len(digits)
	if n <= 3 {
		return digits
	}
	var b strings.Builder
	// Sisa depan yang panjangnya < 3 sebelum grup pertama.
	head := n % 3
	if head == 0 {
		head = 3
	}
	b.WriteString(digits[:head])
	for i := head; i < n; i += 3 {
		b.WriteByte('.')
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}
