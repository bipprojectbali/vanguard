package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// accounts_fls_test.go — sumbu izin di-level baris & field untuk Desa:
//   - F3 (ownership): Sales lihat miliknya, CSM lihat binaan, Support nol baris,
//     Admin lihat semua; di luar cakupan → 404 (bukan 403).
//   - F4 (field-level): anggaran desa (VillageBudget) & ARR langganan tersamar
//     untuk Support. BL-106: HP Kontak account TAK lagi ber-FLS — semua role yang
//     boleh melihat desa melihat & menyunting nomor penuh (mask HP kini khusus
//     modul Kontak/Lead, diuji di contacts_fls_test.go & sales_leads_fls_test.go).
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

// TestAccounts_BL106_PhoneNoFLS: BL-106 — HP Kontak account tak lagi ber-FLS.
// SEMUA business_role yang boleh melihat desa (di sini yang punya scope: sales,
// admin, manager, csm) melihat nomor PENUH di halaman detail; tak ada lagi yang
// menerima mask. Kebalikan dari perilaku lama (dulu manager/csm ter-mask).
func TestAccounts_BL106_PhoneNoFLS(t *testing.T) {
	env, uid := setupAccounts(t)
	phone := "0812-3456-7890"
	a := env.seedAccountWithPhone(t, "Desa Kontak", uid, phone)

	for _, role := range []string{"sales", "admin", "manager", "csm"} {
		t.Run("role="+role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID), nil, itoa(a.ID))
			body := env.runAccount(uid, "owner", role, req, env.h.AccountDetail).Body.String()
			if !strings.Contains(body, phone) {
				t.Errorf("role %q harus melihat nomor penuh (HP Kontak account tak lagi ber-FLS)", role)
			}
			if strings.Contains(body, flsHidden) {
				t.Errorf("role %q: tak boleh ada mask HP di detail account (BL-106)", role)
			}
		})
	}
}

// TestAccountDetailView_BL106_PhoneUnmaskedSupport: bahkan Support (bila jalur
// memanggil accountDetailView) menerima nomor PENUH — bukan lagi mask. Diuji
// LANGSUNG atas accountDetailView karena Support ScopeNone selalu 404 di jalur
// HTTP (TestAccounts_F3_SupportNolBaris); ini mengunci bahwa masking phone sudah
// benar-benar dilepas di builder view, bukan sekadar tak terlihat via HTTP.
func TestAccountDetailView_BL106_PhoneUnmaskedSupport(t *testing.T) {
	env, uid := setupAccounts(t)
	phone := "0812-3456-7890"
	a := env.seedAccountWithPhone(t, "Desa Kontak Support", uid, phone)

	var got string
	req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID), nil, itoa(a.ID))
	env.runAccount(uid, "owner", "support", req, func(w http.ResponseWriter, r *http.Request) {
		got = env.h.accountDetailView(r.Context(), "", a).ContactPhone
	})
	if got != phone {
		t.Errorf("BL-106: ContactPhone harus nomor penuh %q, got %q", phone, got)
	}
}

// TestAccounts_BL106_NonSalesBisaSuntingPhone: editor non-Sales (admin) kini
// BISA mengubah nomor HP — tak ada lagi field terkunci maupun preservation guard.
// Nilai yang dikirim form tersimpan apa adanya.
func TestAccounts_BL106_NonSalesBisaSuntingPhone(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccountWithPhone(t, "Desa Kontak", uid, "0812-3456-7890")

	const newPhone = "0899-0000-1111"
	// Tanpa village_id, nama Desa dipertahankan dari seed (BL-66).
	form := accountFormValues("prospect")
	form.Set("contact_phone", newPhone)
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID), form, itoa(a.ID))
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountUpdate); rec.Code != http.StatusSeeOther {
		t.Fatalf("update gagal: %d\n%s", rec.Code, rec.Body.String())
	}

	got, _ := env.q.GetAccount(t.Context(), a.ID)
	if got.ContactPhone == nil || *got.ContactPhone != newPhone {
		t.Errorf("BL-106: admin harus bisa mengubah HP, got %v want %q", got.ContactPhone, newPhone)
	}
}

// TestAccountDetailView_VillageBudgetMasked: anggaran desa tersamar utk
// Support, tampil apa adanya utk role lain (sales/csm/admin/manager).
// Diperbaiki audit FLS M9-1. Diuji LANGSUNG atas accountDetailView (bukan lewat
// HTTP AccountDetail) — Support ScopeNone (TestAccounts_F3_SupportNolBaris)
// membuat filter.Allows di AccountDetail SELALU menolak Support (404) sebelum
// accountDetailView pernah dipanggil, jadi tak ada jalur HTTP nyata utk body
// "Support melihat anggaran". Ini wiring/pertahanan-berlapis, pola sama dgn
// subscriptions_fls_test.go.
func TestAccountDetailView_VillageBudgetMasked(t *testing.T) {
	env, uid := setupAccounts(t)
	code, err := env.q.GenerateEntityCode(t.Context(), env.tenantID, codes.EntityAccount)
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	a, err := env.q.CreateAccount(t.Context(), db.CreateAccountParams{
		TenantID:      env.tenantID,
		EntityCode:    &code,
		VillageName:   "Desa Anggaran",
		AccountType:   "prospect",
		AccountOwner:  &uid,
		VillageBudget: numFrom(t, "750000000"),
	})
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}
	const wantBudget = "Rp 750.000.000" // BL-112: detail terformat rupiah (formatRupiah), bukan angka mentah

	view := func(role string) string {
		req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID), nil, itoa(a.ID))
		var got string
		env.runAccount(uid, "owner", role, req, func(w http.ResponseWriter, r *http.Request) {
			got = env.h.accountDetailView(r.Context(), "", a).VillageBudget
		})
		return got
	}

	if got := view("support"); got != flsHidden {
		t.Errorf("support: VillageBudget harus tersamar (%s), got %q", flsHidden, got)
	}
	for _, role := range []string{"sales", "csm", "admin", "manager"} {
		if got := view(role); got != wantBudget {
			t.Errorf("role %q: VillageBudget harus %q, got %q", role, wantBudget, got)
		}
	}
}

// TestAccountDetail_F4_ARRMaskedForSupport: MRR/ARR di kartu Ringkasan
// Langganan (M2-8) tersamar bagi Support — kelas sensitivitas sama dgn
// VillageBudget/deal Amount (maskARR, canSeeARR). Pola direct-call sama dgn
// TestAccountDetailView_VillageBudgetMasked — Support ScopeNone selalu 404 di
// jalur HTTP nyata (TestAccounts_F3_SupportNolBaris), jadi wiring masking
// diuji lewat pemanggilan langsung accountDetailView.
func TestAccountDetail_F4_ARRMaskedForSupport(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Langganan Rahasia", &uid, nil, nil)
	plan := env.seedPlan(t, "Paket Uji F4", "PLAN-F4-ARR", "500000")
	env.seedSubscription(t, a.ID, plan, &uid, "Active", "500000", "6000000")

	req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID), nil, itoa(a.ID))
	var v panel.AccountDetailView
	env.runAccount(uid, "owner", "support", req, func(w http.ResponseWriter, r *http.Request) {
		v = env.h.accountDetailView(r.Context(), "", a)
	})
	if v.Subscription.MRR != flsHidden || v.Subscription.ARR != flsHidden {
		t.Errorf("support: MRR/ARR harus tersamar (%s), got MRR=%q ARR=%q",
			flsHidden, v.Subscription.MRR, v.Subscription.ARR)
	}
}

// TestAccountDetail_F4_ARRVisibleForManager: Manager ada di allow-list
// canSeeARR → MRR/ARR tampil apa adanya (diformat formatRupiah).
func TestAccountDetail_F4_ARRVisibleForManager(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedAccount(t, "Desa Langganan Terbuka", &uid, nil, nil)
	plan := env.seedPlan(t, "Paket Uji F4b", "PLAN-F4-ARR2", "500000")
	env.seedSubscription(t, a.ID, plan, &uid, "Active", "500000", "6000000")

	req := accountsReq(http.MethodGet, "/w/test/accounts/"+itoa(a.ID), nil, itoa(a.ID))
	var v panel.AccountDetailView
	env.runAccount(uid, "owner", "manager", req, func(w http.ResponseWriter, r *http.Request) {
		v = env.h.accountDetailView(r.Context(), "", a)
	})
	if v.Subscription.MRR == flsHidden || v.Subscription.MRR == "" {
		t.Errorf("manager: MRR tak boleh tersamar/kosong, got %q", v.Subscription.MRR)
	}
	if v.Subscription.ARR == flsHidden || v.Subscription.ARR == "" {
		t.Errorf("manager: ARR tak boleh tersamar/kosong, got %q", v.Subscription.ARR)
	}
	if !strings.Contains(v.Subscription.MRR, "500.000") {
		t.Errorf("manager: MRR harus memuat nilai terformat 500.000, got %q", v.Subscription.MRR)
	}
	if !strings.Contains(v.Subscription.ARR, "6.000.000") {
		t.Errorf("manager: ARR harus memuat nilai terformat 6.000.000, got %q", v.Subscription.ARR)
	}
}
