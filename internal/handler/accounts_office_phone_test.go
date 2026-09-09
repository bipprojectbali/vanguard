package handler

import (
	"net/http"
	"strings"
	"testing"
)

// accounts_office_phone_test.go — Telepon Kantor (office_phone) kini divalidasi
// sebagai NOMOR (optPhone), sama seperti HP Kontak (BL-106): nilai sah tersimpan
// apa adanya, nilai non-angka ditolak spesifik (?err=office_phone), bukan
// tersimpan sebagai teks bebas.

// TestAccountCreate_OfficePhoneStored: create desa dgn office_phone valid →
// tersimpan apa adanya (format asli dipertahankan optPhone).
func TestAccountCreate_OfficePhoneStored(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)

	const office = "021-555 1234"
	form := withVillage(accountFormValues("prospect"), v)
	form.Set("office_phone", office)
	if rec := createAccountForm(t, env, uid, form); rec.Code != http.StatusSeeOther {
		t.Fatalf("create gagal: %d\n%s", rec.Code, rec.Body.String())
	}

	got := accountByVillageCode(t, env, v.Code)
	if got.OfficePhone == nil || *got.OfficePhone != office {
		t.Errorf("office_phone harus tersimpan %q, got %v", office, got.OfficePhone)
	}
}

// TestAccountCreate_OfficePhoneNonNumericRejected: office_phone berisi huruf →
// create ditolak dgn kode spesifik office_phone (bukan tersimpan mentah).
func TestAccountCreate_OfficePhoneNonNumericRejected(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)

	form := withVillage(accountFormValues("prospect"), v)
	form.Set("office_phone", "bukan-nomor")
	rec := createAccountForm(t, env, uid, form)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("PRG harus 303 (redirect + ?err), got %d\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=office_phone") {
		t.Errorf("redirect harus memuat ?err=office_phone, got %q", loc)
	}
}
