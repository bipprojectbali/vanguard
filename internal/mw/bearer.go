package mw

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// bearer.go — penjaga rute service-to-service (dipakai /mcp). Berbeda dari
// RequireAuth yang menjaga rute MANUSIA lewat session cookie: klien di sini
// adalah agent/program, jadi ia membawa token statis di header, bukan sesi.

// RequireBearer menolak request yang tak membawa `Authorization: Bearer <token>`
// yang cocok. token = rahasia yang diharapkan (dari config.MCPToken).
//
// Pemanggil WAJIB memastikan token tak kosong sebelum memasang middleware ini
// (routes.go hanya mendaftarkan rute /mcp bila token diisi). Sebagai jaring
// terakhir, token kosong di sini menolak SEMUA request — supaya kekeliruan
// wiring menghasilkan rute mati, bukan rute terbuka tanpa penjaga.
func RequireBearer(token string) func(http.Handler) http.Handler {
	want := []byte(token)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(want) == 0 || !bearerMatches(r.Header.Get("Authorization"), want) {
				// 401 + WWW-Authenticate: klien tahu ini soal kredensial, bukan
				// rute tak ada (404) atau terlarang selamanya (403).
				w.Header().Set("WWW-Authenticate", "Bearer")
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// bearerMatches memeriksa header Authorization cocok dengan token.
//
// subtle.ConstantTimeCompare: perbandingan token WAJIB tak bergantung isi —
// perbandingan byte-per-byte biasa berhenti di ketidakcocokan pertama, dan beda
// waktunya (walau mikrodetik) membocorkan berapa banyak prefiks yang benar,
// cukup untuk menebak token karakter demi karakter dari jarak jauh.
func bearerMatches(header string, want []byte) bool {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return false
	}
	got := []byte(header[len(prefix):])
	// ConstantTimeCompare mengembalikan 1 hanya bila panjang SAMA dan isi sama.
	return subtle.ConstantTimeCompare(got, want) == 1
}
