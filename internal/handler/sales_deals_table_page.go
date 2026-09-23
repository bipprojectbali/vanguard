package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// sales_deals_table_page.go — tampilan Tabel berkeyset daftar Deal (dealsTable),
// dipisah dari sales_deals_page.go (pipeline/Kanban + KPI) demi ambang tipe
// Route/Handler (150). Package sama; sumber ownership & F4 identik.
//
// Fungsi sort per-kolom (name/stage/code di sort_a.go, amount/probability di
// sort_b.go, close/owner/default di sort_c.go) dipisah krn ambang File Health
// yang sama.

// dealsTable merender tampilan Tabel berkeyset (alternatif papan). Sama sumber
// ownership; menampilkan lebih banyak kolom & navigasi halaman berikutnya.
func (h *Handler) dealsTable(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	filter := db.DealsListFilterFor(session.BusinessDataScope(ctx))
	uid := session.UserID(ctx)
	canARR := canSeeARR(ctx)

	// q = pencarian bebas (BL-6): MEMPERSEMPIT di atas F3+stage, tak melebarkan.
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	mineOnly := dealMineOnly(r)
	stageFilter := r.URL.Query().Get("stage")

	// sort/dir (BL-157e): whitelist 7 kolom sortable (dealSortableColumns).
	// Kombinasi tak dikenal → jatuh ke jalur default (created_at DESC), TAK
	// error — URL lama/disunting tetap render halaman yang benar.
	sortCol := r.URL.Query().Get("sort")
	if !dealSortableColumns[sortCol] {
		sortCol = ""
	}
	dir := r.URL.Query().Get("dir")
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}

	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("deals: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var shown []db.Deal
	var nextCursor string
	var ok bool
	switch sortCol {
	case "name":
		shown, nextCursor, ok = h.dealsSortByName(w, r, ctx, filter, uid, mineOnly, stageFilter, query, dir, names)
	case "stage":
		shown, nextCursor, ok = h.dealsSortByStage(w, r, ctx, filter, uid, mineOnly, stageFilter, query, dir, names)
	case "code":
		shown, nextCursor, ok = h.dealsSortByCode(w, r, ctx, filter, uid, mineOnly, stageFilter, query, dir, names)
	case "amount":
		shown, nextCursor, ok = h.dealsSortByAmount(w, r, ctx, filter, uid, mineOnly, stageFilter, query, dir, names)
	case "probability":
		shown, nextCursor, ok = h.dealsSortByProbability(w, r, ctx, filter, uid, mineOnly, stageFilter, query, dir, names)
	case "close":
		shown, nextCursor, ok = h.dealsSortByCloseDate(w, r, ctx, filter, uid, mineOnly, stageFilter, query, dir, names)
	case "owner":
		shown, nextCursor, ok = h.dealsSortByOwner(w, r, ctx, filter, uid, mineOnly, stageFilter, query, dir, names)
	default:
		shown, nextCursor, ok = h.dealsSortDefault(w, r, ctx, filter, uid, mineOnly, stageFilter, query, names)
	}
	if !ok {
		return
	}

	items := make([]panel.DealRow, 0, len(shown))
	for _, d := range shown {
		items = append(items, dealRowView(d, names, canARR))
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
		StageFilter:    stageFilter,
		Query:          query,
		Items:          items,
		NextCursor:     nextCursor,
		After:          r.URL.Query().Get("after"),
		Trail:          pageTrail(r),
		Sort:           sortCol,
		Dir:            dir,
	}))
}

// dealSortableColumns = whitelist kolom yang boleh diminta lewat ?sort= (BL-157e:
// 7 kolom tabel Deals). ?sort= di luar daftar ini diperlakukan seolah absen
// (jatuh ke default created_at DESC), TAK error.
var dealSortableColumns = map[string]bool{
	"code":        true,
	"name":        true,
	"stage":       true,
	"amount":      true,
	"probability": true,
	"close":       true,
	"owner":       true,
}
