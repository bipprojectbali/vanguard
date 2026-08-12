package handler

import (
	"net/http"
	"strings"
	"testing"
)

// accounts_fls_test.go — dua sumbu izin di-level baris & field untuk Desa:
//   - F3 (ownership): Sales lihat miliknya, CSM lihat binaan, Support nol baris,
//     Admin lihat semua; di luar cakupan → 404 (bukan 403).
//   - F4 (field-level): nomor HP kontak utuh HANYA Sales; editor non-Sales tak
//     pernah menerima nomor asli & masknya tak boleh menimpa nilai tersimpan.
//
// F2 gate + CRUD + keyset diuji di file lain; helper bersama di accounts_test.go.

// --- F3: ownership ---------------------------------------------------------

// TestAccounts_F3_SalesLihatMiliknya: Sales melihat HANYA desa yang ia miliki
// (account_owner). Desa milik sales lain → tak tampil di daftar & detailnya 404.
func TestAccounts_F3_SalesLihatMiliknya(t *testing.T) {
	env, salesA := setupAccounts(t)
	salesB := env.seedMember(t, "salesb@local", "member", 0).ID

	mine := env.seedAccount(t, "Desa Milik A", &salesA, nil, nil)
	theirs := env.seedAccount(t, "Desa Milik B", &salesB, nil, nil)

	// Daftar sebagai salesA: hanya "Desa Milik A".
	req := accountsReq(http.MethodGet, "/w/test/accounts", nil, "")
	body := env.runAccount(salesA, "member", "sales", req, env.h.AccountsList).Body.String()
	if !strings.Contains(body, "Desa Milik A") {
		t.Error("sales A harus melihat desanya sendiri")
	}
	if strings.Contains(body, "Desa Milik B") {
		t.Error("sales A TAK boleh melihat desa milik sales B (F3 bocor)")
	}

	// Detail desa milik B → 404 (menyangkal keberadaan, bukan 403).
	dReq := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(theirs.ID), nil, itoa(theirs.ID))
	if rec := env.runAccount(salesA, "member", "sales", dReq, env.h.AccountDetail); rec.Code != http.StatusNotFound {
		t.Errorf("detail desa di luar cakupan harus 404, got %d", rec.Code)
	}

	// Detail desa sendiri → 200.
	okReq := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(mine.ID), nil, itoa(mine.ID))
	if rec := env.runAccount(salesA, "member", "sales", okReq, env.h.AccountDetail); rec.Code != http.StatusOK {
		t.Errorf("detail desa sendiri harus 200, got %d", rec.Code)
	}
}

// TestAccounts_F3_CSMLihatBinaan: CSM melihat desa yang di-assign padanya
// (assigned_csm atau backup_csm), bukan yang dimiliki sales lain.
func TestAccounts_F3_CSMLihatBinaan(t *testing.T) {
	env, sales := setupAccounts(t)
	csm := env.seedMember(t, "csm@local", "member", 0).ID

	env.seedAccount(t, "Desa Binaan", &sales, &csm, nil)
	env.seedAccount(t, "Desa Cadangan", &sales, nil, &csm)
	env.seedAccount(t, "Desa Lain", &sales, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts", nil, "")
	body := env.runAccount(csm, "member", "csm", req, env.h.AccountsList).Body.String()
	if !strings.Contains(body, "Desa Binaan") || !strings.Contains(body, "Desa Cadangan") {
		t.Error("CSM harus melihat desa assigned & backup")
	}
	if strings.Contains(body, "Desa Lain") {
		t.Error("CSM tak boleh melihat desa yang bukan binaannya")
	}
}

// TestAccounts_F3_SupportNolBaris: Support lolos gate read tapi ScopeNone →
// daftar KOSONG walau ada desa. Ia menyentuh desa hanya lewat konteks tiket.
func TestAccounts_F3_SupportNolBaris(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAccount(t, "Desa Ada", &uid, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts", nil, "")
	rec := env.runAccount(uid, "owner", "support", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("support lolos gate read, harus 200, got %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "Desa Ada") {
		t.Error("support ScopeNone harus nol baris — desa tak boleh tampil")
	}
}

// TestAccounts_F3_AdminLihatSemua: Admin (business) melihat SEMUA desa di
// workspace, tak peduli siapa ownernya.
func TestAccounts_F3_AdminLihatSemua(t *testing.T) {
	env, sales := setupAccounts(t)
	other := env.seedMember(t, "other@local", "member", 0).ID
	env.seedAccount(t, "Desa P", &sales, nil, nil)
	env.seedAccount(t, "Desa Q", &other, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts", nil, "")
	body := env.runAccount(sales, "owner", "admin", req, env.h.AccountsList).Body.String()
	if !strings.Contains(body, "Desa P") || !strings.Contains(body, "Desa Q") {
		t.Error("admin bisnis harus melihat semua desa di workspace")
	}
}

// --- F4: field masking -----------------------------------------------------

// TestAccounts_F4_PhoneMasking: nomor HP kontak utuh HANYA Sales; role lain
// menerima mask. Nilai asli tak boleh SAMPAI ke browser non-Sales (view-source).
func TestAccounts_F4_PhoneMasking(t *testing.T) {
	env, uid := setupAccounts(t)
	phone := "0812-3456-7890"
	a := env.seedAccountWithPhone(t, "Desa Kontak", uid, phone)

	cases := []struct {
		role string
		full bool
	}{
		{"sales", true},
		{"admin", false},
		{"manager", false},
		{"csm", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID), nil, itoa(a.ID))
			body := env.runAccount(uid, "owner", c.role, req, env.h.AccountDetail).Body.String()
			has := strings.Contains(body, phone)
			if c.full && !has {
				t.Errorf("sales harus melihat nomor utuh")
			}
			if !c.full && has {
				t.Errorf("role %q BOCOR — nomor asli sampai ke browser non-Sales", c.role)
			}
		})
	}
}

// TestAccounts_F4_NonSalesTakBisaTimpaPhone: editor non-Sales mengirim mask
// (field terkunci), tapi handler mempertahankan nomor ASLI — mask tak boleh
// menimpa nilai tersimpan.
func TestAccounts_F4_NonSalesTakBisaTimpaPhone(t *testing.T) {
	env, uid := setupAccounts(t)
	phone := "0812-3456-7890"
	a := env.seedAccountWithPhone(t, "Desa Kontak", uid, phone)

	// Admin menyunting: form mengirim mask (flsHidden) sebagai contact_phone.
	form := accountFormValues("Desa Kontak", "prospect")
	form.Set("contact_phone", flsHidden)
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID), form, itoa(a.ID))
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountUpdate); rec.Code != http.StatusSeeOther {
		t.Fatalf("update gagal: %d\n%s", rec.Code, rec.Body.String())
	}

	got, _ := env.q.GetAccount(t.Context(), a.ID)
	if got.ContactPhone == nil || *got.ContactPhone != phone {
		t.Errorf("nomor asli harus dipertahankan, got %v (mask menimpa = F4 bocor)", got.ContactPhone)
	}
}
