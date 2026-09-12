package handler

import (
	"net/http"
	"strings"
	"testing"
)

// plans_money_test.go — uang: input terformat & prefill tanpa skala (BL-23).
// Setup/helper (planFormValues, seedPlanRow, allPlans) di plans_test.go.

// TestPlans_CreateMoneyFormatted membuktikan pasangan anti-100x sisi TULIS:
// base_price/setup_fee dikirim TERFORMAT dengan pemisah ribuan ("5.000.000" —
// seperti yang numgroup.js hasilkan, atau ketikan manual) → cleanThousands
// membuang titik SEBELUM optNumeric → tersimpan bulat 5000000, BUKAN 500000000
// (100x, yang terjadi bila titik tak dibuang). moneyField (view) meng-inputmode
// numeric-kan field ini; test menegakkan backend tetap penjaga.
func TestPlans_CreateMoneyFormatted(t *testing.T) {
	env, uid := setupAccounts(t)
	form := planFormValues("Paket Format", "PLAN-FMT", "Core")
	form.Set("base_price", "5.000.000")
	form.Set("setup_fee", "1.500.000")
	req := accountsReq(http.MethodPost, "/w/test/plans", form, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.PlanCreate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Fatalf("harus ok=created, got %q (status %d; body:\n%s)", loc, rec.Code, rec.Body.String())
	}
	rows := env.allPlans(t)
	if len(rows) != 1 {
		t.Fatalf("harus 1 baris, ada %d", len(rows))
	}
	p := rows[0]
	if got := moneyRupiahStr(p.BasePrice); got != "5000000" {
		t.Errorf("base_price terformat harus tersimpan 5000000 (bukan 100x), got %q", got)
	}
	if got := moneyRupiahStr(p.SetupFee); got != "1500000" {
		t.Errorf("setup_fee terformat harus tersimpan 1500000 (bukan 100x), got %q", got)
	}
}

// TestPlans_EditPrefillMoneyNoScale membuktikan pasangan anti-100x sisi BACA:
// planFormFields memakai moneyRupiahStr (bukan numericStr) untuk base_price/
// setup_fee → prefill = rupiah bulat "7500000" TANPA ekor ".00". Bila memakai
// numericStr ("7500000.00"), numgroup.js akan membuang titik desimal → tampil &
// tersimpan ulang "750000000" (100x). Test menjaga prefill tetap murni-digit.
func TestPlans_EditPrefillMoneyNoScale(t *testing.T) {
	env, uid := setupAccounts(t)
	form := planFormValues("Paket Prefill", "PLAN-PRE", "Core")
	form.Set("base_price", "7500000")
	form.Set("setup_fee", "250000")
	req := accountsReq(http.MethodPost, "/w/test/plans", form, "")
	if rec := env.runAccount(uid, "owner", "admin", req, env.h.PlanCreate); rec.Code != http.StatusSeeOther {
		t.Fatalf("create gagal, status %d; body:\n%s", rec.Code, rec.Body.String())
	}
	rows := env.allPlans(t)
	if len(rows) != 1 {
		t.Fatalf("harus 1 baris, ada %d", len(rows))
	}

	ff := planFormFields(rows[0])
	if strings.Contains(ff.BasePrice, ".") || ff.BasePrice != "7500000" {
		t.Errorf("prefill base_price harus bulat tanpa .00 = %q, got %q", "7500000", ff.BasePrice)
	}
	if strings.Contains(ff.SetupFee, ".") || ff.SetupFee != "250000" {
		t.Errorf("prefill setup_fee harus bulat tanpa .00 = %q, got %q", "250000", ff.SetupFee)
	}
}
