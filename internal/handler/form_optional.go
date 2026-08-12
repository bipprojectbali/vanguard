package handler

import (
	"strconv"
	"strings"
)

// form_optional.go — helper field FORM OPSIONAL yang dipakai BERSAMA lintas modul
// (accounts, contacts, leads, deals, quotes, activities, convert). Aturan "kosong
// = NULL" harus SATU tempat: tiap modul memperlakukan field opsional dengan cara
// yang sama, jadi helper ini netral-domain dan sengaja tak tinggal di file modul
// mana pun. Lihat mis. parseAccountForm / parseContactForm sebagai pemakainya.

// optTrim mem-trim lalu mengembalikan pointer, atau nil bila kosong. Satu tempat
// aturan "kosong = NULL" agar tiap kolom opsional diperlakukan sama.
func optTrim(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}

// optInt32 mengurai angka opsional: kosong → (nil, ""); terisi & sah & ≥0 →
// (&v, ""); tak terurai/negatif → (nil, "number"). Non-negatif dipaksa di sini
// karena kolomnya berarti hitungan (penduduk, dusun).
func optInt32(s string) (*int32, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, ""
	}
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil || n < 0 {
		return nil, "number"
	}
	v := int32(n)
	return &v, ""
}
