package handler

import (
	"net/http"
	"strings"
	"testing"
)

// accounts_search_test.go — regresi BL-6: pencarian daftar desa (?q=). Yang
// dijaga di sini adalah dua sifat yang harus benar BERSAMA:
//   - MENYEMPITKAN: ?q= menyaring baris via ILIKE contains pada nama/kode desa,
//     case-insensitive.
//   - TAK MEMPERLUAS: pencarian berjalan DI ATAS filter kepemilikan F3 — ia tak
//     pernah bisa memunculkan desa di luar cakupan aktor. Sales yang mencari kata
//     yang cocok dengan desa milik orang lain tetap tak melihatnya.
// (Kontrak URL kotak cari/pager/tab diuji di view: panel/accounts_search_test.go.)

// TestAccountsList_SearchNarrows: ?q= menyaring baris; pencarian case-insensitive
// (huruf kecil "suka" mencocokkan "Sukamaju").
func TestAccountsList_SearchNarrows(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAccount(t, "Desa Sukamaju", &uid, nil, nil)
	env.seedAccount(t, "Desa Mekarsari", &uid, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts?q=suka", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Sukamaju") {
		t.Errorf("q=suka harus memuat desa yang cocok (Desa Sukamaju)")
	}
	if strings.Contains(body, "Desa Mekarsari") {
		t.Errorf("q=suka tak boleh memuat desa yang tak cocok (Desa Mekarsari)")
	}
}

// TestAccountsList_SearchNoMatch: kueri tanpa hasil → empty-state pencarian
// (bukan "belum ada desa" yang menyesatkan).
func TestAccountsList_SearchNoMatch(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedAccount(t, "Desa Sukamaju", &uid, nil, nil)

	req := accountsReq(http.MethodGet, "/w/test/accounts?q=zzz-tidak-ada", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Tak ada desa yang cocok") {
		t.Errorf("kueri tanpa hasil harus menampilkan empty-state pencarian:\n%s", body)
	}
	if strings.Contains(body, "Desa Sukamaju") {
		t.Errorf("kueri tanpa hasil tak boleh memuat desa apa pun")
	}
}

// TestAccountsList_SearchCannotBypassF3: pencarian MENYEMPITKAN, tak pernah
// MEMPERLUAS. Sales hanya melihat desanya; mencari kata yang cocok dengan desa
// milik orang lain tetap nihil — F3 tetap gerbang cakupan, q hanya menyaring di
// dalamnya. Ini pengaman utama slice ini.
func TestAccountsList_SearchCannotBypassF3(t *testing.T) {
	env, uid := setupAccounts(t)
	lain := env.seedMember(t, "lain@x", "member", env.tenantID)
	env.seedAccount(t, "Desa Sales Rahasia", &uid, nil, nil)     // milik aktor
	env.seedAccount(t, "Desa Orang Rahasia", &lain.ID, nil, nil) // milik orang lain

	// Sales mencari "Rahasia" — cocok dengan KEDUA desa secara tekstual, tapi F3
	// hanya membolehkan desa miliknya.
	req := accountsReq(http.MethodGet, "/w/test/accounts?q=Rahasia", nil, "")
	rec := env.runAccount(uid, "member", "sales", req, env.h.AccountsList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Desa Sales Rahasia") {
		t.Errorf("sales harus melihat desa MILIKNYA yang cocok pencarian")
	}
	if strings.Contains(body, "Desa Orang Rahasia") {
		t.Errorf("pencarian tak boleh menembus F3: sales melihat desa milik orang lain")
	}
}
