package handler

import (
	"sync"
	"time"
)

// jena_ai_ratelimit.go — kontrol BIAYA (bukan kontrol keamanan) panggilan
// proxy Claude berbayar-per-token. In-memory per-proses, fixed-window: cukup
// untuk PoC single-binary/single-process (gotcha #17 belum ada — grep nihil
// rate-limit di seluruh project sebelum BL-162 fase 2 ini). Reset saat restart
// dapat diterima; TIDAK dibuat DB-backed/distribusi karena bukan itu yang
// dilindungi (isolasi tenant tetap RLS, izin tetap Casbin/ownership/FLS).
// Pola cache in-process sama seperti internal/settings/settings.go. Spesifik
// satu endpoint (bukan middleware umum) — hindari abstraksi prematur untuk PoC.

// fixedWindowLimiter membatasi N kejadian per user dalam jendela waktu tetap
// (bukan sliding/token-bucket — cukup sederhana untuk kontrol biaya, bukan SLA
// keras). window direset ke waktu SEKARANG saat kejadian pertama setelah
// jendela sebelumnya kedaluwarsa — bukan dijadwalkan tetap (mis. tiap jam
// bulat) — supaya user yang jarang bertanya tak pernah "kehabisan" jatah
// karena jendela orang lain.
type fixedWindowLimiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	state  map[int64]windowState
}

type windowState struct {
	count       int
	windowStart time.Time
}

func newFixedWindowLimiter(limit int, window time.Duration) *fixedWindowLimiter {
	return &fixedWindowLimiter{
		limit:  limit,
		window: window,
		state:  make(map[int64]windowState),
	}
}

// Allow melaporkan apakah userID masih boleh melakukan satu kejadian lagi, dan
// MENCATAT kejadian itu bila ya (side-effect disengaja — call-site tak perlu
// memanggil fungsi kedua "commit").
func (l *fixedWindowLimiter) Allow(userID int64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	st, ok := l.state[userID]
	if !ok || now.Sub(st.windowStart) >= l.window {
		st = windowState{count: 0, windowStart: now}
	}
	if st.count >= l.limit {
		l.state[userID] = st
		return false
	}
	st.count++
	l.state[userID] = st
	return true
}

// jenaRateLimiter — 10 pertanyaan / 10 menit per user (kontrol biaya PoC, lihat
// komentar paket di atas). Dicek di awal JenaAIAsk SEBELUM memanggil claudeClient.
var jenaRateLimiter = newFixedWindowLimiter(10, 10*time.Minute)
