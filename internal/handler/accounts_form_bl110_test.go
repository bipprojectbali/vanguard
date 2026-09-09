package handler

import (
	"net/http"
	"testing"
)

// accounts_form_bl110_test.go — regresi BL-110: field Website desa menerima
// domain telanjang (mis. "www.facebook.com") dan menormalkannya ke URL berskema
// di backend, alih-alih menolak (input lama type=url menolak di browser).

// TestNormalizeWebsite: domain telanjang di-prepend https://; nilai berskema
// dibiarkan; kosong → nil.
func TestNormalizeWebsite(t *testing.T) {
	cases := []struct {
		in   string
		want *string // nil = harap NULL
	}{
		{"", nil},
		{"   ", nil},
		{"www.facebook.com", ptr("https://www.facebook.com")},
		{"facebook.com", ptr("https://facebook.com")},
		{"  desa.id  ", ptr("https://desa.id")}, // trim dulu, lalu prepend
		{"http://desa.id", ptr("http://desa.id")},
		{"https://www.facebook.com", ptr("https://www.facebook.com")},
	}
	for _, c := range cases {
		got := normalizeWebsite(c.in)
		switch {
		case c.want == nil && got != nil:
			t.Errorf("normalizeWebsite(%q) = %q, mau nil", c.in, *got)
		case c.want != nil && got == nil:
			t.Errorf("normalizeWebsite(%q) = nil, mau %q", c.in, *c.want)
		case c.want != nil && got != nil && *got != *c.want:
			t.Errorf("normalizeWebsite(%q) = %q, mau %q", c.in, *got, *c.want)
		}
	}
}

// TestAccountCreate_WebsiteBareDomainNormalized: create desa dgn Website domain
// telanjang → tersimpan sbg URL berskema (terwiring lewat parseAccountForm).
func TestAccountCreate_WebsiteBareDomainNormalized(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)

	form := withVillage(accountFormValues("prospect"), v)
	form.Set("website", "www.facebook.com")
	if rec := createAccountForm(t, env, uid, form); rec.Code != http.StatusSeeOther {
		t.Fatalf("create gagal: %d\n%s", rec.Code, rec.Body.String())
	}

	got := accountByVillageCode(t, env, v.Code)
	if got.Website == nil || *got.Website != "https://www.facebook.com" {
		t.Errorf("Website domain telanjang harus tersimpan https://www.facebook.com, got %v", got.Website)
	}
}
