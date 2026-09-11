package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// HealthScoreList — GET /health-scores.
// Workspace-level listing health score semua desa dalam scope role.
// Gate: crm:health read (canViewHealthScore). F3 ownership via flag boolean.
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

	// Filter status dari tab (?tab=sehat|berisiko|kritis).
	tab := r.URL.Query().Get("tab")
	lp.FilterStatus = healthTabToStatus(tab)

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

	rows, err := h.q(ctx).ListHealthScores(ctx, lp)
	if err != nil {
		h.Log.Error("health-scores: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	shown, nextCursor := splitPage(rows, func(r db.ListHealthScoresRow) (pgtype.Timestamptz, int64) {
		return r.CreatedAt, r.ID
	})

	uid := session.UserID(ctx)
	items := make([]panel.HealthScoreRowView, 0, len(shown))
	for _, row := range shown {
		items = append(items, healthRowToView(row, slug, appTZ, uid))
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
		KPIs:          kpiView,
		Panels:        panels,
		TableSubtitle: healthTableSubtitle(kpis.Total),
		Rows:          items,
	}))
}

// healthSegment menormalkan ?segment= ke nilai SQL sah. Apa pun selain "churned"
// → "active" (default aman: nilai janggal jatuh ke populasi pelanggan aktif, tak
// pernah membuka data churned tanpa diminta eksplisit).
func healthSegment(seg string) string {
	if seg == "churned" {
		return "churned"
	}
	return "active"
}

// healthTabToStatus memetakan nilai query-param ?tab= ke health_status DB.
// Tab kosong / "semua" → "" (tidak difilter).
func healthTabToStatus(tab string) string {
	switch tab {
	case "sehat":
		return "Healthy"
	case "berisiko":
		return "At-Risk"
	case "kritis":
		return "Critical"
	default:
		return ""
	}
}
