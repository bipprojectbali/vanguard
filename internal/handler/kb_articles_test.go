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
//   - Aturan tulis: article_title wajib; visibility enum wajib (CHECK DB);
//     category/keywords teks bebas opsional; status hanya lewat
//     submit-review/publish/return-to-draft (tak tersentuh saat update
//     profil, selalu lahir Draft saat create).
//
// TANPA F3/F4/keyset (katalog bounded, tak ada kolom pemilik). Koneksi test
// = superuser (bypass RLS) → uji LOGIKA handler; isolasi RLS diuji di
// rls_test.go. Setup & helper request memakai ulang
// setupAccounts/accountsReq/runAccount. Meniru playbooks_test.go.

// --- helper ----------------------------------------------------------------

// kbArticleFormValues merakit form minimal yang valid untuk create/update.
func kbArticleFormValues(title string) url.Values {
	return url.Values{"article_title": {title}, "visibility": {"Internal"}}
}

// seedKBArticleRow menaruh satu artikel langsung lewat pool (bypass
// handler), status=Draft, visibility=Internal — untuk menguji
// update/transisi status tanpa merangkai create. Mengembalikan db.KbArticle
// utuh.
func (e *testEnv) seedKBArticleRow(t *testing.T, title string) db.KbArticle {
	t.Helper()
	a, err := e.q.CreateKBArticle(t.Context(), db.CreateKBArticleParams{
		TenantID:     e.tenantID,
		ArticleTitle: title,
		Status:       "Draft",
		Visibility:   "Internal",
	})
	if err != nil {
		t.Fatalf("seed kb article %s: %v", title, err)
	}
	return a
}

// allKBArticles mendaftar seluruh katalog langsung dari pool — untuk
// membuktikan sebuah aksi menyimpan / tak menyimpan baris. Superuser (bypass
// RLS) → satu tenant test.
func (e *testEnv) allKBArticles(t *testing.T) []db.KbArticle {
	t.Helper()
	rows, err := e.q.ListKBArticlesAll(t.Context())
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
	form.Set("category", "Akun")
	form.Set("keywords", "password, reset, login")
	form.Set("article_body", "Langkah reset password: buka halaman lupa sandi...")
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
	if a.Visibility != "Public" {
		t.Errorf("visibility salah: %q", a.Visibility)
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
		{"visibility asing", withField(kbArticleFormValues("Artikel X"), "visibility", "Semua Orang"), "err=visibility"},
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
	a := env.seedKBArticleRow(t, "Artikel Lama")

	form := kbArticleFormValues("Artikel Baru")
	form.Set("category", "Billing")
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
	if got.Visibility != "Portal Only" {
		t.Errorf("visibility tak tersimpan: %q", got.Visibility)
	}
	if got.Status != "Draft" {
		t.Error("update profil tak boleh mengubah status")
	}
	env.assertAudited(t, "kb_article.update")
}

// --- status: submit-review / publish / return-to-draft ------------------

// TestKBArticles_StatusLifecycle: Draft→Review (submit-review, ok=submitted)
// →Published (publish, ok=published)→Draft (return-to-draft,
// ok=returned_to_draft). Setiap transisi ter-audit.
func TestKBArticles_StatusLifecycle(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedKBArticleRow(t, "Artikel Siklus")

	req := accountsReq(http.MethodPost, "/w/test/kb-articles/"+itoa(a.ID)+"/submit-review", url.Values{}, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "admin", req, env.h.KBArticleSubmitReview)
	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=submitted") {
		t.Errorf("harus ok=submitted, got %q (status %d)", loc, rec.Code)
	}
	if got, _ := env.q.GetKBArticle(t.Context(), a.ID); got.Status != "Review" {
		t.Errorf("harus status=Review, got %q", got.Status)
	}
	env.assertAudited(t, "kb_article.submit_review")

	req2 := accountsReq(http.MethodPost, "/w/test/kb-articles/"+itoa(a.ID)+"/publish", url.Values{}, itoa(a.ID))
	rec2 := env.runAccount(uid, "owner", "admin", req2, env.h.KBArticlePublish)
	if loc := rec2.Header().Get("Location"); !strings.Contains(loc, "ok=published") {
		t.Errorf("harus ok=published, got %q (status %d)", loc, rec2.Code)
	}
	if got, _ := env.q.GetKBArticle(t.Context(), a.ID); got.Status != "Published" {
		t.Errorf("harus status=Published, got %q", got.Status)
	}
	env.assertAudited(t, "kb_article.publish")

	req3 := accountsReq(http.MethodPost, "/w/test/kb-articles/"+itoa(a.ID)+"/return-to-draft", url.Values{}, itoa(a.ID))
	rec3 := env.runAccount(uid, "owner", "admin", req3, env.h.KBArticleReturnToDraft)
	if loc := rec3.Header().Get("Location"); !strings.Contains(loc, "ok=returned_to_draft") {
		t.Errorf("harus ok=returned_to_draft, got %q (status %d)", loc, rec3.Code)
	}
	if got, _ := env.q.GetKBArticle(t.Context(), a.ID); got.Status != "Draft" {
		t.Errorf("harus status=Draft, got %q", got.Status)
	}
	env.assertAudited(t, "kb_article.return_to_draft")
}

// TestKBArticles_StatusGateWrite: transisi status butuh act write — sales
// (read-saja, tanpa write) ditolak 403, status tak berubah.
func TestKBArticles_StatusGateWrite(t *testing.T) {
	env, uid := setupAccounts(t)
	a := env.seedKBArticleRow(t, "Artikel Jaga")

	req := accountsReq(http.MethodPost, "/w/test/kb-articles/"+itoa(a.ID)+"/submit-review", url.Values{}, itoa(a.ID))
	rec := env.runAccount(uid, "owner", "sales", req, env.h.KBArticleSubmitReview)
	if rec.Code != http.StatusForbidden {
		t.Errorf("sales harus 403 di submit-review, got %d", rec.Code)
	}
	if got, _ := env.q.GetKBArticle(t.Context(), a.ID); got.Status != "Draft" {
		t.Error("submit-review yang ditolak tak boleh mengubah status")
	}
}
