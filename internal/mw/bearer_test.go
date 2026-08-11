package mw

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// bearer_test.go — penjaga /mcp. Yang dijaga: hanya token yang PERSIS cocok
// lolos, dan kekeliruan (kosong, salah format, prefiks benar tapi tak lengkap)
// selalu ditolak — sebab rute di baliknya membaca database.

// probe menjalankan RequireBearer(token) dengan satu header Authorization dan
// mengembalikan status code + apakah handler dalam sempat jalan.
func callBearer(t *testing.T, token, authHeader string) (int, bool) {
	t.Helper()
	reached := false
	h := RequireBearer(token)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodPost, "/mcp", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, reached
}

func TestBearer_TokenBenarLolos(t *testing.T) {
	code, reached := callBearer(t, "s3cr3t-token-yang-cukup-panjang", "Bearer s3cr3t-token-yang-cukup-panjang")
	if code != http.StatusOK || !reached {
		t.Errorf("token benar harus lolos ke handler, got code=%d reached=%v", code, reached)
	}
}

// TestBearer_DitolakSemuaBentukSalah: satu tabel untuk semua cara gagal —
// masing-masing harus 401 DAN tak menyentuh handler di baliknya.
func TestBearer_DitolakSemuaBentukSalah(t *testing.T) {
	const token = "s3cr3t-token-yang-cukup-panjang"
	cases := []struct {
		name, header string
	}{
		{"tanpa header", ""},
		{"token salah", "Bearer token-yang-salah-sama-sekali"},
		{"prefiks benar tak lengkap", "Bearer s3cr3t-token-yang-cukup-panjan"}, // kurang 1 char
		{"token benar tanpa prefiks Bearer", token},
		{"skema lain", "Basic s3cr3t-token-yang-cukup-panjang"},
		{"Bearer tanpa token", "Bearer "},
		{"Bearer saja", "Bearer"},
		{"token jadi prefiks token benar", "Bearer s3cr3t"}, // panjang beda → tetap ditolak
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			code, reached := callBearer(t, token, c.header)
			if code != http.StatusUnauthorized {
				t.Errorf("%q harus 401, got %d", c.name, code)
			}
			if reached {
				t.Errorf("%q menembus ke handler — rute di baliknya membaca DB", c.name)
			}
		})
	}
}

// TestBearer_PrefiksCaseInsensitive: skema HTTP (RFC 7235) tak peka huruf besar-
// kecil pada kata "Bearer" — klien yang mengirim "bearer" tak boleh ditolak
// karena alasan yang salah. (Token-nya sendiri tetap peka.)
func TestBearer_PrefiksCaseInsensitive(t *testing.T) {
	const token = "s3cr3t-token-yang-cukup-panjang"
	if code, reached := callBearer(t, token, "bearer "+token); code != http.StatusOK || !reached {
		t.Errorf("prefiks 'bearer' huruf kecil harus tetap lolos, got code=%d", code)
	}
}

// TestBearer_TokenKosongMenolakSemua: jaring terakhir. Bila token kosong sampai
// ke middleware (kekeliruan wiring), ia harus menolak SEMUA — rute mati, bukan
// rute terbuka. Termasuk request yang membawa "Bearer " kosong.
func TestBearer_TokenKosongMenolakSemua(t *testing.T) {
	for _, header := range []string{"", "Bearer ", "Bearer apa-saja"} {
		if code, reached := callBearer(t, "", header); code != http.StatusUnauthorized || reached {
			t.Errorf("token kosong harus menolak %q (code=%d reached=%v)", header, code, reached)
		}
	}
}

// TestBearer_Menantang401DenganWWWAuthenticate: 401 wajib disertai header
// WWW-Authenticate agar klien tahu ini soal kredensial, bukan rute hilang.
func TestBearer_Menantang401DenganWWWAuthenticate(t *testing.T) {
	h := RequireBearer("token-apa-saja-cukup-panjang")(http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/mcp", nil))
	if got := rec.Header().Get("WWW-Authenticate"); got == "" {
		t.Error("401 harus menyertakan WWW-Authenticate")
	}
}
