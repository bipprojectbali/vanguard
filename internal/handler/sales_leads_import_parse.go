package handler

import (
	"encoding/csv"
	"io"
	"strings"
)

// sales_leads_import_parse.go — BL-133: parse CSV mentah (tanpa I/O DB)
// menjadi baris data. Resolusi ke master data (kode_kecamatan → district_id)
// ada di sales_leads_import_resolve.go (dipisah agar tiap file di bawah
// ambang Utility=200 — pola sama BL-63, accounts_import_parse.go). Dipakai
// IDENTIK oleh LeadImportPreview (dry-run) & LeadImportConfirm (re-validasi
// wajib) — SATU sumber kebenaran, menutup celah TOCTOU by construction.

// leadImportColumnMap = header CSV (kunci huruf kecil) → nama field internal
// yang dikenali. Nilai peta = NAMA FIELD yang dibaca parseLeadForm
// (sales_leads_form.go) — kebetulan identik nama header CSV-nya sendiri utk
// Lead (beda dgn accounts_import_parse.go yang punya beberapa nama header ≠
// nama field), tapi map eksplisit tetap dipertahankan (bukan dipetakan
// langsung ke fields[col]) agar kolom lain yang mungkin ditambah operator di
// filenya sendiri (kolom catatan bebas) tetap diabaikan, bukan ikut lolos ke
// parseLeadForm dgn nama yang salah sangka field valid.
var leadImportColumnMap = map[string]string{
	"lead_name":       "lead_name",
	"contact_person":  "contact_person",
	"job_title":       "job_title",
	"lead_status":     "lead_status",
	"rating":          "rating",
	"estimated_value": "estimated_value",
	"mobile_phone":    "mobile_phone",
	"email":           "email",
}

// leadImportRequiredColumns = header wajib ada agar file dianggap terbaca.
// "kode_kecamatan" TIDAK masuk leadImportColumnMap (ditangani khusus di
// parseLeadImportCSV: bukan field parseLeadForm, tapi identitas baris yang
// perlu diresolusi ke district_id lebih dulu).
var leadImportRequiredColumns = []string{"lead_name", "kode_kecamatan"}

// leadImportRow = satu baris data CSV APA ADANYA (sebelum resolusi
// kode_kecamatan → district_id). rowNum = nomor baris FILE (1-based; header =
// baris 1) agar pesan bisa merujuk baris asli di spreadsheet operator.
type leadImportRow struct {
	rowNum       int
	districtCode string
	fields       map[string]string
}

// resolvedLeadRow = leadImportRow + hasil resolusi, siap dioper ke CreateLead.
// errCode != "" → baris ini GAGAL (kode dipetakan wsErrMsg); field lain tak
// berarti bila errCode terisi.
type resolvedLeadRow struct {
	rowNum       int
	districtCode string
	districtName string
	errCode      string
	form         leadForm
}

// parseLeadImportCSV membaca CSV mentah → baris data. Fungsi murni (tanpa I/O
// DB) — resolusi ke master data ada di resolveLeadImportRows. Galat
// mengembalikan kode PRG siap-redirect; tak ada partial-parse saat file
// dianggap rusak.
func parseLeadImportCSV(r io.Reader) ([]leadImportRow, string) {
	cr := csv.NewReader(r)
	cr.TrimLeadingSpace = true

	header, err := cr.Read()
	if err != nil {
		return nil, "import_invalid"
	}
	colIdx := make(map[string]int, len(header))
	for i, h := range header {
		colIdx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, req := range leadImportRequiredColumns {
		if _, ok := colIdx[req]; !ok {
			return nil, "import_invalid"
		}
	}

	var rows []leadImportRow
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

		row := leadImportRow{rowNum: rowNum, fields: make(map[string]string, len(leadImportColumnMap))}
		for col, idx := range colIdx {
			val := ""
			if idx < len(rec) {
				val = strings.TrimSpace(rec[idx])
			}
			switch col {
			case "kode_kecamatan":
				row.districtCode = val
			default:
				if field, ok := leadImportColumnMap[col]; ok {
					row.fields[field] = val
				}
			}
		}
		// Baris sepenuhnya kosong (semua kolom dikenal kosong) diabaikan —
		// pemisah visual antar-blok data yang lumrah di spreadsheet, bukan
		// data yang dimaksudkan operator. allFieldsEmpty didefinisikan di
		// accounts_import_parse.go (paket sama, dipakai ulang apa adanya).
		if row.districtCode == "" && allFieldsEmpty(row.fields) {
			continue
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		return nil, "import_empty"
	}
	if len(rows) > maxLeadImportRows {
		return nil, "import_too_many"
	}
	return rows, ""
}
