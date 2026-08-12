package handler

import (
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_quotes_form.go — parse & validasi form Quote (header) & item, dipakai
// bersama create/update. Dipisah dari aksi (sales_quotes.go) & baca
// (sales_quotes_page.go) agar aturan "kosong = NULL / terisi = wajib sah" punya
// SATU tempat: create & edit tak boleh menerima nilai yang berbeda sahnya untuk
// kolom yang sama. Meniru sales_deals_form.go.
//
// Status quote di sini = CERMIN CHECK constraint migrasi 00010 (quotes_status_chk):
// penolakan terjadi di sini SEBELUM DB; CHECK jaring terakhir. Pajak = INPUT MANUAL
// (keputusan scope terkunci) → field form, bukan konstanta PPN.

const (
	maxQuoteNameLen  = 200  // quote_name (nullable)
	maxQuoteTermsLen = 2000 // payment_terms / notes_terms — teks bebas, guard luapan
)

// quoteInitialStatus = status setiap quote baru. Transisi berikutnya = aksi manual
// tersendiri (UpdateQuoteStatus); approval flow ditunda (keputusan scope).
const quoteInitialStatus = "Draft"

// validQuoteStatuses = himpunan status legal (cermin quotes_status_chk 00010).
// Approval flow ditunda → semua status boleh diset manual dari kontrol status.
var validQuoteStatuses = map[string]struct{}{
	"Draft": {}, "Sent": {}, "Under Review": {},
	"Accepted": {}, "Rejected": {}, "Expired": {},
}

// quoteStatusOptions = status untuk dropdown, BERURUT (map validasi tak berurut).
// Harus himpunan yang sama dengan validQuoteStatuses & CHECK 00010.
var quoteStatusOptions = []string{"Draft", "Sent", "Under Review", "Accepted", "Rejected", "Expired"}

// compile-time: opsi & map validasi sepakat (panjang sama). Berbeda = dropdown
// menawarkan nilai yang ditolak backend, atau sebaliknya.
var _ = func() struct{} {
	if len(quoteStatusOptions) != len(validQuoteStatuses) {
		panic("quotes: opsi status tak sinkron dengan map validasi")
	}
	return struct{}{}
}()

// quoteForm = nilai form HEADER quote yang SUDAH divalidasi. Semua kolom opsional
// (quote menempel ke deal induk untuk account/deal_id). TaxAmount pgtype.Numeric
// (nil-valid = NULL). PreparedBy *int64 (nil = tak ditunjuk).
type quoteForm struct {
	QuoteName      *string
	ExpirationDate pgtype.Date
	PaymentTerms   *string
	NotesTerms     *string
	TaxAmount      pgtype.Numeric
	PreparedBy     *int64
}

// parseQuoteForm membaca & memvalidasi form header. (form, "") bila sah, atau
// (zero, kode) yang dipetakan wsErrMsg. account_id & deal_id TAK di sini —
// diwarisi dari deal induk oleh handler (alur nest).
func parseQuoteForm(fv func(string) string) (quoteForm, string) {
	var f quoteForm

	// quote_name opsional: kosong = NULL; terisi wajib ≤ batas.
	if s := strings.TrimSpace(fv("quote_name")); s != "" {
		if len(s) > maxQuoteNameLen {
			return quoteForm{}, "quote_name"
		}
		f.QuoteName = &s
	}

	// expiration_date opsional (belum ditetapkan = NULL).
	ed, code := optDate(fv("expiration_date"))
	if code != "" {
		return quoteForm{}, code
	}
	f.ExpirationDate = ed

	// payment_terms / notes_terms teks bebas opsional (guard panjang).
	if pt := optTrim(fv("payment_terms")); pt != nil {
		if len(*pt) > maxQuoteTermsLen {
			return quoteForm{}, "terms"
		}
		f.PaymentTerms = pt
	}
	if nt := optTrim(fv("notes_terms")); nt != nil {
		if len(*nt) > maxQuoteTermsLen {
			return quoteForm{}, "terms"
		}
		f.NotesTerms = nt
	}

	// tax_amount MANUAL: kosong = NULL; terisi wajib desimal ≥ 0.
	tax, code := optNumeric(fv("tax_amount"), "tax")
	if code != "" {
		return quoteForm{}, code
	}
	if tax.Valid && !numericNonNegative(tax) {
		return quoteForm{}, "tax"
	}
	f.TaxAmount = tax

	// prepared_by opsional: kosong = NULL; terisi wajib id positif. Picker hanya
	// menawarkan anggota workspace; id tak dikenal ternetralkan saat resolusi nama.
	pb, code := optPositiveID(fv("prepared_by"), "prepared_by")
	if code != "" {
		return quoteForm{}, code
	}
	f.PreparedBy = pb

	return f, ""
}

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
