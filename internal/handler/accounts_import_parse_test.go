package handler

import (
	"strconv"
	"strings"
	"testing"
)

// accounts_import_parse_test.go — BL-63: parseImportCSV murni (tanpa I/O DB),
// tak butuh TEST_DATABASE_URL. Resolusi lintas-tabel (village_code tak
// ketemu, duplikat, owner email) ada di accounts_import_resolve_test.go
// (butuh DB nyata via testEnv).

const importValidHeader = "kode_desa,tipe_akun,email_pemilik,alamat,kode_pos,website,deskripsi,status_desa,klasifikasi_idm,jumlah_penduduk,jumlah_dusun,anggaran_apbdes,telepon_kantor,email_kantor,hp_kontak"

func TestParseImportCSV_Valid(t *testing.T) {
	csvText := importValidHeader + "\n" +
		"32.01.01.2001,prospect,,Jl. Merdeka 1,40311,,,,,,,,,,\n" +
		"32.01.01.2002,customer,owner@test.local,,,,,,,,,,,,\n"
	rows, code := parseImportCSV(strings.NewReader(csvText))
	if code != "" {
		t.Fatalf("code = %q, want empty", code)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].villageCode != "32.01.01.2001" || rows[0].fields["account_type"] != "prospect" {
		t.Errorf("row 0 = %+v", rows[0])
	}
	if rows[1].ownerEmail != "owner@test.local" {
		t.Errorf("row 1 ownerEmail = %q, want owner@test.local", rows[1].ownerEmail)
	}
	if rows[0].rowNum != 2 || rows[1].rowNum != 3 {
		t.Errorf("rowNum = %d,%d — want 2,3 (header = baris 1)", rows[0].rowNum, rows[1].rowNum)
	}
}

func TestParseImportCSV_HeaderCaseInsensitiveAndReordered(t *testing.T) {
	csvText := "TIPE_AKUN,Kode_Desa\nprospect,32.01.01.2001\n"
	rows, code := parseImportCSV(strings.NewReader(csvText))
	if code != "" {
		t.Fatalf("code = %q, want empty", code)
	}
	if len(rows) != 1 || rows[0].villageCode != "32.01.01.2001" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestParseImportCSV_MissingRequiredColumn(t *testing.T) {
	csvText := "kode_desa\n32.01.01.2001\n" // tipe_akun hilang
	_, code := parseImportCSV(strings.NewReader(csvText))
	if code != "import_invalid" {
		t.Errorf("code = %q, want import_invalid", code)
	}
}

func TestParseImportCSV_EmptyFile(t *testing.T) {
	_, code := parseImportCSV(strings.NewReader(""))
	if code != "import_invalid" {
		t.Errorf("code = %q, want import_invalid (header tak terbaca)", code)
	}
}

func TestParseImportCSV_HeaderOnlyNoDataRows(t *testing.T) {
	_, code := parseImportCSV(strings.NewReader(importValidHeader + "\n"))
	if code != "import_empty" {
		t.Errorf("code = %q, want import_empty", code)
	}
}

func TestParseImportCSV_BlankRowIgnored(t *testing.T) {
	csvText := "kode_desa,tipe_akun\n32.01.01.2001,prospect\n,\n32.01.01.2002,customer\n"
	rows, code := parseImportCSV(strings.NewReader(csvText))
	if code != "" {
		t.Fatalf("code = %q, want empty", code)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2 (baris kosong diabaikan)", len(rows))
	}
}

func TestParseImportCSV_TooManyRows(t *testing.T) {
	var b strings.Builder
	b.WriteString("kode_desa,tipe_akun\n")
	for i := 0; i < maxImportRows+1; i++ {
		b.WriteString("32.01.01." + strconv.Itoa(1000+i) + ",prospect\n")
	}
	_, code := parseImportCSV(strings.NewReader(b.String()))
	if code != "import_too_many" {
		t.Errorf("code = %q, want import_too_many", code)
	}
}

func TestParseImportCSV_MalformedRow(t *testing.T) {
	// Kutip tak tertutup — encoding/csv menolaknya sebagai galat parse baris.
	csvText := "kode_desa,tipe_akun\n\"32.01.01.2001,prospect\n"
	_, code := parseImportCSV(strings.NewReader(csvText))
	if code != "import_invalid" {
		t.Errorf("code = %q, want import_invalid", code)
	}
}
