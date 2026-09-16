package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// sales_leads_import_gate_test.go — BL-133: F2 gerbang requireLeadWrite
// berlaku SAMA di keempat rute impor (GET form, GET template, POST preview,
// POST confirm). Mirror accounts_import_gate_test.go (BL-63), TAPI tabel role
// BEDA: csm TIDAK punya crm:leads write (beda dari crm:accounts write yang
// mengizinkan csm) — lihat business_policy.csv. Setup & helper (setupAccounts,
// accountsReq, newMultipartUploadRequest, dst.) di accounts_test.go — dipakai
// bersama utk Account & Lead (satu testEnv/paket).

func TestLeadImport_GateForm(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"sales", true},
		{"csm", false},
		{"support", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			req := accountsReq(http.MethodGet, "/w/test/leads/import", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.LeadImportForm)
			if c.allow && rec.Code != http.StatusOK {
				t.Errorf("role %q harus lolos gate, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow && rec.Code != http.StatusForbidden {
				t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
			}
		})
	}
}

func TestLeadImport_GateTemplate(t *testing.T) {
	env, uid := setupAccounts(t)
	req := accountsReq(http.MethodGet, "/w/test/leads/import/template", nil, "")
	rec := env.runAccount(uid, "owner", "csm", req, env.h.LeadImportTemplate)
	if rec.Code != http.StatusForbidden {
		t.Errorf("csm harus ditolak unduh template Lead (403), got %d", rec.Code)
	}

	req2 := accountsReq(http.MethodGet, "/w/test/leads/import/template", nil, "")
	rec2 := env.runAccount(uid, "owner", "sales", req2, env.h.LeadImportTemplate)
	if rec2.Code != http.StatusOK {
		t.Errorf("sales harus bisa unduh template, got %d", rec2.Code)
	}
	if !strings.Contains(rec2.Body.String(), "kode_kecamatan") {
		t.Error("template CSV harus memuat header kode_kecamatan")
	}
}

func TestLeadImport_GatePreview(t *testing.T) {
	const csvText = "lead_name,kode_kecamatan\nTest Lead,32.01.01\n"
	cases := []struct {
		role  string
		allow bool
	}{
		{"sales", true},
		{"csm", false},
		{"support", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			req := newMultipartUploadRequest(t, "/w/test/leads/import", "csv_file", "import.csv", csvText)
			rec := env.runAccount(uid, "owner", c.role, req, env.h.LeadImportPreview)
			if c.allow && rec.Code != http.StatusOK {
				t.Errorf("role %q harus lolos gate preview, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow && rec.Code != http.StatusForbidden {
				t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
			}
		})
	}
}

func TestLeadImport_GateConfirm(t *testing.T) {
	env, uid := setupAccounts(t)
	form := url.Values{"raw_csv": {"lead_name,kode_kecamatan\nTest Lead,32.01.01\n"}}

	req := accountsReq(http.MethodPost, "/w/test/leads/import/confirm", form, "")
	rec := env.runAccount(uid, "owner", "csm", req, env.h.LeadImportConfirm)
	if rec.Code != http.StatusForbidden {
		t.Errorf("csm harus ditolak confirm Lead (403), got %d", rec.Code)
	}
	if len(env.allLeads(t)) != 0 {
		t.Error("ditolak gate tak boleh menulis apa pun")
	}
}
