package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/codes"
	"go_starter/internal/db"
)

// accounts_form_bl60_test.go — regresi BL-60 (penyederhanaan form Desa) sisi
// handler:
//   (1) Teritori dilepas dari UI → AccountUpdate TAK boleh menghapus data
//       teritori lama walau form tak mengirimnya (pola data-loss guard, sama
//       dgn contact_phone F4).
//   (2) Anggaran (APBDes) jadi input UANG → backend menerima nilai terkelompok
//       ("5.000.000") lewat cleanThousands, sama sahnya dgn digit polos.

// TestAccountUpdate_PreservesTerritory: desa punya teritori tersimpan; update
// profil (form tanpa field territory) tak boleh menimpanya jadi NULL.
func TestAccountUpdate_PreservesTerritory(t *testing.T) {
	env, uid := setupAccounts(t)

	code, err := env.q.GenerateEntityCode(t.Context(), env.tenantID, codes.EntityAccount)
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	terr := "Zona Barat"
	a, err := env.q.CreateAccount(t.Context(), db.CreateAccountParams{
		TenantID:     env.tenantID,
		EntityCode:   &code,
		VillageName:  "Desa Berteritori",
		AccountType:  "prospect",
		AccountOwner: &uid,
		Territory:    &terr,
	})
	if err != nil {
		t.Fatalf("seed account: %v", err)
	}

	form := accountFormValues("customer") // tak ada "territory"; nama dipertahankan (tanpa village_id)
	req := accountsReq(http.MethodPost, "/w/test/accounts/"+itoa(a.ID), form, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountUpdate)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Fatalf("update harus ok=saved, got %q (status %d)", loc, rec.Code)
	}

	got, err := env.q.GetAccount(t.Context(), a.ID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if got.Territory == nil || *got.Territory != terr {
		t.Errorf("teritori harus dipertahankan (%q), got %v", terr, got.Territory)
	}
}

// TestAccountCreate_BudgetGroupedThousands: nilai anggaran terkelompok
// ("5.000.000") diterima backend & tersimpan sebagai 5000000 (cleanThousands).
func TestAccountCreate_BudgetGroupedThousands(t *testing.T) {
	env, uid := setupAccounts(t)
	v := firstVillage(t, env)

	form := withVillage(accountFormValues("prospect"), v)
	form.Set("village_budget", "5.000.000")
	if rec := createAccountForm(t, env, uid, form); rec.Code != http.StatusSeeOther {
		t.Fatalf("create gagal: %d\n%s", rec.Code, rec.Body.String())
	}

	got := accountByVillageCode(t, env, v.Code)
	if s := moneyRupiahStr(got.VillageBudget); s != "5000000" {
		t.Errorf("anggaran terkelompok harus tersimpan 5000000, got %q", s)
	}
}

// TestAccountCreate_BudgetInvalidRejected: anggaran non-angka ditolak backend
// (err=budget) — bukan 500 mentah, dan tak menyimpan baris.
func TestAccountCreate_BudgetInvalidRejected(t *testing.T) {
	env, uid := setupAccounts(t)

	// Anggaran diparse SEBELUM village_id, jadi err=budget muncul tanpa village_id.
	form := accountFormValues("prospect")
	form.Set("village_budget", "abc")
	rec := createAccountForm(t, env, uid, form)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=budget") {
		t.Errorf("anggaran invalid harus err=budget, got %q", loc)
	}
	if rows := env.allAccounts(t); len(rows) != 0 {
		t.Errorf("input invalid tak boleh menyimpan, ada %d baris", len(rows))
	}
}
