package handler

import (
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"
)

// sortcursor.go — cursor keyset BERTIPE TEKS (BL-157a), utk daftar yang diurut
// per kolom pilihan-user (mis. Subscriptions by village_name), bukan created_at.
// Cermin pagecursor.go (filosofi sama: cursor tak ditandatangani, rusak → halaman
// pertama) tapi generik atas STRING, bukan pgtype.Timestamptz.
//
// Format: hex(value) + cursorSep("_") + id — HEX, bukan base64: alfabet base64
// URL-safe memuat '_' yang bentrok dgn cursorSep (parseCursorText memotong di
// cursorSep PERTAMA via strings.Cut, base64 "-_" bisa memuat '_' di tengah nilai
// yang membuat potongan salah). Alfabet hex (0-9a-f) tak pernah bentrok cursorSep
// atau pagerTrailSep ("~", ui/pager.go).

// pageCursorText membaca ?after= untuk daftar sort-per-kolom. hasCursor=false =
// halaman pertama (?after= kosong/rusak) — BUKAN sentinel nilai minimum/maksimum
// string, yang mustahil digeneralisasi lintas kolom teks. Query SQL memakai flag
// ini utk melewati predikat keyset sepenuhnya di halaman pertama.
func pageCursorText(r *http.Request) (value string, id int64, hasCursor bool) {
	raw := r.URL.Query().Get("after")
	if raw == "" {
		return "", 0, false
	}
	value, id, ok := parseCursorText(raw)
	if !ok {
		return "", 0, false
	}
	return value, id, true
}

func parseCursorText(raw string) (value string, id int64, ok bool) {
	hexVal, idRaw, found := strings.Cut(raw, cursorSep)
	if !found {
		return "", 0, false
	}
	b, err := hex.DecodeString(hexVal)
	if err != nil {
		return "", 0, false
	}
	id, err = strconv.ParseInt(idRaw, 10, 64)
	if err != nil {
		return "", 0, false
	}
	return string(b), id, true
}

// formatCursorText merakit cursor dari baris TERAKHIR halaman ini (cermin
// formatCursor).
func formatCursorText(value string, id int64) string {
	return hex.EncodeToString([]byte(value)) + cursorSep + strconv.FormatInt(id, 10)
}

// splitPageText = varian splitPage utk cursor bertipe teks (cermin splitPage,
// baris lebih pageSize+1 hanya penanda "masih ada lagi?", tak ikut dirender).
func splitPageText[T any](rows []T, keyOf func(T) (string, int64)) ([]T, string) {
	if len(rows) <= pageSize {
		return rows, ""
	}
	rows = rows[:pageSize]
	val, id := keyOf(rows[len(rows)-1])
	return rows, formatCursorText(val, id)
}

// ── Varian NULLABLE (BL-157a fase lanjutan: Paket/MRR/Renewal Date/CSM) ──
//
// Kolom nullable butuh SATU bit tambahan di cursor: "apakah baris cursor ini
// bernilai NULL". Tanpa itu, predikat keyset di query SQL (WHERE) tak bisa
// tahu harus melanjutkan dari kelompok NULL atau kelompok non-NULL (ORDER BY
// sendiri tak perlu berubah — default NULLS Postgres, ASC=akhir/DESC=awal,
// sudah sesuai keputusan user).
//
// Format: "<flag><hexvalue>_<id>" — flag '0'=non-null (diikuti hex value),
// '1'=null (hexvalue KOSONG, nilai tak bermakna). Flag sengaja dalam alfabet
// hex (0/1 ⊂ 0-9a-f) dan digabung LANGSUNG ke hexvalue (bukan token
// terpisah) — jadi bagian "at" hasil strings.Cut tetap satu blob hex biasa,
// validTrailToken/isHexDigits (ui/pager.go) TAK PERLU diubah lagi.

// pageCursorTextNullable membaca ?after= untuk daftar sort kolom NULLABLE.
func pageCursorTextNullable(r *http.Request) (value string, id int64, isNull, hasCursor bool) {
	raw := r.URL.Query().Get("after")
	if raw == "" {
		return "", 0, false, false
	}
	value, id, isNull, ok := parseCursorTextNullable(raw)
	if !ok {
		return "", 0, false, false
	}
	return value, id, isNull, true
}

func parseCursorTextNullable(raw string) (value string, id int64, isNull, ok bool) {
	flagAndHex, idRaw, found := strings.Cut(raw, cursorSep)
	if !found || flagAndHex == "" {
		return "", 0, false, false
	}
	flag := flagAndHex[0]
	if flag != '0' && flag != '1' {
		return "", 0, false, false
	}
	b, err := hex.DecodeString(flagAndHex[1:])
	if err != nil {
		return "", 0, false, false
	}
	id, err = strconv.ParseInt(idRaw, 10, 64)
	if err != nil {
		return "", 0, false, false
	}
	return string(b), id, flag == '1', true
}

// formatCursorTextNullable merakit cursor dari baris TERAKHIR halaman ini
// (cermin formatCursorText). Saat isNull, value diabaikan (hexvalue kosong) —
// tak ada apa pun bermakna untuk disimpan selain "baris ini NULL".
func formatCursorTextNullable(value string, id int64, isNull bool) string {
	if isNull {
		return "1" + cursorSep + strconv.FormatInt(id, 10)
	}
	return "0" + hex.EncodeToString([]byte(value)) + cursorSep + strconv.FormatInt(id, 10)
}

// splitPageTextNullable = varian splitPageText utk kolom nullable.
func splitPageTextNullable[T any](rows []T, keyOf func(T) (value string, id int64, isNull bool)) ([]T, string) {
	if len(rows) <= pageSize {
		return rows, ""
	}
	rows = rows[:pageSize]
	val, id, isNull := keyOf(rows[len(rows)-1])
	return rows, formatCursorTextNullable(val, id, isNull)
}
