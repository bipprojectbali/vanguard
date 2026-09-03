package handler

import (
	"net/http"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// kb_articles_search_test.go — regresi BL-36: pencarian daftar Knowledge Base
// (?q=), MENGAKTIFKAN `keywords` yang semula write-only. Kembaran
// subscriptions_search_test.go untuk katalog KB. Sifat yang WAJIB benar di sisi
// handler:
//   - MENYEMPITKAN: ?q= menyaring baris via ILIKE contains (case-insensitive)
//     pada JUDUL, KATA KUNCI, atau KATEGORI — bukan pada isi artikel
//     (article_body sengaja di luar kunci cari).
//   - keywords sebagai kunci cari: artikel ketemu lewat kata kunci meski kata itu
//     TAK muncul di judul (findability tiket → artikel, tujuan asli field).
//   - q kosong → tak menyaring (seluruh katalog aktif tampil).
//
// Isolasi tenant TIDAK diuji di sini: harness handler menyuntik querier
// superuser (bypass RLS), jadi cross-tenant tak bermakna di lapisan ini —
// isolasi RLS diuji di rls_test.go. Search hanya meng-AND predikat penyempit di
// atas query ber-scope RLS; ia tak pernah melepas pengurung tenant.
// (Kontrak URL kotak cari/pager diuji di view: panel/kb_articles_search_test.go.)

// seedKBArticleFull menaruh satu artikel dengan kategori + kata kunci (di luar
// jangkauan seedKBArticleRow yang hanya judul) — untuk menguji ketiga kunci
// cari. status=Published agar tampil di tab Aktif (default).
func (e *testEnv) seedKBArticleFull(t *testing.T, title, category, keywords string) db.KbArticle {
	t.Helper()
	a, err := e.q.CreateKBArticle(t.Context(), db.CreateKBArticleParams{
		TenantID:     e.tenantID,
		ArticleTitle: title,
		Category:     ptr(category),
		Keywords:     ptr(keywords),
		Status:       "Published",
		Visibility:   "Internal",
	})
	if err != nil {
		t.Fatalf("seed kb article %s: %v", title, err)
	}
	return a
}

func TestKBArticlesList_SearchByTitle(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedKBArticleFull(t, "Cara Reset Password", "Teknis", "sandi")
	env.seedKBArticleFull(t, "Panduan Pembayaran PBB", "Pembayaran", "pajak")

	req := accountsReq(http.MethodGet, "/w/test/kb-articles?q=reset", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.KBArticlesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Cara Reset Password") {
		t.Errorf("q=reset harus memuat artikel yang judulnya cocok (case-insensitive)")
	}
	if strings.Contains(body, "Panduan Pembayaran PBB") {
		t.Errorf("q=reset tak boleh memuat artikel yang tak cocok")
	}
}

// TestKBArticlesList_SearchByKeywords — artikel ketemu lewat KATA KUNCI walau
// kata itu tak ada di judul. Inti BL-36: mengaktifkan field keywords yang mati.
func TestKBArticlesList_SearchByKeywords(t *testing.T) {
	env, uid := setupAccounts(t)
	// "kartu keluarga" hanya di keywords, TIDAK di judul.
	env.seedKBArticleFull(t, "Dokumen Kependudukan", "Kependudukan", "KK, kartu keluarga, NIK")
	env.seedKBArticleFull(t, "Tagihan Air", "Pembayaran", "PDAM, meteran")

	req := accountsReq(http.MethodGet, "/w/test/kb-articles?q=kartu+keluarga", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.KBArticlesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Dokumen Kependudukan") {
		t.Errorf("q=kartu keluarga harus menemukan artikel lewat kata kunci (bukan judul)")
	}
	if strings.Contains(body, "Tagihan Air") {
		t.Errorf("q=kartu keluarga tak boleh memuat artikel dengan kata kunci lain")
	}
}

func TestKBArticlesList_SearchByCategory(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedKBArticleFull(t, "Langkah Awal Aplikasi", "Panduan Awal", "mulai")
	env.seedKBArticleFull(t, "Konfigurasi Server", "Teknis", "deploy")

	req := accountsReq(http.MethodGet, "/w/test/kb-articles?q=teknis", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.KBArticlesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Konfigurasi Server") {
		t.Errorf("q=teknis harus memuat artikel berkategori Teknis (case-insensitive)")
	}
	if strings.Contains(body, "Langkah Awal Aplikasi") {
		t.Errorf("q=teknis tak boleh memuat artikel berkategori lain")
	}
}

func TestKBArticlesList_SearchNoMatchEmpty(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedKBArticleFull(t, "Cara Reset Password", "Teknis", "sandi")

	req := accountsReq(http.MethodGet, "/w/test/kb-articles?q=zzz-tak-ada", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.KBArticlesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "Cara Reset Password") {
		t.Errorf("kueri tanpa hasil tak boleh memuat artikel apa pun")
	}
	if !strings.Contains(body, "Belum ada artikel yang cocok pencarian") {
		t.Errorf("kueri tanpa hasil harus empty-state pencarian:\n%s", body)
	}
}

// TestKBArticlesList_EmptyQueryReturnsAll — q kosong = tak menyaring: seluruh
// katalog aktif tampil (search berdiri sendiri, tak menghapus baris tanpa q).
func TestKBArticlesList_EmptyQueryReturnsAll(t *testing.T) {
	env, uid := setupAccounts(t)
	env.seedKBArticleFull(t, "Artikel Satu", "Teknis", "a")
	env.seedKBArticleFull(t, "Artikel Dua", "Umum", "b")

	req := accountsReq(http.MethodGet, "/w/test/kb-articles", nil, "")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.KBArticlesList)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d\n%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"Artikel Satu", "Artikel Dua"} {
		if !strings.Contains(body, want) {
			t.Errorf("q kosong harus menampilkan seluruh katalog; %q hilang", want)
		}
	}
}
