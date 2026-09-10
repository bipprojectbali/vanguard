package handler

import (
	"net/url"
	"strings"
	"testing"
)

// sales_quotes_tax_test.go — BL-147: input Nominal Pajak = uang rupiah BULAT
// (moneyFieldRp + numgroup.js). Backend parseTaxForm membersihkan pemisah ribuan
// (cleanThousands) SEBELUM parse, jadi nilai terkelompok "5.000.000" (dari
// numgroup.js maupun ketikan manual, tanpa JS pun) sama sahnya dengan "5000000".
// tax_rate TIDAK dibersihkan (di sana titik = pemisah desimal persen).

// TestQuoteTax_AmountAcceptsGroupedThousands: tax_mode=amount + tax_amount ber-titik
// ribuan → ok=tax_saved & TaxAmount tersimpan sbg 5000000 (bukan gagal parse / 5).
func TestQuoteTax_AmountAcceptsGroupedThousands(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Pajak", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification") // BL-13: jendela quoting
	q := env.seedQuote(t, deal.ID, acc.ID, "0")

	form := url.Values{"tax_mode": {"amount"}, "tax_amount": {"5.000.000"}}
	rec := env.setTax(t, uid, deal.ID, q.ID, form)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=tax_saved") {
		t.Fatalf("nominal terkelompok harus ok=tax_saved, got %q (code %d)", loc, rec.Code)
	}
	if got := trimDecZeros(numericStr(env.mustGetQuote(t, q.ID).TaxAmount)); got != "5000000" {
		t.Errorf("TaxAmount harus 5000000 (titik ribuan dibuang), got %q", got)
	}
}

// TestQuoteTax_AmountPlainDigitsStillWork: digit polos "7500000" (tanpa JS/tanpa
// pemisah) tetap sah — cleanThousands idempoten untuk input tanpa titik.
func TestQuoteTax_AmountPlainDigitsStillWork(t *testing.T) {
	env, uid := setupAccounts(t)
	acc := env.seedAccount(t, "Desa Polos", &uid, nil, nil)
	deal := env.seedDeal(t, acc.ID, &uid)
	env.setDealStage(t, deal.ID, "Qualification")
	q := env.seedQuote(t, deal.ID, acc.ID, "0")

	form := url.Values{"tax_mode": {"amount"}, "tax_amount": {"7500000"}}
	rec := env.setTax(t, uid, deal.ID, q.ID, form)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=tax_saved") {
		t.Fatalf("digit polos harus ok=tax_saved, got %q (code %d)", loc, rec.Code)
	}
	if got := trimDecZeros(numericStr(env.mustGetQuote(t, q.ID).TaxAmount)); got != "7500000" {
		t.Errorf("TaxAmount harus 7500000, got %q", got)
	}
}
