package handler

import (
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_format_number.go — parse/format nilai numerik Sales: probabilitas (%),
// numeric generik, & Rupiah (ribuan bertitik). Dipisah dari sales_format.go
// (helper tanggal/waktu) agar tiap file di bawah ambang Route/Handler (150).
// Semua fungsi murni (tak sentuh DB/ctx); satu paket dengan pemanggilnya.
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
