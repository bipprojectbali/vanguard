package handler

import (
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// pagecursor.go — cursor keyset lewat query string (?after=<unixnano>_<id>).
//
// Keyset, BUKAN offset: daftar diurut created_at DESC dan baris baru terus
// masuk di atas, jadi OFFSET akan menggeser isi halaman di antara dua klik —
// user melihat baris yang sama dua kali atau melewatkannya sama sekali. Cursor
// menunjuk POSISI dalam urutan, jadi ia tetap benar walau ada sisipan.
//
// Cursor tak dienkode/ditandatangani: isinya hanya waktu & id baris yang memang
// sudah tampil di halaman ini, dan halamannya sendiri sudah dijaga izin. Yang
// mengarang cursor cuma bisa melompat ke posisi lain di daftar yang sama.

// cursorSep memisahkan dua bagian cursor. Underscore, bukan koma/titik-dua:
// keduanya butuh escaping di query string.
const cursorSep = "_"

// pageCursor membaca ?after= dari request. Kosong/rusak → halaman pertama.
//
// Rusak diperlakukan sebagai halaman pertama, BUKAN error: cursor datang dari
// URL yang bisa disunting, dipendekkan pengirim pesan, atau ditandai buruk oleh
// bookmark lama. Halaman awal selalu jawaban yang benar dan tak menakut-nakuti.
func pageCursor(r *http.Request) (pgtype.Timestamptz, int64) {
	raw := r.URL.Query().Get("after")
	if raw == "" {
		return firstPageCursor()
	}
	at, id, ok := parseCursor(raw)
	if !ok {
		return firstPageCursor()
	}
	return at, id
}

// pageCursorAsc = varian ASC dari pageCursor untuk daftar yang diurut MENAIK
// (mis. Customer Journey: lama-di-fase terpanjang lebih dulu = stage_entry_date
// ASC). Halaman pertama memakai cursor MINIMUM (-Infinity, MinInt64) sehingga
// semua baris lolos syarat > cursor. Sisanya (parse ?after=, toleran rusak)
// identik pageCursor — cursor yang tersimpan sudah berupa nilai finit.
func pageCursorAsc(r *http.Request) (pgtype.Timestamptz, int64) {
	raw := r.URL.Query().Get("after")
	if raw == "" {
		return firstPageCursorAsc()
	}
	at, id, ok := parseCursor(raw)
	if !ok {
		return firstPageCursorAsc()
	}
	return at, id
}

// firstPageCursorAsc mengembalikan cursor keyset halaman pertama untuk urutan
// MENAIK: (stage_entry_date, id) = minimum, sehingga semua baris lolos syarat
// > cursor. Cermin firstPageCursor (dev_users.go) untuk arah sebaliknya.
func firstPageCursorAsc() (pgtype.Timestamptz, int64) {
	return pgtype.Timestamptz{Valid: true, InfinityModifier: pgtype.NegativeInfinity}, math.MinInt64
}

// pageTrail membaca ?trail= mentah dari request — jejak cursor halaman-halaman
// sebelumnya (BL-7), diteruskan apa adanya ke view lalu ke ui.KeysetPager yang
// memparse & memvalidasinya. Dibatasi panjang di sini sebagai pagar pertama
// (helper render juga mem-validasi lenient); nilai kelewat panjang → "" agar tak
// terbawa ke URL halaman berikutnya. Sama filosofi dengan pageCursor: masukan
// dari URL yang bisa disunting tak boleh menggagalkan render.
func pageTrail(r *http.Request) string {
	raw := r.URL.Query().Get("trail")
	if len(raw) > pageTrailMax {
		return ""
	}
	return raw
}

// pageTrailMax = pagar panjang jejak (selaras ui.pagerTrailMax). Jejak jauh lebih
// panjang dari ini hampir pasti diarang, bukan navigasi wajar.
const pageTrailMax = 4096

func parseCursor(raw string) (pgtype.Timestamptz, int64, bool) {
	tsRaw, idRaw, found := strings.Cut(raw, cursorSep)
	if !found {
		return pgtype.Timestamptz{}, 0, false
	}
	nano, err := strconv.ParseInt(tsRaw, 10, 64)
	if err != nil {
		return pgtype.Timestamptz{}, 0, false
	}
	id, err := strconv.ParseInt(idRaw, 10, 64)
	if err != nil {
		return pgtype.Timestamptz{}, 0, false
	}
	return pgtype.Timestamptz{Time: time.Unix(0, nano).UTC(), Valid: true}, id, true
}

// formatCursor merakit cursor dari baris TERAKHIR halaman ini — halaman
// berikutnya dimulai tepat sesudahnya. UnixNano dipakai apa adanya (UTC, sesuai
// konvensi penyimpanan): presisi mikrodetik Postgres muat di dalamnya, jadi
// tak ada baris yang terlewat karena pembulatan.
func formatCursor(at pgtype.Timestamptz, id int64) string {
	if !at.Valid {
		return ""
	}
	return strconv.FormatInt(at.Time.UTC().UnixNano(), 10) + cursorSep + strconv.FormatInt(id, 10)
}

// splitPage memotong hasil query yang sengaja mengambil pageSize+1 baris:
// mengembalikan baris yang ditampilkan + cursor halaman berikutnya ("" bila
// halaman ini yang terakhir).
//
// Baris lebih itu HANYA penanda keberadaan — ia tak ikut dirender. Begitulah
// "masih ada lagi?" dijawab tanpa COUNT terpisah, yang di tabel besar berarti
// memindai seluruh isinya untuk menyalakan satu tombol.
func splitPage[T any](rows []T, keyOf func(T) (pgtype.Timestamptz, int64)) ([]T, string) {
	if len(rows) <= pageSize {
		return rows, ""
	}
	rows = rows[:pageSize]
	at, id := keyOf(rows[len(rows)-1])
	return rows, formatCursor(at, id)
}
