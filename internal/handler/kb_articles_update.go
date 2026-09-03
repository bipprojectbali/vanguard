package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// kb_articles_update.go — aksi UPDATE artikel KB + helper loadKBArticle. Dipisah
// dari kb_articles.go (guard tulis + form baru/sunting + create) agar keduanya di
// bawah ambang tipe Route/Handler (150). Aturan tulis & warisan pemuatan baris sama.
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
	existing, ok := h.loadKBArticle(w, r, id)
	if !ok {
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
		Visibility:   existing.Visibility, // BL-37: read-only → pertahankan nilai lama, jangan balikkan ke default
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
