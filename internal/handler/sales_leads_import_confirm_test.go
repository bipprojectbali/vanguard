package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// sales_leads_import_confirm_test.go — BL-133: bukti all-or-nothing di DB
// nyata (butuh TEST_DATABASE_URL, auto-skip via setupAccounts→setupTest bila
// kosong). Resolusi/parse murni ada di sales_leads_import_parse_test.go;
// gerbang F2 di sales_leads_import_gate_test.go. Mirror
// accounts_import_confirm_test.go (BL-63), TANPA test owner-by-email/
// duplikat (tak berlaku utk Lead — owner SELALU importer, tak ada constraint
// unik terkait Kecamatan).

func leadConfirmReq(rawCSV string) url.Values {
	return url.Values{"raw_csv": {rawCSV}}
}

// TestLeadImportConfirm_AllValid: file semua baris valid → SEMUA masuk DB,
// tiap baris dapat entity_code unik, owner = importer, audit tercatat.
func TestLeadImportConfirm_AllValid(t *testing.T) {
	env, uid := setupAccounts(t)
	_, code1, _ := firstDistrictCode(t, env)

	csvText := "lead_name,kode_kecamatan\n" +
		"Lead Satu," + code1 + "\n" +
		"Lead Dua," + code1 + "\n"

	req := accountsReq(http.MethodPost, "/w/test/leads/import/confirm", leadConfirmReq(csvText), "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadImportConfirm)

	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther || !strings.Contains(loc, "ok=imported") {
		t.Fatalf("harus redirect ok=imported (303), got %d %q\n%s", rec.Code, loc, rec.Body.String())
	}

	rows := env.allLeads(t)
	if len(rows) != 2 {
		t.Fatalf("harus tersimpan 2 lead, ada %d", len(rows))
	}
	codes := map[string]bool{}
	for _, l := range rows {
		if l.EntityCode == nil || *l.EntityCode == "" {
			t.Errorf("entity_code kosong utk %+v", l)
		}
		codes[*l.EntityCode] = true
		if l.LeadOwner == nil || *l.LeadOwner != uid {
			t.Errorf("owner harus SELALU importer (uid=%d), got %v", uid, l.LeadOwner)
		}
	}
	if len(codes) != 2 {
		t.Errorf("entity_code harus unik per baris, got %v", codes)
	}
	env.assertAudited(t, "lead.import")
}

// TestLeadImportConfirm_OneInvalid_NoRowsWritten: 1 baris valid + 1 baris kode
// kecamatan tak dikenal → confirm menolak SELURUHNYA, nol baris masuk DB —
// bukti kebijakan all-or-nothing.
func TestLeadImportConfirm_OneInvalid_NoRowsWritten(t *testing.T) {
	env, uid := setupAccounts(t)
	_, code1, _ := firstDistrictCode(t, env)

	csvText := "lead_name,kode_kecamatan\n" +
		"Lead Satu," + code1 + "\n" +
		"Lead Dua,99.99.99\n" // kode tak ada di master regions

	req := accountsReq(http.MethodPost, "/w/test/leads/import/confirm", leadConfirmReq(csvText), "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadImportConfirm)

	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "err=import_row_invalid") {
		t.Errorf("harus err=import_row_invalid, got %q (status %d)", loc, rec.Code)
	}
	if len(env.allLeads(t)) != 0 {
		t.Fatalf("all-or-nothing: satu baris invalid harus batalkan SEMUA, ada %d baris tertulis", len(env.allLeads(t)))
	}
}

// TestLeadImportConfirm_EmptyRawCSV: raw_csv kosong (mis. form dipalsukan
// langsung ke /confirm tanpa lewat preview) → ditolak import_empty, tak crash.
func TestLeadImportConfirm_EmptyRawCSV(t *testing.T) {
	env, uid := setupAccounts(t)
	req := accountsReq(http.MethodPost, "/w/test/leads/import/confirm", leadConfirmReq(""), "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.LeadImportConfirm)

	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "err=import_empty") {
		t.Errorf("harus err=import_empty, got %q (status %d)", loc, rec.Code)
	}
}

// TestLeadImportConfirm_FLSSkip_PhoneWrittenRegardlessOfRole (keputusan FLS
// BL-133, lihat sales_leads_import_resolve.go & sales_leads_import_confirm.go):
// Manager DIKUNCI dari HP/WhatsApp di form manual sunting
// (TestLeadEdit_PhoneFieldsLocked, sales_leads_fls_test.go) — tapi jalur impor
// CSV TIDAK PERNAH memanggil canEditPhone sama sekali, jadi Manager yang
// mengimpor CSV berisi mobile_phone tetap menulis nomor itu UTUH. Membuktikan
// skip itu benar-benar tercapai (bukan cuma diklaim di komentar). CSV hanya
// punya SATU kolom nomor ("mobile_phone", mengikuti form manual yang
// menggabung HP & WhatsApp) — whatsapp harus tetap NULL, SAMA seperti
// LeadCreate manual (sales_leads.go), bukan celah.
func TestLeadImportConfirm_FLSSkip_PhoneWrittenRegardlessOfRole(t *testing.T) {
	env, uid := setupAccounts(t)
	_, code1, _ := firstDistrictCode(t, env)
	mobile := "0812-1111-2222"

	csvText := "lead_name,kode_kecamatan,mobile_phone\n" +
		"Lead Kontak," + code1 + "," + mobile + "\n"

	req := accountsReq(http.MethodPost, "/w/test/leads/import/confirm", leadConfirmReq(csvText), "")
	// Role "manager": TERKUNCI dari HP/WA di form manual (LeadEdit) tapi TETAP
	// punya crm:leads write, jadi lolos F2 gate impor ini.
	rec := env.runAccount(uid, "owner", "manager", req, env.h.LeadImportConfirm)

	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther || !strings.Contains(loc, "ok=imported") {
		t.Fatalf("harus redirect ok=imported (303), got %d %q\n%s", rec.Code, loc, rec.Body.String())
	}

	rows := env.allLeads(t)
	if len(rows) != 1 {
		t.Fatalf("harus tersimpan 1 lead, ada %d", len(rows))
	}
	l := rows[0]
	if l.MobilePhone == nil || *l.MobilePhone != mobile {
		t.Errorf("FLS-skip GAGAL: mobile_phone harus tertulis apa adanya dari CSV meski role=manager, got %v", l.MobilePhone)
	}
	if l.Whatsapp != nil {
		t.Errorf("whatsapp harus NULL (CSV tak lagi punya kolom whatsapp terpisah), got %v", *l.Whatsapp)
	}
}
