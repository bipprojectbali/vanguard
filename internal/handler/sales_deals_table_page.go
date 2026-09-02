package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_deals_table_page.go — tampilan Tabel berkeyset daftar Deal (dealsTable),
// dipisah dari sales_deals_page.go (pipeline/Kanban + KPI) demi ambang tipe
// Route/Handler (150). Package sama; sumber ownership & F4 identik.

// dealsTable merender tampilan Tabel berkeyset (alternatif papan). Sama sumber
// ownership; menampilkan lebih banyak kolom & navigasi halaman berikutnya.
func (h *Handler) dealsTable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	filter := db.DealsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	br := session.BusinessRole(ctx)

	// q = pencarian bebas (BL-6): MEMPERSEMPIT di atas F3+stage, tak melebarkan.
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	mineOnly := dealMineOnly(r)
	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListDeals(ctx, db.ListDealsParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		MineOnly:        mineOnly,
		StageFilter:     r.URL.Query().Get("stage"),
		Search:          query,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("deals: table", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	shown, nextCursor := splitPage(rows, func(d db.Deal) (pgtype.Timestamptz, int64) {
		return d.CreatedAt, d.ID
	})
	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("deals: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	items := make([]panel.DealRow, 0, len(shown))
	for _, d := range shown {
		items = append(items, dealRowView(d, names, br))
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Deals", "/deals", panel.DealPipeline(panel.DealPipelineView{
		Base:           base,
		View:           "table",
		CanWrite:       canWriteDeals(ctx),
		Mine:           mineOnly,
		ShowMineToggle: filter.ScopeAll, // BL-10: 'own' sudah milik sendiri → redundan
		Err:            wsErrMsg(r.URL.Query().Get("err")),
		Msg:            dealsMsg(r.URL.Query().Get("ok")),
		StageFilter:    r.URL.Query().Get("stage"),
		Query:          query,
		Items:          items,
		NextCursor:     nextCursor,
		After:          r.URL.Query().Get("after"),
		Trail:          pageTrail(r),
	}))
}
