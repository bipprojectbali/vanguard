package handler

import (
	"encoding/csv"
	"io"
	"strings"
)

// accounts_import_parse.go — BL-63: parse CSV mentah (tanpa I/O DB) menjadi
// baris data. Resolusi ke master data (village_code/owner_email) ada di
// accounts_import_resolve.go (dipisah agar tiap file di bawah ambang
// Utility=200). Dipakai IDENTIK oleh AccountImportPreview (dry-run) &
// AccountImportConfirm (re-validasi wajib) — SATU sumber kebenaran, menutup
// celah TOCTOU by construction.

// importColumnMap = header CSV (kunci huruf kecil) → nama field internal yang
// dikenali. Kolom lain di file diabaikan (lenient — operator boleh menambah
// kolom catatan sendiri). Nilai peta = NAMA FIELD yang dibaca parseAccountForm
// (accounts_form_parse.go) — TIDAK ada validasi baru ditulis di sini, hanya
// pemetaan kolom→field yang sudah divalidasi jalur create manual.
var importColumnMap = map[string]string{
	"tipe_akun":       "account_type",
	"alamat":          "village_address",
	"kode_pos":        "postal_code",
	"website":         "website",
	"deskripsi":       "description",
	"status_desa":     "village_status",
	"klasifikasi_idm": "village_classification",
	"jumlah_penduduk": "population",
	"jumlah_dusun":    "hamlets_count",
	"anggaran_apbdes": "village_budget",
	"telepon_kantor":  "office_phone",
	"email_kantor":    "office_email",
	"hp_kontak":       "contact_phone",
}

// importRequiredColumns = header wajib ada agar file dianggap terbaca.
// "kode_desa" & "email_pemilik" TIDAK masuk importColumnMap (ditangani khusus
// di parseImportCSV: bukan field parseAccountForm, tapi identitas baris &
// input resolusi owner).
var importRequiredColumns = []string{"kode_desa", "tipe_akun"}

// importRow = satu baris data CSV APA ADANYA (sebelum resolusi village_code/
// owner_email). rowNum = nomor baris FILE (1-based; header = baris 1) agar
// pesan bisa merujuk baris asli di spreadsheet operator.
type importRow struct {
	rowNum      int
	villageCode string
	ownerEmail  string
	fields      map[string]string
}

// resolvedRow = importRow + hasil resolusi, siap dioper ke CreateAccount.
// errCode != "" → baris ini GAGAL (kode dipetakan wsErrMsg/wsErrMsgCRM/
// wsErrMsgImport); field lain tak berarti bila errCode terisi.
//
// villageName/districtID DITURUNKAN dari regions (sama seperti AccountCreate
// via GetVillageRegion) — disimpan di sini krn sudah didapat SEKALI lewat
// ListVillagesByCodes (resolveImportRows); memanggil ulang GetVillageRegion
// per baris saat insert akan jadi N+1 (Rule 13).
type resolvedRow struct {
	rowNum      int
	villageCode string
	villageName string
	districtID  *int64
	errCode     string
	form        accountForm
	ownerID     *int64
}

// parseImportCSV membaca CSV mentah → baris data. Fungsi murni (tanpa I/O DB)
// — resolusi ke master data ada di resolveImportRows. Galat mengembalikan
// kode PRG siap-redirect; tak ada partial-parse saat file dianggap rusak.
func parseImportCSV(r io.Reader) ([]importRow, string) {
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
	for _, req := range importRequiredColumns {
		if _, ok := colIdx[req]; !ok {
			return nil, "import_invalid"
		}
	}

	var rows []importRow
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

		row := importRow{rowNum: rowNum, fields: make(map[string]string, len(importColumnMap))}
		for col, idx := range colIdx {
			val := ""
			if idx < len(rec) {
				val = strings.TrimSpace(rec[idx])
			}
			switch col {
			case "kode_desa":
				row.villageCode = val
			case "email_pemilik":
				row.ownerEmail = val
			default:
				if field, ok := importColumnMap[col]; ok {
					row.fields[field] = val
				}
			}
		}
		// Baris sepenuhnya kosong (semua kolom dikenal kosong) diabaikan —
		// pemisah visual antar-blok data yang lumrah di spreadsheet, bukan
		// data yang dimaksudkan operator.
		if row.villageCode == "" && row.ownerEmail == "" && allFieldsEmpty(row.fields) {
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

func allFieldsEmpty(m map[string]string) bool {
	for _, v := range m {
		if v != "" {
			return false
		}
	}
	return true
}
