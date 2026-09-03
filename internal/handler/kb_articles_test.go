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
//     category enum wajib bila diisi (CHECK DB 00035, opsional-nullable);
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
		{"visibility asing", withField(kbArticleFormValues("Artikel X"), "visibility", "Semua Orang"), "err=visibility"},
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
	a := env.seedKBArticleRow(t, "Artikel Lama")

	form := kbArticleFormValues("Artikel Baru")
	form.Set("category", "Pembayaran")
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
	if got.Category == nil || *got.Category != "Pembayaran" {
		t.Errorf("category enum tak tersimpan: %v", got.Category)
	}
	if got.Status != "Draft" {
		t.Error("update profil tak boleh mengubah status")
	}
	env.assertAudited(t, "kb_article.update")
}

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
