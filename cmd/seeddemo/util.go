package main

import (
	"fmt"
	"math/rand"

	"github.com/jackc/pgx/v5/pgtype"
)

// util.go — helper bersama seluruh domain seeddemo. rng lokal (bukan
// math/rand global) di-seed dari tag run: reproducible ANTAR file dalam satu
// run yang sama, tapi beda tiap kali `go run ./cmd/seeddemo` dieksekusi ulang
// (tag berbasis timestamp), sehingga variasi tetap terasa "baru" tiap demo.

// newRNG membuat generator lokal dari tag run (mis. "0826-153000") sebagai
// seed — hash string sederhana, cukup untuk kebutuhan distribusi demo.
func newRNG(tag string) *rand.Rand {
	var seed int64
	for _, c := range tag {
		seed = seed*31 + int64(c)
	}
	return rand.New(rand.NewSource(seed))
}

// desaSeqOffset menghasilkan angka 0-4999 deterministik dari tag run (hash
// string sederhana, pola sama newRNG) — basis segmen ke-4 village_code
// pseudo-Kemendagri (lihat accounts.go/leads.go). Digeser seragam thd basis
// lokal yang SUDAH disjoint (1..40 akun biasa vs 1000+seq akun hasil
// konversi lead) supaya tak tabrakan sesama baris satu run, DAN beda tiap
// run (tag beda) supaya rerun ke tenant yang sama tak bentrok idx_accounts_code
// (UNIQUE tenant_id, village_code) — pengganti prefix "VC-<tag>-" lama.
func desaSeqOffset(tag string) int {
	var h int
	for _, c := range tag {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return h % 5000
}

// num mengubah string desimal → pgtype.Numeric (pola sama cmd/seedsubs).
func num(s string) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		return n, fmt.Errorf("numeric %q: %w", s, err)
	}
	return n, nil
}

// mustNum panic bila string literal internal (bukan input user) ternyata
// bukan angka valid — dipakai utk konstanta harga/skor yang sudah kita tulis
// sendiri, supaya galat ketik ketahuan saat compile-time-ish (saat run),
// bukan menyusup jadi NULL diam-diam.
func mustNum(s string) pgtype.Numeric {
	n, err := num(s)
	if err != nil {
		panic(err)
	}
	return n
}

// pick mengembalikan satu elemen acak dari slice non-kosong.
func pick[T any](rng *rand.Rand, items []T) T {
	return items[rng.Intn(len(items))]
}

// pickOrNil mengembalikan pointer ke salinan elemen acak dgn peluang pctTrue
// (0-100), selain itu nil — dipakai utk kolom nullable yang sebagian saja
// diisi (mis. website, territory) supaya distribusinya terkontrol, bukan
// separuh-separuh murni acak.
func pickOrNil[T any](rng *rand.Rand, pctTrue int, items []T) *T {
	if rng.Intn(100) >= pctTrue {
		return nil
	}
	v := pick(rng, items)
	return &v
}

// weighted = satu pilihan berbobot untuk weightedPick.
type weighted[T any] struct {
	value  T
	weight int
}

// weightedPick memilih satu value sesuai proporsi weight — dipakai utk
// distribusi enum yang disengaja (mis. 28 customer/8 prospect/4 former)
// alih-alih pick() seragam yang akan meratakan semua nilai.
func weightedPick[T any](rng *rand.Rand, opts []weighted[T]) T {
	total := 0
	for _, o := range opts {
		total += o.weight
	}
	r := rng.Intn(total)
	for _, o := range opts {
		if r < o.weight {
			return o.value
		}
		r -= o.weight
	}
	return opts[len(opts)-1].value
}

// ptr adalah shorthand &v utk literal yang butuh pointer (kolom nullable).
func ptr[T any](v T) *T { return &v }

// intn32 mengembalikan int32 acak di rentang [min,max] (inklusif).
func intn32(rng *rand.Rand, min, max int32) int32 {
	return min + rng.Int31n(max-min+1)
}
