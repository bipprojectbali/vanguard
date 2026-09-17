package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// accounts_import_confirm_test.go — BL-63: bukti all-or-nothing di DB nyata
// (butuh TEST_DATABASE_URL, auto-skip via setupAccounts→setupTest bila kosong).
// Resolusi/parse murni ada di accounts_import_parse_test.go; gerbang F2 di
// accounts_import_gate_test.go.

func confirmReq(rawCSV string) url.Values {
	return url.Values{"raw_csv": {rawCSV}}
}

// TestAccountImportConfirm_AllValid: file semua baris valid → SEMUA masuk DB,
// tiap baris dapat entity_code unik, audit tercatat.
func TestAccountImportConfirm_AllValid(t *testing.T) {
	env, uid := setupAccounts(t)
	vs := villages(t, env, 2)

	csvText := "kode_desa,tipe_akun\n" +
		vs[0].Code + ",prospect\n" +
		vs[1].Code + ",customer\n"

	req := accountsReq(http.MethodPost, "/w/test/accounts/import/confirm", confirmReq(csvText), "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountImportConfirm)

	loc := rec.Header().Get("Location")
	if rec.Code != http.StatusSeeOther || !strings.Contains(loc, "ok=imported") {
		t.Fatalf("harus redirect ok=imported (303), got %d %q\n%s", rec.Code, loc, rec.Body.String())
	}

	rows := env.allAccounts(t)
	if len(rows) != 2 {
		t.Fatalf("harus tersimpan 2 desa, ada %d", len(rows))
	}
	codes := map[string]bool{}
	for _, a := range rows {
		if a.EntityCode == nil || *a.EntityCode == "" {
			t.Errorf("entity_code kosong utk %+v", a)
		}
		codes[*a.EntityCode] = true
		if a.AccountOwner == nil || *a.AccountOwner != uid {
			t.Errorf("owner default harus aktor (uid=%d), got %v", uid, a.AccountOwner)
		}
	}
	if len(codes) != 2 {
		t.Errorf("entity_code harus unik per baris, got %v", codes)
	}
	env.assertAudited(t, "account.import")
}

// TestAccountImportConfirm_OneInvalid_NoRowsWritten: 1 baris valid + 1 baris
// tak valid (kode desa tak ketemu) → confirm menolak SELURUHNYA, nol baris
// masuk DB — bukti kebijakan all-or-nothing (Rule TOCTOU re-validate).
func TestAccountImportConfirm_OneInvalid_NoRowsWritten(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)

	csvText := "kode_desa,tipe_akun\n" +
		v.Code + ",prospect\n" +
		"99.99.99.9999,customer\n" // kode tak ada di master regions

	req := accountsReq(http.MethodPost, "/w/test/accounts/import/confirm", confirmReq(csvText), "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountImportConfirm)

	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "err=import_row_invalid") {
		t.Errorf("harus err=import_row_invalid, got %q (status %d)", loc, rec.Code)
	}
	rows := env.allAccounts(t)
	if len(rows) != 0 {
		t.Fatalf("all-or-nothing: satu baris invalid harus batalkan SEMUA, ada %d baris tertulis", len(rows))
	}
}

// TestAccountImportConfirm_OwnerByEmail: kolom email_pemilik yang cocok
// anggota tenant → owner baris = user tsb, bukan aktor pengimpor.
func TestAccountImportConfirm_OwnerByEmail(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)
	owner := env.seedMember(t, "pemilik@local", "member", 0)

	csvText := "kode_desa,tipe_akun,email_pemilik\n" +
		v.Code + ",prospect," + owner.Email + "\n"

	req := accountsReq(http.MethodPost, "/w/test/accounts/import/confirm", confirmReq(csvText), "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountImportConfirm)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("harus sukses (303), got %d\n%s", rec.Code, rec.Body.String())
	}

	rows := env.allAccounts(t)
	if len(rows) != 1 {
		t.Fatalf("harus tersimpan 1 desa, ada %d", len(rows))
	}
	if rows[0].AccountOwner == nil || *rows[0].AccountOwner != owner.ID {
		t.Errorf("owner harus %d (dari email_pemilik), got %v", owner.ID, rows[0].AccountOwner)
	}
}

// TestAccountImportConfirm_OwnerEmailNotFound_NoRowsWritten: email_pemilik
// yang TAK cocok anggota tenant mana pun → baris gagal → all-or-nothing
// menolak seluruh file, nol baris tertulis.
func TestAccountImportConfirm_OwnerEmailNotFound_NoRowsWritten(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)

	csvText := "kode_desa,tipe_akun,email_pemilik\n" +
		v.Code + ",prospect,tak-ada@local\n"

	req := accountsReq(http.MethodPost, "/w/test/accounts/import/confirm", confirmReq(csvText), "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountImportConfirm)

	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "err=import_row_invalid") {
		t.Errorf("harus err=import_row_invalid, got %q (status %d)", loc, rec.Code)
	}
	if len(env.allAccounts(t)) != 0 {
		t.Error("email owner tak ketemu harus batalkan SELURUH file, tak ada baris tertulis")
	}
}

// TestAccountImportConfirm_DuplicateVsExisting_NoRowsWritten: village_code yang
// sudah dipakai akun HIDUP lain di tenant yang sama → gagal village_code_dup,
// all-or-nothing menolak seluruh file.
func TestAccountImportConfirm_DuplicateVsExisting_NoRowsWritten(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)

	code, err := env.q.GenerateEntityCode(t.Context(), env.tenantID, "account")
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	vcode := v.Code
	if _, err := env.q.CreateAccount(t.Context(), db.CreateAccountParams{
		TenantID:    env.tenantID,
		EntityCode:  &code,
		VillageName: v.Name,
		VillageCode: &vcode,
		AccountType: "prospect",
	}); err != nil {
		t.Fatalf("seed existing account: %v", err)
	}

	csvText := "kode_desa,tipe_akun\n" + v.Code + ",customer\n"
	req := accountsReq(http.MethodPost, "/w/test/accounts/import/confirm", confirmReq(csvText), "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountImportConfirm)

	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "err=import_row_invalid") {
		t.Errorf("harus err=import_row_invalid, got %q (status %d)", loc, rec.Code)
	}
	rows := env.allAccounts(t)
	if len(rows) != 1 {
		t.Errorf("hanya akun seed yg boleh ada, ada %d baris", len(rows))
	}
}

// TestAccountImportConfirm_EmptyRawCSV: raw_csv kosong (mis. form dipalsukan
// langsung ke /confirm tanpa lewat preview) → ditolak import_empty, tak crash.
func TestAccountImportConfirm_EmptyRawCSV(t *testing.T) {
	env, uid := setupAccounts(t)
	req := accountsReq(http.MethodPost, "/w/test/accounts/import/confirm", confirmReq(""), "")
	rec := env.runAccount(uid, "owner", "sales", req, env.h.AccountImportConfirm)

	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "err=import_empty") {
		t.Errorf("harus err=import_empty, got %q (status %d)", loc, rec.Code)
	}
}
