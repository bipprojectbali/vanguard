package handler

import (
	"testing"

	"go_starter/internal/db"
)

// sales_convert_amount_test.go — regresi 100x: prefill Amount review konversi
// HARUS membuang pecahan skala kolom NUMERIC(15,2) (moneyRupiahStr, BUKAN
// numericStr). Tanpa ini "5000000.00" lolos ke numgroup.js yang membuang SEMUA
// non-digit (titik desimal ikut) → "500000000" (100x) saat deal disimpan.
// Pola sama dijaga utk prefill Deal & Lead (sales_deals_form_test, sales_leads_form_test).

func TestConvertPrefill_AmountTanpaPecahan(t *testing.T) {
	cases := []struct {
		in   string // EstimatedValue lead (bentuk kolom NUMERIC(15,2))
		want string // prefill Amount review
	}{
		{"5000000.00", "5000000"},
		{"7500000.50", "7500000"},
		{"5000000", "5000000"},
		{"", ""}, // NULL → kosong
	}
	for _, c := range cases {
		l := db.Lead{EstimatedValue: numFrom(t, c.in)}
		got := convertPrefill(l, true).Amount
		if got != c.want {
			t.Errorf("convertPrefill Amount(%q) = %q, mau %q (regresi 100x)", c.in, got, c.want)
		}
	}
}
