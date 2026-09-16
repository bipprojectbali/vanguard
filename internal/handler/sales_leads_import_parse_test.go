package handler

import (
	"strconv"
	"strings"
	"testing"
)

// sales_leads_import_parse_test.go — BL-133: parseLeadImportCSV murni (tanpa
// I/O DB), tak butuh TEST_DATABASE_URL. Resolusi lintas-tabel (kode_kecamatan
// tak ketemu) ada di sales_leads_import_confirm_test.go (butuh DB nyata via
// testEnv). Mirror accounts_import_parse_test.go (BL-63), tanpa test
// email_pemilik (Lead tak punya kolom itu).

const leadImportValidHeader = "lead_name,kode_kecamatan,contact_person,job_title,lead_status,rating,estimated_value,mobile_phone,email"

func TestParseLeadImportCSV_Valid(t *testing.T) {
	csvText := leadImportValidHeader + "\n" +
		"Lead Satu,32.01.01,Budi,Manager,New,Warm,,,\n" +
		"Lead Dua,32.01.02,,,,,,,\n"
	rows, code := parseLeadImportCSV(strings.NewReader(csvText))
	if code != "" {
		t.Fatalf("code = %q, want empty", code)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2", len(rows))
	}
	if rows[0].districtCode != "32.01.01" || rows[0].fields["lead_name"] != "Lead Satu" {
		t.Errorf("row 0 = %+v", rows[0])
	}
	if rows[0].fields["contact_person"] != "Budi" {
		t.Errorf("row 0 contact_person = %q, want Budi", rows[0].fields["contact_person"])
	}
	if rows[0].rowNum != 2 || rows[1].rowNum != 3 {
		t.Errorf("rowNum = %d,%d — want 2,3 (header = baris 1)", rows[0].rowNum, rows[1].rowNum)
	}
}

func TestParseLeadImportCSV_HeaderCaseInsensitiveAndReordered(t *testing.T) {
	csvText := "KODE_KECAMATAN,Lead_Name\n32.01.01,Lead Satu\n"
	rows, code := parseLeadImportCSV(strings.NewReader(csvText))
	if code != "" {
		t.Fatalf("code = %q, want empty", code)
	}
	if len(rows) != 1 || rows[0].districtCode != "32.01.01" || rows[0].fields["lead_name"] != "Lead Satu" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestParseLeadImportCSV_MissingRequiredColumn(t *testing.T) {
	csvText := "lead_name\nLead Satu\n" // kode_kecamatan hilang
	_, code := parseLeadImportCSV(strings.NewReader(csvText))
	if code != "import_invalid" {
		t.Errorf("code = %q, want import_invalid", code)
	}
}

func TestParseLeadImportCSV_EmptyFile(t *testing.T) {
	_, code := parseLeadImportCSV(strings.NewReader(""))
	if code != "import_invalid" {
		t.Errorf("code = %q, want import_invalid (header tak terbaca)", code)
	}
}

func TestParseLeadImportCSV_HeaderOnlyNoDataRows(t *testing.T) {
	_, code := parseLeadImportCSV(strings.NewReader(leadImportValidHeader + "\n"))
	if code != "import_empty" {
		t.Errorf("code = %q, want import_empty", code)
	}
}

func TestParseLeadImportCSV_BlankRowIgnored(t *testing.T) {
	csvText := "lead_name,kode_kecamatan\nLead Satu,32.01.01\n,\nLead Dua,32.01.02\n"
	rows, code := parseLeadImportCSV(strings.NewReader(csvText))
	if code != "" {
		t.Fatalf("code = %q, want empty", code)
	}
	if len(rows) != 2 {
		t.Fatalf("len(rows) = %d, want 2 (baris kosong diabaikan)", len(rows))
	}
}

func TestParseLeadImportCSV_TooManyRows(t *testing.T) {
	var b strings.Builder
	b.WriteString("lead_name,kode_kecamatan\n")
	for i := 0; i < maxLeadImportRows+1; i++ {
		b.WriteString("Lead " + strconv.Itoa(i) + ",32.01." + strconv.Itoa(1+i%99) + "\n")
	}
	_, code := parseLeadImportCSV(strings.NewReader(b.String()))
	if code != "import_too_many" {
		t.Errorf("code = %q, want import_too_many", code)
	}
}

func TestParseLeadImportCSV_MalformedRow(t *testing.T) {
	// Kutip tak tertutup — encoding/csv menolaknya sebagai galat parse baris.
	csvText := "lead_name,kode_kecamatan\n\"Lead Satu,32.01.01\n"
	_, code := parseLeadImportCSV(strings.NewReader(csvText))
	if code != "import_invalid" {
		t.Errorf("code = %q, want import_invalid", code)
	}
}
