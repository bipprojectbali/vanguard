package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// contacts_import_test.go — BL-134: setup/helper bersama utk test impor
// Kontak (CSV) + gerbang F2 (requireContactWrite) di KEEMPAT rute (GET form,
// GET template, POST preview, POST confirm), pola SAMA accounts_import_gate_test.go
// (BL-63). Skenario DB (all-or-nothing, F3 per-baris, primary, FLS-skip) ada
// di contacts_import_confirm_test.go.

// contactImportHeaderLine = header CSV mentah persis contactImportCSVHeader
// (contacts_import.go) — disalin literal di sini (bukan strings.Join dari
// var package) agar test tetap mendeteksi bila urutan kolom produksi berubah
// tanpa sengaja.
const contactImportHeaderLine = "kode_desa,first_name,last_name,job_title," +
	"mobile_phone,whatsapp_number,is_primary_contact"

// contactImportRowCSV merakit satu baris CSV impor kontak sesuai URUTAN
// contactImportHeaderLine dari peta kolom→nilai — agar tiap test kasus hanya
// menyebut kolom yang relevan, bukan mengetik 20 kolom kosong manual.
// kode_desa & first_name selalu disetel dari parameter (dua kolom wajib).
func contactImportRowCSV(villageCode, firstName string, extra map[string]string) string {
	fields := map[string]string{"kode_desa": villageCode, "first_name": firstName}
	for k, v := range extra {
		fields[k] = v
	}
	cols := strings.Split(contactImportHeaderLine, ",")
	vals := make([]string, len(cols))
	for i, c := range cols {
		vals[i] = fields[c]
	}
	return strings.Join(vals, ",")
}

// seedAccountForImport menaruh satu desa langsung lewat pool DENGAN
// village_code TERISI dari master regions (v) — beda dari seedAccountFull
// (accounts_test.go) yang tak pernah mengisi village_code. Impor kontak
// me-resolve akun lewat kolom ini (ListAccountsByVillageCodesForContactImport),
// jadi test butuh varian seed sendiri.
func (e *testEnv) seedAccountForImport(t *testing.T, v villageRow, owner *int64) db.Account {
	t.Helper()
	code, err := e.q.GenerateEntityCode(t.Context(), e.tenantID, codes.EntityAccount)
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	a, err := e.q.CreateAccount(t.Context(), db.CreateAccountParams{
		TenantID:     e.tenantID,
		EntityCode:   &code,
		VillageName:  v.Name,
		VillageCode:  &v.Code,
		AccountType:  "prospect",
		AccountOwner: owner,
	})
	if err != nil {
		t.Fatalf("seed account for import %s: %v", v.Code, err)
	}
	return a
}

// --- F2: gerbang requireContactWrite di keempat rute ------------------------

func TestContactImport_GateForm(t *testing.T) {
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
			req := contactsReq(http.MethodGet, "/w/test/contacts/import", nil, "", "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.ContactImportForm)
			if c.allow && rec.Code != http.StatusOK {
				t.Errorf("role %q harus lolos gate, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow && rec.Code != http.StatusForbidden {
				t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
			}
		})
	}
}

func TestContactImport_GateTemplate(t *testing.T) {
	env, uid := setupAccounts(t)
	req := contactsReq(http.MethodGet, "/w/test/contacts/import/template", nil, "", "")
	rec := env.runAccount(uid, "owner", "support", req, env.h.ContactImportTemplate)
	if rec.Code != http.StatusForbidden {
		t.Errorf("support harus ditolak unduh template (403), got %d", rec.Code)
	}

	req2 := contactsReq(http.MethodGet, "/w/test/contacts/import/template", nil, "", "")
	rec2 := env.runAccount(uid, "owner", "sales", req2, env.h.ContactImportTemplate)
	if rec2.Code != http.StatusOK {
		t.Errorf("sales harus bisa unduh template, got %d", rec2.Code)
	}
	if !strings.Contains(rec2.Body.String(), "kode_desa") {
		t.Error("template CSV harus memuat header kode_desa")
	}
}

func TestContactImport_GatePreview(t *testing.T) {
	csvText := contactImportHeaderLine + "\n" + contactImportRowCSV("32.01.01.2001", "Budi", nil) + "\n"
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
			req := newMultipartUploadRequest(t, "/w/test/contacts/import", "csv_file", "import.csv", csvText)
			rec := env.runAccount(uid, "owner", c.role, req, env.h.ContactImportPreview)
			if c.allow && rec.Code != http.StatusOK {
				t.Errorf("role %q harus lolos gate preview, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow && rec.Code != http.StatusForbidden {
				t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
			}
		})
	}
}

func TestContactImport_GateConfirm(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)
	a := env.seedAccountForImport(t, v, &uid)
	rawCSV := contactImportHeaderLine + "\n" + contactImportRowCSV(v.Code, "Budi", nil) + "\n"
	form := url.Values{"raw_csv": {rawCSV}}

	req := contactsReq(http.MethodPost, "/w/test/contacts/import/confirm", form, "", "")
	rec := env.runAccount(uid, "owner", "support", req, env.h.ContactImportConfirm)
	if rec.Code != http.StatusForbidden {
		t.Errorf("support harus ditolak confirm (403), got %d", rec.Code)
	}
	if len(env.liveContacts(t, a.ID)) != 0 {
		t.Error("ditolak gate tak boleh menulis apa pun")
	}
}
