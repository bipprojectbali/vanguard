package handler

import (
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_quote_items_form.go — parse & validasi form baris quote (quote_items):
// tambah & sunting. Dipisah dari sales_quotes_form.go (form HEADER quote) semata
// untuk file health — dua concern independen (header vs baris) meniru
// sales_deals_form.go.

// quoteItemForm = nilai form TAMBAH item yang sudah divalidasi. PlanID wajib (harga
// di-snapshot dari plan). DiscountPct opsional (NULL = tanpa diskon).
type quoteItemForm struct {
	PlanID      int64
	Quantity    int32
	DiscountPct pgtype.Numeric
}

// parseQuoteItemForm memvalidasi form tambah item. plan_id wajib (jangkar harga
// snapshot); quantity wajib > 0; discount 0–100 (cermin CHECK 00010).
func parseQuoteItemForm(fv func(string) string) (quoteItemForm, string) {
	var f quoteItemForm

	plan, err := strconv.ParseInt(strings.TrimSpace(fv("plan_id")), 10, 64)
	if err != nil || plan <= 0 {
		return quoteItemForm{}, "plan_req"
	}
	f.PlanID = plan

	qty, code := parseQuantity(fv("quantity"))
	if code != "" {
		return quoteItemForm{}, code
	}
	f.Quantity = qty

	disc, code := parseDiscount(fv("discount_pct"))
	if code != "" {
		return quoteItemForm{}, code
	}
	f.DiscountPct = disc

	return f, ""
}

// quoteItemEditForm = nilai form SUNTING item (qty/diskon). plan_id TAK diubah —
// unit_price tetap snapshot harga saat item dibuat.
type quoteItemEditForm struct {
	Quantity    int32
	DiscountPct pgtype.Numeric
}

// parseQuoteItemEditForm memvalidasi form sunting item (tanpa plan_id).
func parseQuoteItemEditForm(fv func(string) string) (quoteItemEditForm, string) {
	var f quoteItemEditForm

	qty, code := parseQuantity(fv("quantity"))
	if code != "" {
		return quoteItemEditForm{}, code
	}
	f.Quantity = qty

	disc, code := parseDiscount(fv("discount_pct"))
	if code != "" {
		return quoteItemEditForm{}, code
	}
	f.DiscountPct = disc

	return f, ""
}

// parseQuantity mengurai kuantitas item (wajib bilangan bulat > 0, cermin
// quote_items_quantity_chk). Kosong/tak terurai/≤0 → (0, "qty").
func parseQuantity(s string) (int32, string) {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 32)
	if err != nil || n <= 0 {
		return 0, "qty"
	}
	return int32(n), ""
}

// parseDiscount mengurai diskon persen opsional (kosong = NULL; terisi wajib 0–100,
// cermin quote_items_discount_chk). Di luar rentang/tak terurai → (zero, "discount").
func parseDiscount(s string) (pgtype.Numeric, string) {
	d, code := optNumeric(s, "discount")
	if code != "" {
		return pgtype.Numeric{}, code
	}
	if d.Valid && !numericBetween(d, 0, 100) {
		return pgtype.Numeric{}, "discount"
	}
	return d, ""
}

// optPositiveID mengurai id opsional (kosong = nil; terisi wajib bilangan bulat
// positif). code dioper pemanggil agar pesan galat menyebut field yang benar.
func optPositiveID(s, code string) (*int64, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, ""
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return nil, code
	}
	return &n, ""
}
