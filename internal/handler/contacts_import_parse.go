package handler

import (
	"encoding/csv"
	"io"
	"strings"
)

// contacts_import_parse.go — BL-134: parse CSV mentah (tanpa I/O DB) menjadi
// baris data kontak. Pola SAMA dgn accounts_import_parse.go (BL-63) — dipisah
// dari resolusi (contacts_import_resolve.go) agar tiap file di bawah ambang
// Utility=200. Dipakai IDENTIK oleh ContactImportPreview (dry-run) &
// ContactImportConfirm (re-validasi wajib) — SATU sumber kebenaran, menutup
// celah TOCTOU by construction. allFieldsEmpty (accounts_import_parse.go)
// dipakai ulang APA ADANYA — generik, tak spesifik akun.

// contactImportColumnMap = header CSV → nama field yang dibaca parseContactForm
// (contacts_form.go). Beda dgn importColumnMap (BL-63, Indonesia→Inggris),
// header kolom kontak SUDAH sama persis nama field-nya — peta ini tetap
// eksplisit (identity map) agar kolom tak dikenal di file operator diabaikan
// dgn jelas (lenient), bukan diam-diam ikut jadi field lewat refleksi nama.
//
// Diperkecil ke 6 kolom (keputusan 15 Sep, lihat contacts_import.go) — kolom
// lain yang sebelumnya didukung TETAP diisi lewat form manual, bukan dihapus
// dari parseContactForm/skema; hanya tak lagi ditawarkan di jalur impor massal.
var contactImportColumnMap = map[string]string{
	"first_name":         "first_name",
	"last_name":          "last_name",
	"job_title":          "job_title",
	"mobile_phone":       "mobile_phone",
	"whatsapp_number":    "whatsapp_number",
	"is_primary_contact": "is_primary_contact",
}

// contactImportRequiredColumns = header wajib ada agar file dianggap terbaca.
// "kode_desa" TIDAK masuk contactImportColumnMap (ditangani khusus di
// parseContactImportCSV: identitas baris & input resolusi desa induk, bukan
// field parseContactForm).
var contactImportRequiredColumns = []string{"kode_desa", "first_name"}

// contactImportRow = satu baris data CSV APA ADANYA (sebelum resolusi
// village_code → account_id). rowNum = nomor baris FILE (1-based; header =
// baris 1).
type contactImportRow struct {
	rowNum      int
	villageCode string
	fields      map[string]string
}

// contactResolvedRow = contactImportRow + hasil resolusi, siap dioper ke
// insertContact. errCode != "" → baris ini GAGAL (kode dipetakan wsErrMsg/
// wsErrMsgCRM/wsErrMsgImport); field lain tak berarti bila errCode terisi.
type contactResolvedRow struct {
	rowNum      int
	villageCode string
	villageName string
	accountID   int64
	errCode     string
	form        contactForm
}

// parseContactImportCSV membaca CSV mentah → baris data. Fungsi murni (tanpa
// I/O DB) — resolusi ke master data ada di resolveContactImportRows. Galat
// mengembalikan kode PRG siap-redirect; tak ada partial-parse saat file
// dianggap rusak.
func parseContactImportCSV(r io.Reader) ([]contactImportRow, string) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err != nil {
		return nil, "import_invalid"
	}
	colIdx := make(map[string]int, len(header))
	for i, hd := range header {
		colIdx[strings.ToLower(strings.TrimSpace(hd))] = i
	}
	for _, req := range contactImportRequiredColumns {
		if _, ok := colIdx[req]; !ok {
			return nil, "import_invalid"
		}
	}

	var rows []contactImportRow
	rowNum := 1
	for {
		rec, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "import_invalid"
		}
		rowNum++

		row := contactImportRow{rowNum: rowNum, fields: make(map[string]string, len(contactImportColumnMap))}
		for col, idx := range colIdx {
			val := ""
			if idx < len(rec) {
				val = strings.TrimSpace(rec[idx])
			}
			switch col {
			case "kode_desa":
				row.villageCode = val
			default:
				if field, ok := contactImportColumnMap[col]; ok {
					row.fields[field] = val
				}
			}
		}
		// Baris sepenuhnya kosong diabaikan — pemisah visual antar-blok data
		// yang lumrah di spreadsheet, bukan data yang dimaksudkan operator.
		if row.villageCode == "" && allFieldsEmpty(row.fields) {
			continue
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return nil, "import_empty"
	}
	if len(rows) > maxImportRows {
		return nil, "import_too_many"
	}
	return rows, ""
}

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
