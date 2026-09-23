package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// HealthScoreList — GET /health-scores.
// Workspace-level listing health score semua desa dalam scope role.
// Gate: crm:health read (canViewHealthScore). F3 ownership via flag boolean.
//
// healthScoreSortableColumns/healthSegment/healthTabToStatus di
// health_score_view.go; fungsi sort per-kolom di health_score_sort_a.go,
// health_score_sort_b.go & health_score_sort_c.go (dipecah krn ambang File
// Health).
func (h *Handler) HealthScoreList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !canViewHealthScore(ctx) {
		h.renderHealthScoreForbidden(w, r)
		return
	}

	slug := slugFromRequest(r)

	// Filter ownership — pola AccountsListFilter.
	lp := healthScoreListParams(ctx)
	lp.CursorCreatedAt, lp.CursorID = pageCursor(r)
	lp.PageSize = pageSize + 1

	// Filter status dari tab (?tab=sehat|berisiko|kritis). filterStatus disimpan
	// terpisah (string) karena lp.FilterStatus di-generate sqlc sbg interface{}
	// (query dasar ListHealthScores tanpa cast ::text) — 7 query sort BARU
	// (BL-157i) tulis predikat DENGAN ::text sehingga field Params-nya string.
	tab := r.URL.Query().Get("tab")
	filterStatus := healthTabToStatus(tab)
	lp.FilterStatus = filterStatus

	// BL-114: segmen populasi (?segment=active|churned). Default "active" = desa
	// pelanggan berlangganan hidup; "churned" = eks-pelanggan (punya langganan,
	// tak ada yang hidup). Prospek tanpa langganan tak muncul di segmen mana pun.
	segment := healthSegment(r.URL.Query().Get("segment"))
	lp.Segment = segment

	// Pencarian bebas (BL-6) — menyaring pada nama desa yang tampil.
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	lp.Search = query

	kpis, err := h.q(ctx).CountHealthScoreKPIs(ctx, healthScoreKPIParams(lp))
	if err != nil {
		h.Log.Error("health-scores: count kpis", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	uid := session.UserID(ctx)

	// sort/dir (BL-157i): whitelist 7 kolom sortable (healthScoreSortableColumns).
	// Status TAK sortable — tab sudah jadi sumbu kategorik & sejak BL-24
	// health_status derivasi overall_health_score (sort terpisah nyaris duplikat
	// sort "score").
	sortCol := r.URL.Query().Get("sort")
	if !healthScoreSortableColumns[sortCol] {
		sortCol = ""
	}
	dir := r.URL.Query().Get("dir")
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}

	var items []panel.HealthScoreRowView
	var nextCursor string
	var ok bool
	switch sortCol {
	case "village":
		items, nextCursor, ok = h.healthSortByVillage(w, r, ctx, lp, uid, segment, filterStatus, query, dir, slug)
		if !ok {
			return
		}
	case "score":
		items, nextCursor, ok = h.healthSortByScore(w, r, ctx, lp, uid, segment, filterStatus, query, dir, slug)
		if !ok {
			return
		}
	case "adoption":
		items, nextCursor, ok = h.healthSortByAdoption(w, r, ctx, lp, uid, segment, filterStatus, query, dir, slug)
		if !ok {
			return
		}
	case "engagement":
		items, nextCursor, ok = h.healthSortByEngagement(w, r, ctx, lp, uid, segment, filterStatus, query, dir, slug)
		if !ok {
			return
		}
	case "support":
		items, nextCursor, ok = h.healthSortBySupport(w, r, ctx, lp, uid, segment, filterStatus, query, dir, slug)
		if !ok {
			return
		}
	case "trend":
		items, nextCursor, ok = h.healthSortByTrend(w, r, ctx, lp, uid, segment, filterStatus, query, dir, slug)
		if !ok {
			return
		}
	case "renewal":
		items, nextCursor, ok = h.healthSortByRenewal(w, r, ctx, lp, uid, segment, filterStatus, query, dir, slug)
		if !ok {
			return
		}
	default:
		items, nextCursor, ok = h.healthSortDefault(w, r, ctx, lp, uid, slug)
		if !ok {
			return
		}
	}
	if items == nil {
		items = []panel.HealthScoreRowView{}
	}

	kpiView, panels := h.healthKPIsToView(kpis)

	h.renderWorkspaceShell(w, r, "Customer Health Score", "/health-scores", panel.HealthScoreList(panel.HealthScoreListView{
		Base:          wsPath(slug, ""),
		ActiveTab:     tab,
		Segment:       segment,
		Query:         query,
		NextCursor:    nextCursor,
		After:         r.URL.Query().Get("after"),
		Trail:         pageTrail(r),
		Sort:          sortCol,
		Dir:           dir,
		KPIs:          kpiView,
		Panels:        panels,
		TableSubtitle: healthTableSubtitle(kpis.Total, sortCol),
		Rows:          items,
	}))
}
