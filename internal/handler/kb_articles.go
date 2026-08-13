package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// kb_articles.go — AKSI atas katalog Knowledge Base: form buat/sunting,
// create, update. Transisi status (submit-review/publish/return-to-draft)
// di kb_articles_status.go; halaman baca di kb_articles_page.go. Dipisah
// karena aksi tumbuh dengan aturan TULIS (F2 write = admin/manager/support),
// halaman dengan aturan LIHAT (F2 read). Meniru playbooks.go (slice A2).
//
// Katalog master milik WORKSPACE (bukan per-desa) → TANPA F3 ownership: RLS
// (h.q ber-tenant) satu-satunya pengurung. kb_articles TANPA entity_code &
// TANPA soft-delete (status='Draft' = draf, bukan terhapus). TANPA unique
// constraint selain PK → tak ada writeErr khusus.

// requireKBArticleWrite = gerbang tulis bersama. false & menulis penolakan
// bila aktor tak berhak (izin F2 write; read-only workspace ditolak lebih
// awal dgn pesan jelas).
func (h *Handler) requireKBArticleWrite(w http.ResponseWriter, r *http.Request) bool {
	if !canWriteKBArticlesPerm(r.Context()) {
		h.renderKBArticlesForbidden(w, r)
		return false
	}
	return true
}

// KBArticleNew — GET /w/{workspace}/kb-articles/new. Form kosong untuk
// artikel baru. status TAK di form — artikel baru selalu lahir Draft.
func (h *Handler) KBArticleNew(w http.ResponseWriter, r *http.Request) {
	if !h.requireKBArticleWrite(w, r) {
		return
	}
	base := wsPath(slugFromRequest(r), "")
	v := panel.KBArticleFormView{
		Base:              base,
		Action:            base + "/kb-articles",
		IsEdit:            false,
		Err:               kbArticlesErrMsg(r.URL.Query().Get("err")),
		Fields:            panel.KBArticleFormFields{Visibility: "Internal"},
		VisibilityOptions: kbArticleVisibilityOptions,
	}
	h.renderWorkspaceShell(w, r, "Tambah Artikel", "/kb-articles", panel.KBArticleForm(v))
}

// KBArticleCreate — POST /w/{workspace}/kb-articles. Membuat artikel
// katalog. Lahir status=Draft. author_id = created_by (lihat komentar
// migrasi 00018). tenant_id dari sesi (RLS WITH CHECK memverifikasinya =
// GUC).
func (h *Handler) KBArticleCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requireKBArticleWrite(w, r) {
		return
	}
	ctx := r.Context()
	form, errCode := parseKBArticleForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/kb-articles/new", errCode)
		return
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	a, err := h.q(ctx).CreateKBArticle(ctx, db.CreateKBArticleParams{
		TenantID:     tenantID,
		ArticleTitle: form.ArticleTitle,
		ArticleBody:  form.ArticleBody,
		Category:     form.Category,
		Keywords:     form.Keywords,
		Status:       "Draft", // artikel baru selalu lahir draf
		Visibility:   form.Visibility,
		AuthorID:     &uid,
		CreatedBy:    &uid,
	})
	if err != nil {
		h.Log.Error("kb_articles: create", "err", err)
		wsRedirect(w, r, "/kb-articles/new", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "kb_article.create", tenantID, map[string]string{
		"kb_article_id": strconv.FormatInt(a.ID, 10),
	})
	wsRedirectOK(w, r, "/kb-articles", "created")
}

// KBArticleEdit — GET /w/{workspace}/kb-articles/{id}/edit. Form terisi
// profil/isi artikel (bukan status — transisi jalur tersendiri).
func (h *Handler) KBArticleEdit(w http.ResponseWriter, r *http.Request) {
	if !h.requireKBArticleWrite(w, r) {
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	a, ok := h.loadKBArticle(w, r, id)
	if !ok {
		return
	}

	base := wsPath(slugFromRequest(r), "")
	v := panel.KBArticleFormView{
		Base:              base,
		Action:            base + "/kb-articles/" + strconv.FormatInt(a.ID, 10),
		IsEdit:            true,
		Err:               kbArticlesErrMsg(r.URL.Query().Get("err")),
		Fields:            kbArticleFormFields(a),
		VisibilityOptions: kbArticleVisibilityOptions,
	}
	h.renderWorkspaceShell(w, r, "Sunting Artikel", "/kb-articles", panel.KBArticleForm(v))
}

// KBArticleUpdate — POST /w/{workspace}/kb-articles/{id}. Menyimpan sunting
// PROFIL/ISI. status TAK disentuh (transisi jalur tersendiri) → tahap
// editorial dipertahankan.
func (h *Handler) KBArticleUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.requireKBArticleWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadKBArticle(w, r, id); !ok {
		return
	}

	form, errCode := parseKBArticleForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/kb-articles/"+strconv.FormatInt(id, 10)+"/edit", errCode)
		return
	}

	uid := session.UserID(ctx)
	if _, err := h.q(ctx).UpdateKBArticle(ctx, db.UpdateKBArticleParams{
		ArticleTitle: form.ArticleTitle,
		ArticleBody:  form.ArticleBody,
		Category:     form.Category,
		Keywords:     form.Keywords,
		Visibility:   form.Visibility,
		UpdatedBy:    &uid,
		ID:           id,
	}); err != nil {
		h.Log.Error("kb_articles: update", "err", err)
		wsRedirect(w, r, "/kb-articles/"+strconv.FormatInt(id, 10)+"/edit", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "kb_article.update", session.TenantID(ctx), map[string]string{
		"kb_article_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/kb-articles", "saved")
}

// loadKBArticle memuat satu artikel katalog. kb_articles TANPA F3 (milik
// workspace) → cukup RLS. Tak ada → 404. Termasuk artikel draf agar bisa
// disunting/ditransisikan. Mengembalikan (artikel, true) atau menulis
// 404/500 & (zero, false).
func (h *Handler) loadKBArticle(w http.ResponseWriter, r *http.Request, id int64) (db.KbArticle, bool) {
	a, err := h.q(r.Context()).GetKBArticle(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.KbArticle{}, false
		}
		h.Log.Error("kb_articles: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.KbArticle{}, false
	}
	return a, true
}
