package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// kb_articles_page.go — HALAMAN baca katalog Knowledge Base (Modul 6 slice
// A3). Aksi ada di kb_articles.go / kb_articles_status.go. Dipisah karena
// halaman tumbuh dengan aturan LIHAT (F2 read), aksi dengan aturan TULIS
// (F2 write). Meniru playbooks_page.go (slice A2).
//
// Katalog master per-workspace (ListKBArticlesAll, TERMASUK draf) → keyset
// (created_at DESC, id DESC) lewat ?after=, TANPA F3: RLS satu-satunya
// pengurung. Urutan pindah dari updated_at ke created_at (kursor keyset STABIL).
//
// KPI cards wireframe 6.10 ("Artikel Terbit"/"Dilihat 30 Hr"/"Tiket
// Ter-deflect"/"Perlu Review") & filter tab (Semua/Terpopuler/Perlu
// Review/Draf) DIHILANGKAN dari slice ini SENGAJA — meniru keputusan skop
// playbooks_page.go (A2): "Dilihat 30 Hr" & "Tiket Ter-deflect" butuh
// tabel log tayang/deflect yang belum ada; "Perlu Review" versi wireframe
// (usang/rendah rating) butuh ambang bisnis yang belum didefinisikan;
// katalog master ini bounded & kecil, filter tab menambah kompleksitas
// tanpa data eksekusi untuk membuatnya berguna. Kolom "Rating" wireframe
// ("Membantu 94%") butuh total-suara yang TAK ADA di skema (hanya
// helpful_votes) — ditampilkan sebagai hitungan suara mentah, bukan
// persentase fabrikasi.

// KBArticlesList — GET /w/{workspace}/kb-articles. Seluruh katalog (semua
// status, terbaru diperbarui dulu). Bukan pemegang peran CRM (read) → 403 +
// penjelasan.
func (h *Handler) KBArticlesList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewKBArticles(ctx) {
		h.renderKBArticlesForbidden(w, r)
		return
	}
	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListKBArticlesAll(ctx, db.ListKBArticlesAllParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("kb_articles: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	shown, nextCursor := splitPage(rows, func(a db.KbArticle) (pgtype.Timestamptz, int64) {
		return a.CreatedAt, a.ID
	})
	items := make([]panel.KBArticleRow, 0, len(shown))
	for _, a := range shown {
		items = append(items, kbArticleRowView(a))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Knowledge Base", "/kb-articles", panel.KBArticleList(panel.KBArticleListView{
		Base:       base,
		CanWrite:   canWriteKBArticles(ctx),
		Err:        kbArticlesErrMsg(r.URL.Query().Get("err")),
		Msg:        kbArticlesMsg(r.URL.Query().Get("ok")),
		Items:      items,
		NextCursor: nextCursor,
	}))
}

// kbArticleRowView memetakan satu artikel → baris tabel katalog.
func kbArticleRowView(a db.KbArticle) panel.KBArticleRow {
	return panel.KBArticleRow{
		ID:           a.ID,
		ArticleTitle: a.ArticleTitle,
		Category:     deref(a.Category),
		ViewCount:    int(a.ViewCount),
		HelpfulVotes: int(a.HelpfulVotes),
		Status:       a.Status,
		UpdatedAt:    fmtDateTime(a.UpdatedAt),
	}
}
