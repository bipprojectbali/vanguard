package handler

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgtype"
)

// pagecursor_dynamic.go — varian cursor keyset utk daftar sort-per-kolom arah
// DINAMIS (bisa asc ATAU desc lewat ?dir=). Dipisah dari pagecursor.go agar
// file itu di bawah ambang tipe Route/Handler (150); memakai parseCursor,
// formatCursor, dan pageSize dari sana (satu paket).

// pageCursorTimestamp membaca ?after= untuk daftar sort-per-kolom kolom
// timestamptz NOT NULL dengan arah DINAMIS (bisa asc ATAU desc) — beda dari
// pageCursor/pageCursorAsc yang arahnya TETAP (sentinel ±Infinity halaman
// pertama, tak bisa dipakai kolom yang arahnya berubah-ubah lewat ?dir=).
// hasCursor=false = halaman pertama (cermin pageCursorText/sortcursor.go);
// query SQL melewati predikat keyset sepenuhnya, BUKAN pakai sentinel
// infinity. Format cursor SAMA persis parseCursor/formatCursor (unix nano) —
// timestamptz tak butuh hex-encoding sortcursor.go (bukan teks bebas).
func pageCursorTimestamp(r *http.Request) (at pgtype.Timestamptz, id int64, hasCursor bool) {
	raw := r.URL.Query().Get("after")
	if raw == "" {
		return pgtype.Timestamptz{}, 0, false
	}
	at, id, ok := parseCursor(raw)
	if !ok {
		return pgtype.Timestamptz{}, 0, false
	}
	return at, id, true
}

// splitPageTimestamp = varian splitPage utk cursor timestamptz arah dinamis
// (cermin splitPageText/sortcursor.go).
func splitPageTimestamp[T any](rows []T, keyOf func(T) (pgtype.Timestamptz, int64)) ([]T, string) {
	if len(rows) <= pageSize {
		return rows, ""
	}
	rows = rows[:pageSize]
	at, id := keyOf(rows[len(rows)-1])
	return rows, formatCursor(at, id)
}
