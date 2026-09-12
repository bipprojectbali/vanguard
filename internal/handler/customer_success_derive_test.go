package handler

import "testing"

// customer_success_derive_test.go — logika murni turunan health/skor CS
// (deriveHealthStatus, deriveScoreTrend) + helper strptr dipakai lintas file
// customer_success_*_test.go. Tanpa dependensi HTTP/env — lihat
// customer_success_test.go untuk setup & test F2/F3.

// TestDeriveHealthStatus mengunci pemetaan skor→status (BL-24) di ambang
// ≥80 Healthy · 40–79 At-Risk · <40 Critical. Batas atas/bawah tiap band diuji
// eksplisit (79 vs 80, 39 vs 40) karena di situlah off-by-one paling mungkin;
// nil (belum ada dasar hitung) HARUS nil, bukan default ke status apa pun.
func TestDeriveHealthStatus(t *testing.T) {
	i16 := func(v int16) *int16 { return &v }
	cases := []struct {
		name string
		in   *int16
		want *string // nil = harap nil
	}{
		{"nil belum dinilai", nil, nil},
		{"0 kritis", i16(0), strptr("Critical")},
		{"39 kritis (batas atas band)", i16(39), strptr("Critical")},
		{"40 at-risk (batas bawah band)", i16(40), strptr("At-Risk")},
		{"79 at-risk (batas atas band)", i16(79), strptr("At-Risk")},
		{"80 healthy (batas bawah band)", i16(80), strptr("Healthy")},
		{"100 healthy", i16(100), strptr("Healthy")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := deriveHealthStatus(c.in)
			switch {
			case c.want == nil && got != nil:
				t.Errorf("in=%v: harap nil, got %q", c.in, *got)
			case c.want != nil && got == nil:
				t.Errorf("in=%v: harap %q, got nil", c.in, *c.want)
			case c.want != nil && got != nil && *got != *c.want:
				t.Errorf("in=%v: harap %q, got %q", c.in, *c.want, *got)
			}
		})
	}
}

// TestDeriveScoreTrend: arah tren dari (prev, curr) dgn dead-band. Batas eksplisit
// di sekitar ±deadband + kasus nil (belum ada pembanding → nil, BUKAN "Stable").
func TestDeriveScoreTrend(t *testing.T) {
	i16 := func(v int16) *int16 { return &v }
	const dead = healthTrendDeadband // 3
	cases := []struct {
		name       string
		prev, curr *int16
		want       *string // nil = harap nil
	}{
		{"prev nil → nil (snapshot pertama)", nil, i16(80), nil},
		{"curr nil → nil (skor kosong)", i16(80), nil, nil},
		{"keduanya nil → nil", nil, nil, nil},
		{"delta 0 → Stable", i16(50), i16(50), strptr("Stable")},
		{"delta +3 (= dead-band) → Stable", i16(50), i16(53), strptr("Stable")},
		{"delta +4 (> dead-band) → Improving", i16(50), i16(54), strptr("Improving")},
		{"delta −3 (= dead-band) → Stable", i16(50), i16(47), strptr("Stable")},
		{"delta −4 (< −dead-band) → Declining", i16(50), i16(46), strptr("Declining")},
		{"lonjakan besar naik → Improving", i16(10), i16(90), strptr("Improving")},
		{"terjun besar → Declining", i16(90), i16(10), strptr("Declining")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := deriveScoreTrend(c.prev, c.curr, dead)
			switch {
			case c.want == nil && got != nil:
				t.Errorf("prev=%v curr=%v: harap nil, got %q", c.prev, c.curr, *got)
			case c.want != nil && got == nil:
				t.Errorf("prev=%v curr=%v: harap %q, got nil", c.prev, c.curr, *c.want)
			case c.want != nil && got != nil && *got != *c.want:
				t.Errorf("prev=%v curr=%v: harap %q, got %q", c.prev, c.curr, *c.want, *got)
			}
		})
	}
}

func strptr(s string) *string { return &s }
