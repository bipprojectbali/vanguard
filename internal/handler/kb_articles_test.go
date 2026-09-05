package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// kb_articles_test.go — katalog Knowledge Base (Modul 6 slice A3) di sisi
// handler. Sama seperti Playbooks (slice A2), master milik WORKSPACE →
// hanya DUA sumbu yang dijaga:
//
//   - F2 (Casbin bisnis): GET butuh crm:kb read — admin/manager/support
//     (write mencakup read) DAN sales/csm (read-saja, BEDA dari Playbooks
//     tempat sales/support tanpa objek sama sekali) lolos; hanya "" ditolak.
//     POST butuh crm:kb write — admin/manager/support; sales & csm
//     (read-saja) & "" ditolak 403.
//   - Aturan tulis: article_title wajib; visibility read-only (BL-37 —
//     dikunci "Internal" saat create, dipertahankan saat update, TAK dibaca
//     form); category enum wajib bila diisi (CHECK DB 00035, opsional-nullable);
//     keywords teks bebas opsional; status hanya lewat
//     publish/return-to-draft/archive/unarchive (BL-34, tak tersentuh saat
//     update profil, selalu lahir Draft saat create).
//
// TANPA F3/F4/keyset (katalog bounded, tak ada kolom pemilik). Koneksi test
// = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS diuji di
// rls_test.go. Setup & helper request memakai ulang
// setupAccounts/accountsReq/runAccount. Meniru playbooks_test.go.

// --- helper ----------------------------------------------------------------

// kbArticleFormValues merakit form minimal yang valid untuk create/update.
// BL-37: visibility TAK lagi dikirim form (field read-only) → form minimal
// cukup judul; handler menetapkan visibility sendiri.
func kbArticleFormValues(title string) url.Values {
	return url.Values{"article_title": {title}}
}

// seedKBArticleRow menaruh satu artikel langsung lewat pool (bypass
// handler), status=Draft, visibility=Internal — untuk menguji
// update/transisi status tanpa merangkai create. Mengembalikan db.KbArticle
// utuh.
func (e *testEnv) seedKBArticleRow(t *testing.T, title string) db.KbArticle {
	t.Helper()
	return e.seedKBArticleRowVis(t, title, "Internal")
}

// seedKBArticleRowVis = seedKBArticleRow dengan visibility eksplisit — untuk
// membuktikan update MEMPERTAHANKAN nilai lama (BL-37), bukan membalikkannya
// ke default.
func (e *testEnv) seedKBArticleRowVis(t *testing.T, title, vis string) db.KbArticle {
	t.Helper()
	a, err := e.q.CreateKBArticle(t.Context(), db.CreateKBArticleParams{
		TenantID:     e.tenantID,
		ArticleTitle: title,
		Status:       "Draft",
		Visibility:   vis,
	})
	if err != nil {
		t.Fatalf("seed kb article %s: %v", title, err)
	}
	return a
}

// allKBArticles mendaftar katalog AKTIF (non-Archived) langsung dari pool —
// cermin tab default. Untuk membuktikan sebuah aksi menyimpan / tak menyimpan
// baris. Superuser (bypass RLS) → satu tenant test.
func (e *testEnv) allKBArticles(t *testing.T) []db.KbArticle {
	t.Helper()
	return e.listKBArticles(t, false)
}

// archivedKBArticles mendaftar HANYA katalog Archived (cermin tab Arsip,
// BL-34) — untuk membuktikan artikel diarsipkan hilang dari tab Aktif tapi
// muncul di tab Arsip.
func (e *testEnv) archivedKBArticles(t *testing.T) []db.KbArticle {
	t.Helper()
	return e.listKBArticles(t, true)
}

func (e *testEnv) listKBArticles(t *testing.T, onlyArchived bool) []db.KbArticle {
	t.Helper()
	cAt, cID := firstPageCursor()
	rows, err := e.q.ListKBArticlesAll(t.Context(), db.ListKBArticlesAllParams{
		CursorCreatedAt: cAt, CursorID: cID, OnlyArchived: onlyArchived, PageSize: allCatalogPageSize,
	})
	if err != nil {
		t.Fatalf("list kb articles: %v", err)
	}
	return rows
}

// --- F2: gerbang read --------------------------------------------------

// TestKBArticles_GateRead: siapa boleh MEMBUKA katalog (act read).
// admin/manager/support (write mencakup read) DAN sales/csm (read-saja)
// semua lolos — hanya "" ditolak 403 dengan penjelasan butuh peran CRM.
func TestKBArticles_GateRead(t *testing.T) {
	env, uid := setupAccounts(t)
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"support", true},
		{"sales", true},
		{"csm", true},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			req := accountsReq(http.MethodGet, "/w/test/kb-articles", nil, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.KBArticlesList)
			if c.allow && rec.Code != http.StatusOK {
				t.Errorf("role %q harus lolos gate read, got %d\n%s", c.role, rec.Code, rec.Body.String())
			}
			if !c.allow {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if !strings.Contains(rec.Body.String(), "peran CRM") {
					t.Errorf("penolakan harus menjelaskan butuh peran CRM")
				}
			}
		})
	}
}

// TestKBArticles_GateWrite: POST create butuh act write — admin/manager/
// support; sales & csm (read-saja) & "" ditolak 403 tanpa menyentuh DB.
func TestKBArticles_GateWrite(t *testing.T) {
	cases := []struct {
		role  string
		allow bool
	}{
		{"admin", true},
		{"manager", true},
		{"support", true},
		{"sales", false},
		{"csm", false},
		{"", false},
	}
	for _, c := range cases {
		t.Run("role="+c.role, func(t *testing.T) {
			env, uid := setupAccounts(t)
			form := kbArticleFormValues("Artikel Standar")
			req := accountsReq(http.MethodPost, "/w/test/kb-articles", form, "")
			rec := env.runAccount(uid, "owner", c.role, req, env.h.KBArticleCreate)

			rows := env.allKBArticles(t)
			if c.allow {
				if rec.Code != http.StatusSeeOther {
					t.Errorf("role %q harus create (303), got %d\n%s", c.role, rec.Code, rec.Body.String())
				}
				if len(rows) != 1 {
					t.Errorf("role %q create harus menyimpan 1 baris, ada %d", c.role, len(rows))
				}
			} else {
				if rec.Code != http.StatusForbidden {
					t.Errorf("role %q harus 403, got %d", c.role, rec.Code)
				}
				if len(rows) != 0 {
					t.Errorf("role %q ditolak tak boleh menyimpan apa pun, ada %d baris", c.role, len(rows))
				}
			}
		})
	}
}

// --- create ------------------------------------------------------------

// TestKBArticles_CreateSuccess: support create → 303 ok=created; baris
// tersimpan status=Draft, author_id & created_by = pembuat; audit
// kb_article.create tercatat.
func TestKBArticles_CreateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	form := kbArticleFormValues("Cara Reset Password")
	form.Set("category", "Kependudukan")
	form.Set("keywords", "password, reset, login")
	form.Set("article_body", "Langkah reset password: buka halaman lupa sandi...")
	// BL-37: walau form nekat mengirim visibility (mis. request buatan tangan),
	// handler MENGABAIKANNYA & tetap menyimpan "Internal".
	form.Set("visibility", "Public")
	req := accountsReq(http.MethodPost, "/w/test/kb-articles", form, "")
	rec := env.runAccount(uid, "owner", "support", req, env.h.KBArticleCreate)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303; body:\n%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=created") {
		t.Errorf("redirect harus ok=created, got %q", loc)
	}
	rows := env.allKBArticles(t)
	if len(rows) != 1 {
		t.Fatalf("harus 1 baris, ada %d", len(rows))
	}
	a := rows[0]
	if a.ArticleTitle != "Cara Reset Password" {
		t.Errorf("judul salah: %q", a.ArticleTitle)
	}
	if a.Status != "Draft" {
		t.Errorf("artikel baru harus lahir Draft, got %q", a.Status)
	}
	if a.Visibility != "Internal" {
		t.Errorf("BL-37: create harus mengunci visibility=Internal (abaikan form), got %q", a.Visibility)
	}
	if a.Category == nil || *a.Category != "Kependudukan" {
		t.Errorf("category enum tak tersimpan: %v", a.Category)
	}
	if a.AuthorID == nil || *a.AuthorID != uid {
		t.Errorf("author_id harus pembuat, got %v", a.AuthorID)
	}
	if a.CreatedBy == nil || *a.CreatedBy != uid {
		t.Errorf("created_by harus pembuat, got %v", a.CreatedBy)
	}
	env.assertAudited(t, "kb_article.create")
}

// TestKBArticles_CreateRejectsInvalid: input yang melanggar validasi
// backend ditolak → redirect err + tak menyentuh DB.
func TestKBArticles_CreateRejectsInvalid(t *testing.T) {
	cases := []struct {
		name    string
		form    url.Values
		wantErr string
	}{
		{"judul kosong", kbArticleFormValues(""), "err=required"},
		{"category asing", withField(kbArticleFormValues("Artikel X"), "category", "Akun"), "err=category"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env, uid := setupAccounts(t)
			req := accountsReq(http.MethodPost, "/w/test/kb-articles", c.form, "")
			rec := env.runAccount(uid, "owner", "admin", req, env.h.KBArticleCreate)
			if loc := rec.Header().Get("Location"); !strings.Contains(loc, c.wantErr) {
				t.Errorf("harus %q, got %q", c.wantErr, loc)
			}
			if rows := env.allKBArticles(t); len(rows) != 0 {
				t.Errorf("input invalid tak boleh menyimpan, ada %d baris", len(rows))
			}
		})
	}
}

// --- update --------------------------------------------------------------

// TestKBArticles_UpdateSuccess: admin update → ok=saved, profil tersimpan,
// status dipertahankan (bukan efek samping edit), audit kb_article.update
// tercatat.
func TestKBArticles_UpdateSuccess(t *testing.T) {
	env, uid := setupAccounts(t)
	// Seed ber-visibility non-default (Public) untuk membuktikan BL-37:
	// update MEMPERTAHANKAN nilai lama, tak membalikkannya ke Internal.
	a := env.seedKBArticleRowVis(t, "Artikel Lama", "Public")

	form := kbArticleFormValues("Artikel Baru")
	form.Set("category", "Pembayaran")
	// Form nekat mengirim visibility berbeda → harus DIABAIKAN (field read-only).
	form.Set("visibility", "Portal Only")
	req := accountsReq(http.MethodPost, "/w/test/kb-articles/"+itoa(a.ID), form, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.KBArticleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Errorf("harus ok=saved, got %q (status %d)", loc, rec.Code)
	}
	got, _ := env.q.GetKBArticle(t.Context(), a.ID)
	if got.ArticleTitle != "Artikel Baru" {
		t.Errorf("judul tak tersimpan: %q", got.ArticleTitle)
	}
	if got.Visibility != "Public" {
		t.Errorf("BL-37: update harus mempertahankan visibility lama (Public), got %q", got.Visibility)
	}
	if got.Category == nil || *got.Category != "Pembayaran" {
		t.Errorf("category enum tak tersimpan: %v", got.Category)
	}
	if got.Status != "Draft" {
		t.Error("update profil tak boleh mengubah status")
	}
	env.assertAudited(t, "kb_article.update")
}
