package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// kb_articles_status.go — AKSI transisi status katalog: ajukan review /
// terbitkan / kembalikan ke draf. Dipisah dari sunting profil
// (kb_articles.go) karena status adalah aksi bisnis tersendiri
// (SetKBArticleStatus), bukan efek samping edit — mengedit isi artikel
// tak boleh diam-diam mengubah tahap editorialnya. Meniru
// playbooks_status.go (slice A2). Gerbang = requireKBArticleWrite.
//
// Alur: Draft --ajukan review--> Review --terbitkan--> Published. Dari
// Review atau Published bisa "kembalikan ke draf" (mis. artikel usang perlu
// disunting ulang sebelum tampil lagi). Tombol yang ditawarkan per baris
// ditentukan VIEW dari status saat ini (lihat kb_articles.go panel) — jalur
// handler ini tak menolak transisi "tak wajar" (mis. Draft→Published
// langsung) karena CHECK di DB hanya menjaga DOMAIN nilai, bukan urutan;
// urutan cukup dijaga di UI (tak ada risiko keamanan, hanya alur kerja).

// KBArticleSubmitReview — POST /w/{workspace}/kb-articles/{id}/submit-review.
// Ajukan artikel draf untuk direview.
func (h *Handler) KBArticleSubmitReview(w http.ResponseWriter, r *http.Request) {
	h.setKBArticleStatus(w, r, "Review", "kb_article.submit_review", "submitted")
}

// KBArticlePublish — POST /w/{workspace}/kb-articles/{id}/publish. Terbitkan
// artikel yang sudah direview.
func (h *Handler) KBArticlePublish(w http.ResponseWriter, r *http.Request) {
	h.setKBArticleStatus(w, r, "Published", "kb_article.publish", "published")
}

// KBArticleReturnToDraft — POST /w/{workspace}/kb-articles/{id}/return-to-draft.
// Kembalikan artikel (review atau terbit) ke draf untuk disunting ulang.
func (h *Handler) KBArticleReturnToDraft(w http.ResponseWriter, r *http.Request) {
	h.setKBArticleStatus(w, r, "Draft", "kb_article.return_to_draft", "returned_to_draft")
}

// setKBArticleStatus = jalur bersama transisi status: gate tulis → parse id
// → muat (menegakkan keberadaan/RLS) → SetKBArticleStatus → audit → 303.
// target/auditAction/okCode dioper pemanggil agar jejak & pesan spesifik
// per aksi.
func (h *Handler) setKBArticleStatus(w http.ResponseWriter, r *http.Request, target, auditAction, okCode string) {
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

	uid := session.UserID(ctx)
	if err := h.q(ctx).SetKBArticleStatus(ctx, db.SetKBArticleStatusParams{
		Status:    target,
		UpdatedBy: &uid,
		ID:        id,
	}); err != nil {
		h.Log.Error("kb_articles: set status", "err", err, "target", target)
		wsRedirect(w, r, "/kb-articles", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, auditAction, session.TenantID(ctx), map[string]string{
		"kb_article_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/kb-articles", okCode)
}
