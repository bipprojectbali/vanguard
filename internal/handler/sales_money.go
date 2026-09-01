package handler

import (
	"math/big"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_money.go — aritmatika uang untuk Quote Builder (Modul 4 Sales). Satu-satunya
// tempat modul ini menghitung desimal: subtotal item & penjumlahan total. pgtype.Numeric
// hanya bisa .Scan/.Value (string), jadi konversi lewat big.Rat memberi aritmatika
// EKSAK (tanpa galat float) lalu dibulatkan sekali ke skala kolom NUMERIC(15,2).
//
// Tanpa dependency baru (rule 10) — math/big ada di stdlib. Semua nilai uang di sini
// non-negatif (harga, kuantitas, diskon 0–100) → pembulatan half-away-from-zero
// (big.Rat.FloatString) setara half-up.

// moneyScale = jumlah digit desimal kolom uang (NUMERIC(15,2)). Hasil hitung SELALU
// dibulatkan ke skala ini sebelum ditulis agar snapshot cocok dgn presisi DB.
const moneyScale = 2

// ratFromNumeric mengubah pgtype.Numeric → *big.Rat. NULL/invalid → 0 (nol rasional):
// item tanpa harga/diskon dihitung sebagai nol, bukan galat — validasi bentuk terjadi
// di parse form, di sini murni aritmatika.
func ratFromNumeric(n pgtype.Numeric) *big.Rat {
	s := numericStr(n)
	if s == "" {
		return new(big.Rat)
	}
	r, ok := new(big.Rat).SetString(s)
	if !ok {
		return new(big.Rat)
	}
	return r
}

// ratToNumeric membulatkan *big.Rat ke moneyScale desimal (half-up untuk nilai
// non-negatif) lalu memuatnya ke pgtype.Numeric via string — jalur yang sama dgn
// optNumeric agar representasi konsisten. FloatString membulatkan digit terakhir ke
// terdekat, setengah dijauhkan dari nol.
func ratToNumeric(r *big.Rat) pgtype.Numeric {
	var out pgtype.Numeric
	// FloatString selalu menghasilkan desimal sah (mis. "0.00", "12345.50").
	_ = out.Scan(r.FloatString(moneyScale))
	return out
}

// itemSubtotal menghitung subtotal satu baris quote (SNAPSHOT, dihitung app):
//
//	subtotal = unit_price × quantity × (1 − discount_pct/100)
//
// Dibulatkan sekali ke NUMERIC(15,2). unit_price = harga yang sudah di-snapshot dari
// plans.base_price; discount NULL/kosong = tanpa diskon (faktor 1). quantity dijamin
// > 0 oleh parse form + CHECK DB, tapi tetap benar bila 0 (subtotal 0).
func itemSubtotal(unitPrice pgtype.Numeric, qty int32, discountPct pgtype.Numeric) pgtype.Numeric {
	price := ratFromNumeric(unitPrice)
	qtyRat := new(big.Rat).SetInt64(int64(qty))

	// faktor = 1 − discount/100
	discount := ratFromNumeric(discountPct)
	factor := new(big.Rat).Sub(
		big.NewRat(1, 1),
		new(big.Rat).Quo(discount, big.NewRat(100, 1)),
	)

	res := new(big.Rat).Mul(price, qtyRat)
	res.Mul(res, factor)
	return ratToNumeric(res)
}

// addNumeric menjumlahkan dua NUMERIC (Σ subtotal + tax → grand_total). NULL diperlakukan
// 0 (quote tanpa item: grand = tax; tanpa tax: grand = Σsubtotal). Hasil dibulatkan ke
// NUMERIC(15,2) — masukan sudah 2-dp jadi ini idempoten, tapi tetap menormalkan bentuk.
func addNumeric(a, b pgtype.Numeric) pgtype.Numeric {
	sum := new(big.Rat).Add(ratFromNumeric(a), ratFromNumeric(b))
	return ratToNumeric(sum)
}

// percentOfNumeric = base × pct / 100 (EKSAK via big.Rat, dibulatkan moneyScale).
// Dipakai pajak mode 'percent' (BL-14): tax_amount = subtotal × tax_rate/100. NULL
// pada salah satu operand = 0 → hasil 0 (subtotal/rate belum ada = tanpa pajak).
func percentOfNumeric(base, pct pgtype.Numeric) pgtype.Numeric {
	res := new(big.Rat).Mul(ratFromNumeric(base), ratFromNumeric(pct))
	res.Quo(res, big.NewRat(100, 1))
	return ratToNumeric(res)
}

// numericNonNegative benar bila n ≥ 0 (atau NULL/invalid = 0, dianggap non-negatif).
// Dipakai validasi form: pajak & harga tak boleh negatif (di luar CHECK DB → dijaga app).
func numericNonNegative(n pgtype.Numeric) bool {
	return ratFromNumeric(n).Sign() >= 0
}

// numericBetween benar bila lo ≤ n ≤ hi (batas inklusif). Dipakai validasi diskon
// (0–100) SEBELUM DB — cermin quote_items_discount_chk (migrasi 00010).
func numericBetween(n pgtype.Numeric, lo, hi int64) bool {
	r := ratFromNumeric(n)
	return r.Cmp(big.NewRat(lo, 1)) >= 0 && r.Cmp(big.NewRat(hi, 1)) <= 0
}

// numericGreater benar bila a > b (perbandingan EKSAK via big.Rat, tanpa galat
// float). Dipakai deteksi Upsell renewal (MRR baru > previous_value) — batas yang
// menentukan apakah renewal butuh persetujuan.
func numericGreater(a, b pgtype.Numeric) bool {
	return ratFromNumeric(a).Cmp(ratFromNumeric(b)) > 0
}

// mulNumericInt mengalikan NUMERIC dengan bilangan bulat (EKSAK, lalu dibulatkan ke
// moneyScale). Dipakai menurunkan ARR = MRR × monthsPerYear saat renewal.
func mulNumericInt(n pgtype.Numeric, k int64) pgtype.Numeric {
	return ratToNumeric(new(big.Rat).Mul(ratFromNumeric(n), new(big.Rat).SetInt64(k)))
}
