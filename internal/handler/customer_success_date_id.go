package handler

import (
	"fmt"

	"github.com/jackc/pgx/v5/pgtype"
)

// customer_success_date_id.go — format tampil BACA (locale Indonesia) KHUSUS
// halaman detail Customer Success. TERPISAH SENGAJA dari dateStr/dateTimeStr
// (sales_format.go, dipakai Sales & form edit CS): fungsi itu WAJIB tetap ISO
// (layout sama dgn <input type=date>/<input type=datetime-local>) agar nilai
// bolak-balik ke form tanpa pergeseran zona — mengubahnya ke locale ID akan
// merusak SEMUA form date di Sales & CS. Go stdlib time.Format tak punya nama
// bulan Indonesia (9/12 beda dgn Inggris), jadi ditulis manual via tabel bulan.
// NULL → "" (sama pola dateStr/dateTimeStr) — dikonversi "—" saat render oleh
// orDash (accounts.go), bukan di sini, biar satu tempat saja yg atur placeholder.

var bulanID = [...]string{
	"Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

// dateStrID memformat pgtype.Date sebagai "DD MMMM YYYY" (mis. "21 September
// 2026") untuk tampilan BACA halaman detail CS. NULL → "".
func dateStrID(d pgtype.Date) string {
	if !d.Valid {
		return ""
	}
	t := d.Time
	return fmt.Sprintf("%02d %s %d", t.Day(), bulanID[t.Month()-1], t.Year())
}

// dateTimeStrID memformat pgtype.Timestamptz sebagai "DD MMMM YYYY HH:ii"
// (mis. "08 September 2026 06:59") untuk tampilan BACA halaman detail CS.
// NULL → "".
func dateTimeStrID(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	t := ts.Time.UTC()
	return fmt.Sprintf("%02d %s %d %02d:%02d", t.Day(), bulanID[t.Month()-1], t.Year(), t.Hour(), t.Minute())
}
