package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// accounts_import_gate_test.go — BL-63: F2 gerbang requireAccountWrite berlaku
// SAMA di keempat rute impor (GET form, GET template, POST preview, POST
// confirm) — support (read-only) ditolak semuanya, sales/manager/csm lolos.
// Setup & helper (setupAccounts, newMultipartUploadRequest, dst.) di
// accounts_test.go.

func TestAccountImport_GateForm(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"sales", true},
		{"csm", true},
		{"support", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			req := accountsReq(http.MethodGet, "/w/test/accounts/import", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.AccountImportForm)
			if c.allow && rec.Code != http.StatusOK {
				t.Errorf("role %q harus lolos gate, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow && rec.Code != http.StatusForbidden {
				t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
			}
		})
	}
}

func TestAccountImport_GateTemplate(t *testing.T) {
	env, uid := setupAccounts(t)
	req := accountsReq(http.MethodGet, "/w/test/accounts/import/template", nil, "")
	rec := env.runAccount(uid, "owner", "support", req, env.h.AccountImportTemplate)
	if rec.Code != http.StatusForbidden {
		t.Errorf("support harus ditolak unduh template (403), got %d", rec.Code)
	}

	req2 := accountsReq(http.MethodGet, "/w/test/accounts/import/template", nil, "")
	rec2 := env.runAccount(uid, "owner", "sales", req2, env.h.AccountImportTemplate)
	if rec2.Code != http.StatusOK {
		t.Errorf("sales harus bisa unduh template, got %d", rec2.Code)
	}
	if !strings.Contains(rec2.Body.String(), "kode_desa") {
		t.Error("template CSV harus memuat header kode_desa")
	}
}

func TestAccountImport_GatePreview(t *testing.T) {
	const csvText = "kode_desa,tipe_akun\n32.01.01.2001,prospect\n"
	cases := []struct {
		role  string
		allow bool
	}{
		{"sales", true},
		{"support", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			req := newMultipartUploadRequest(t, "/w/test/accounts/import", "csv_file", "import.csv", csvText)
			rec := env.runAccount(uid, "owner", c.role, req, env.h.AccountImportPreview)
			if c.allow && rec.Code != http.StatusOK {
				t.Errorf("role %q harus lolos gate preview, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow && rec.Code != http.StatusForbidden {
				t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
			}
		})
	}
}

func TestAccountImport_GateConfirm(t *testing.T) {
	env, uid := setupAccounts(t)
	form := url.Values{"raw_csv": {"kode_desa,tipe_akun\n32.01.01.2001,prospect\n"}}

	req := accountsReq(http.MethodPost, "/w/test/accounts/import/confirm", form, "")
	rec := env.runAccount(uid, "owner", "support", req, env.h.AccountImportConfirm)
	if rec.Code != http.StatusForbidden {
		t.Errorf("support harus ditolak confirm (403), got %d", rec.Code)
	}
	if len(env.allAccounts(t)) != 0 {
		t.Error("ditolak gate tak boleh menulis apa pun")
	}
}
