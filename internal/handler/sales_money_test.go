package handler

import (
	"math/big"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_money_test.go — unit murni aritmatika uang Quote (sales_money.go). TANPA DB:
// menguji pembulatan 2-dp & perlakuan NULL secara langsung, jadi tetap jalan walau
// TEST_DATABASE_URL kosong. numFrom/numEq dipakai bersama sales_quotes_test.go
// (paket sama) agar perbandingan NUMERIC tak bergantung pada bentuk string kanonik.

// numFrom membuat pgtype.Numeric dari string; "" = NULL (invalid). Fatal bila tak sah.
func numFrom(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if s == "" {
		return n // invalid = NULL
	}
	if err := n.Scan(s); err != nil {
		t.Fatalf("scan numeric %q: %v", s, err)
	}
	return n
}

// numEq benar bila got secara NILAI sama dengan want (dibandingkan sebagai big.Rat,
// bukan string) — "200000.00" == "200000" == "200000.0". Menutup perbedaan bentuk.
func numEq(got pgtype.Numeric, want string) bool {
	w, ok := new(big.Rat).SetString(want)
	if !ok {
		return false
	}
	return ratFromNumeric(got).Cmp(w) == 0
}

// TestItemSubtotal: subtotal = unit_price × qty × (1 − diskon/100), dibulatkan 2-dp
// half-up. Diskon 0/50/100 + pembulatan pecahan (mirror acceptance snapshot harga).
func TestItemSubtotal(t *testing.T) {
	cases := []struct {
		name  string
		price string
		qty   int32
		disc  string // "" = NULL (tanpa diskon)
		want  string
	}{
		{"tanpa diskon", "100000.00", 1, "", "100000.00"},
		{"qty ganda", "100000.00", 2, "", "200000.00"},
		{"diskon 0", "100000.00", 3, "0", "300000.00"},
		{"diskon 50", "100000.00", 1, "50", "50000.00"},
		{"diskon 100", "100000.00", 1, "100", "0.00"},
		{"harga pecahan", "99999.99", 1, "", "99999.99"},
		{"pembulatan half-up", "1", 1, "33.33", "0.67"}, // 0.6667 → 0.67
		{"harga NULL = 0", "", 5, "", "0.00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := itemSubtotal(numFrom(t, c.price), c.qty, numFrom(t, c.disc))
			if !numEq(got, c.want) {
				t.Errorf("itemSubtotal(%s, %d, %s) = %s, want %s",
					c.price, c.qty, c.disc, numericStr(got), c.want)
			}
		})
	}
}

// TestAddNumeric: Σ subtotal + tax → grand_total; NULL diperlakukan 0.
func TestAddNumeric(t *testing.T) {
	cases := []struct {
		name, a, b, want string
	}{
		{"desimal", "100.50", "0.25", "100.75"},
		{"a NULL", "", "5", "5.00"},
		{"b NULL", "200000", "", "200000.00"},
		{"keduanya NULL", "", "", "0.00"},
		{"subtotal + pajak", "200000.00", "22000.00", "222000.00"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := addNumeric(numFrom(t, c.a), numFrom(t, c.b))
			if !numEq(got, c.want) {
				t.Errorf("addNumeric(%q,%q) = %s, want %s", c.a, c.b, numericStr(got), c.want)
			}
		})
	}
}

// TestNumericGuards: guard validasi form (non-negatif & rentang inklusif diskon).
func TestNumericGuards(t *testing.T) {
	if numericNonNegative(numFrom(t, "-0.01")) {
		t.Error("nilai negatif harus ditolak numericNonNegative")
	}
	for _, s := range []string{"", "0", "0.01", "999999"} {
		if !numericNonNegative(numFrom(t, s)) {
			t.Errorf("%q harus non-negatif", s)
		}
	}
	// Diskon 0–100 inklusif (cermin quote_items_discount_chk).
	for _, s := range []string{"0", "50", "100"} {
		if !numericBetween(numFrom(t, s), 0, 100) {
			t.Errorf("%q harus dalam 0–100", s)
		}
	}
	for _, s := range []string{"-1", "100.01", "200"} {
		if numericBetween(numFrom(t, s), 0, 100) {
			t.Errorf("%q harus di luar 0–100", s)
		}
	}
}
