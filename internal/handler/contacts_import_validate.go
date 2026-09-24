package handler

import "strings"

// contacts_import_validate.go — normalizeContactPrimaryFlag (validasi kolom
// is_primary_contact CSV) & failAllContactRows (tandai semua baris gagal saat
// query batch resolusi error). Dipisah dari contacts_import_parse.go agar
// file itu di bawah ambang tipe Route/Handler (150).

// normalizeContactPrimaryFlag memvalidasi & menormalkan nilai mentah kolom
// is_primary_contact dari CSV. SENGAJA lebih ketat drpd optBool form manual
// (checkbox HTML: hadir apa pun isinya = true) — di CSV teks bebas, nilai
// seperti "FALSE"/"tidak"/"0" gampang disalahartikan operator sbg false
// padahal optBool akan membacanya true krn sel tak kosong (keputusan 15 Sep,
// respons atas risiko itu). Jalur impor HANYA menerima tiga bentuk
// (case-insensitive, spasi diabaikan): "true" → true, "false"/kosong →
// false; nilai lain = baris invalid ("contact_primary_invalid"), bukan diam-
// diam ditafsir true.
func normalizeContactPrimaryFlag(raw string) (normalized string, ok bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "false":
		return "", true
	case "true":
		return "true", true
	default:
		return "", false
	}
}

// failAllContactRows menandai SELURUH baris gagal dgn kode yang sama — dipakai
// saat query batch resolusi sendiri gagal (galat DB, bukan galat data baris).
func failAllContactRows(rows []contactImportRow, code string) []contactResolvedRow {
	out := make([]contactResolvedRow, len(rows))
	for i, row := range rows {
		out[i] = contactResolvedRow{rowNum: row.rowNum, villageCode: row.villageCode, errCode: code}
	}
	return out
}
