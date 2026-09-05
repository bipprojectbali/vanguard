package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// kb_articles_status_test.go — siklus status (BL-34) & parse form kategori,
// dipisah dari kb_articles_test.go (ukuran file). Sumbu sama; lihat doc di sana.

// --- status: publish / archive / unarchive / return-to-draft ------------

// kbTransition = satu langkah transisi status untuk tabel siklus (BL-34).
type kbTransition struct {
	path        string
	handler     http.HandlerFunc
	okCode      string
	wantStatus  string
	auditAction string
}

// TestKBArticles_StatusLifecycle: siklus penuh Draft→Published→Archived→Draft
// (unarchive) lalu →Published→Draft (return-to-draft). Membuktikan alur BL-34:
// terbit langsung tanpa gerbang Review, arsip & pulihkan, kembalikan-ke-draf.
// Setiap transisi mendarat status benar, redirect ok=<kode>, & ter-audit.
func TestKBArticles_StatusLifecycle(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedKBArticleRow(t, "Artikel Siklus")

	steps := []kbTransition{
		{"publish", env.h.KBArticlePublish, "published", "Published", "kb_article.publish"},
		{"archive", env.h.KBArticleArchive, "archived", "Archived", "kb_article.archive"},
		{"unarchive", env.h.KBArticleUnarchive, "unarchived", "Draft", "kb_article.unarchive"},
		{"publish", env.h.KBArticlePublish, "published", "Published", "kb_article.publish"},
		{"return-to-draft", env.h.KBArticleReturnToDraft, "returned_to_draft", "Draft", "kb_article.return_to_draft"},
	}
	for _, s := range steps {
		t.Run(s.path+"->"+s.wantStatus, func(t *testing.T) {
			req := accountsReq(http.MethodPost, "/w/test/kb-articles/"+itoa(a.ID)+"/"+s.path, url.Values{}, itoa(a.ID))
			rec := env.runAccount(uid, "owner", "admin", req, s.handler)
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok="+s.okCode) {
				t.Errorf("harus ok=%s, got %q (status %d)", s.okCode, loc, rec.Code)
			}
			if got, _ := env.q.GetKBArticle(t.Context(), a.ID); got.Status != s.wantStatus {
				t.Errorf("harus status=%s, got %q", s.wantStatus, got.Status)
			}
			env.assertAudited(t, s.auditAction)
		})
	}
}

// TestKBArticles_ArchivedHiddenFromDefaultList: artikel yang diarsipkan hilang
// dari daftar Aktif (tab default) tapi muncul di daftar Arsip (BL-34) — bukti
// filter only_archived memisahkan dua tab dengan benar.
func TestKBArticles_ArchivedHiddenFromDefaultList(t *testing.T) {
	env, uid := setupAccounts(t)
	live := env.seedKBArticleRow(t, "Artikel Aktif")
	arch := env.seedKBArticleRow(t, "Artikel Arsip")

	// Arsipkan `arch`: Draft→Published→Archived (arsip hanya dari Published).
	for _, path := range []string{"publish", "archive"} {
		h := env.h.KBArticlePublish
		if path == "archive" {
			h = env.h.KBArticleArchive
		}
		req := accountsReq(http.MethodPost, "/w/test/kb-articles/"+itoa(arch.ID)+"/"+path, url.Values{}, itoa(arch.ID))
		if rec := env.runAccount(uid, "owner", "admin", req, h); rec.Code != http.StatusSeeOther {
			t.Fatalf("%s harus 303, got %d", path, rec.Code)
		}
	}

	active := env.allKBArticles(t)
	if !containsKBArticle(active, live.ID) {
		t.Error("artikel aktif harus tampil di daftar Aktif")
	}
	if containsKBArticle(active, arch.ID) {
		t.Error("artikel diarsipkan TAK boleh tampil di daftar Aktif")
	}

	archived := env.archivedKBArticles(t)
	if !containsKBArticle(archived, arch.ID) {
		t.Error("artikel diarsipkan harus tampil di daftar Arsip")
	}
	if containsKBArticle(archived, live.ID) {
		t.Error("artikel aktif TAK boleh tampil di daftar Arsip")
	}
}

func containsKBArticle(rows []db.KbArticle, id int64) bool {
	for _, r := range rows {
		if r.ID == id {
			return true
		}
	}
	return false
}

// TestKBArticles_StatusGateWrite: transisi status butuh act write — sales
// (read-saja, tanpa write) ditolak 403, status tak berubah.
func TestKBArticles_StatusGateWrite(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedKBArticleRow(t, "Artikel Jaga")

	req := accountsReq(http.MethodPost, "/w/test/kb-articles/"+itoa(a.ID)+"/publish", url.Values{}, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.KBArticlePublish)
	if rec.Code != http.StatusForbidden {
		t.Errorf("sales harus 403 di publish, got %d", rec.Code)
	}
	if got, _ := env.q.GetKBArticle(t.Context(), a.ID); got.Status != "Draft" {
		t.Error("publish yang ditolak tak boleh mengubah status")
	}
}

// --- parse: category enum (BL-35) ---------------------------------------

// TestParseKBArticleForm_Category — category kini enum terkunci (00035),
// opsional-nullable: kosong → NULL (sah), nilai enum → tersimpan, nilai di
// luar enum → galat "category". Uji langsung parser (penjaga sebenarnya).
func TestParseKBArticleForm_Category(t *testing.T) {
	base := map[string]string{"article_title": "Judul", "visibility": "Internal"}
	form := func(overrides map[string]string) func(string) string {
		return func(k string) string {
			if v, ok := overrides[k]; ok {
				return v
			}
			return base[k]
		}
	}

	t.Run("kosong → NULL sah", func(t *testing.T) {
		f, code := parseKBArticleForm(form(map[string]string{"category": ""}))
		if code != "" {
			t.Fatalf("kategori kosong harus sah, got err %q", code)
		}
		if f.Category != nil {
			t.Errorf("kategori kosong harus NULL, got %v", *f.Category)
		}
	})

	t.Run("enum sah tersimpan", func(t *testing.T) {
		f, code := parseKBArticleForm(form(map[string]string{"category": "Teknis"}))
		if code != "" {
			t.Fatalf("kategori enum harus sah, got err %q", code)
		}
		if f.Category == nil || *f.Category != "Teknis" {
			t.Errorf("kategori enum tak tersimpan: %v", f.Category)
		}
	})

	t.Run("di luar enum ditolak", func(t *testing.T) {
		if _, code := parseKBArticleForm(form(map[string]string{"category": "Akun"})); code != "category" {
			t.Errorf("kategori di luar enum harus err=category, got %q", code)
		}
	})
}
