package handler

import (
	"testing"
	"time"
)

// jena_ai_ratelimit_test.go — bukti fixedWindowLimiter: N pertama dalam
// window lolos, ke-(N+1) ditolak, user LAIN tak ikut kena batas user ini,
// dan window kedaluwarsa mereset hitungan. Instance terpisah dari var paket
// jenaRateLimiter (bukan mutasi global yang bisa bocor ke test lain) —
// newFixedWindowLimiter diuji langsung.

func TestFixedWindowLimiter_AllowsUpToLimit(t *testing.T) {
	l := newFixedWindowLimiter(10, time.Minute)
	for i := 0; i < 10; i++ {
		if !l.Allow(1) {
			t.Fatalf("permintaan ke-%d (dalam batas) harus diizinkan", i+1)
		}
	}
}

func TestFixedWindowLimiter_BlocksAfterLimit(t *testing.T) {
	l := newFixedWindowLimiter(10, time.Minute)
	for i := 0; i < 10; i++ {
		l.Allow(1)
	}
	if l.Allow(1) {
		t.Error("permintaan ke-11 dalam window yang sama harus DITOLAK")
	}
}

func TestFixedWindowLimiter_PerUserIndependent(t *testing.T) {
	l := newFixedWindowLimiter(10, time.Minute)
	for i := 0; i < 10; i++ {
		l.Allow(1)
	}
	if !l.Allow(2) {
		t.Error("user LAIN tak boleh ikut kena batas user 1")
	}
}

func TestFixedWindowLimiter_ResetsAfterWindowExpires(t *testing.T) {
	l := newFixedWindowLimiter(1, 20*time.Millisecond)
	if !l.Allow(1) {
		t.Fatal("permintaan pertama harus diizinkan")
	}
	if l.Allow(1) {
		t.Fatal("permintaan kedua SEBELUM window kedaluwarsa harus ditolak")
	}
	time.Sleep(30 * time.Millisecond)
	if !l.Allow(1) {
		t.Error("setelah window kedaluwarsa, hitungan harus reset & permintaan diizinkan lagi")
	}
}
