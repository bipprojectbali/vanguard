package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// health_score_sort_c.go — lanjutan health_score_sort_a.go/_b.go (renewal +
// jalur default created_at DESC).

func (h *Handler) healthSortByRenewal(w http.ResponseWriter, r *http.Request, ctx context.Context, lp db.ListHealthScoresParams, uid int64, segment, filterStatus, query, dir, slug string) ([]panel.HealthScoreRowView, string, bool) {
	cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	// cursor Jatuh Tempo kanonik = ISO tanggal apa adanya (optDate,
	// sales_format.go), mirror kolom Close Date Deals/Renewal Date Renewals.
	cursorVal, code := optDate(cursorRaw)
	if code != "" {
		hasCursor = false
	}
	rows, err := h.q(ctx).ListHealthScoresSortByRenewal(ctx, db.ListHealthScoresSortByRenewalParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorIsNull: isNull,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     lp.ScopeAll,
		IsCsm:        lp.IsCsm,
		Uid:          &uid,
		IsSales:      lp.IsSales,
		Segment:      segment,
		FilterStatus: filterStatus,
		Search:       query,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("health-scores: list sort renewal", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageTextNullable(rows, func(s db.ListHealthScoresSortByRenewalRow) (string, int64, bool) {
		if !s.RenewalEndDate.Valid {
			return "", s.ID, true
		}
		return dateStr(s.RenewalEndDate), s.ID, false
	})
	items := make([]panel.HealthScoreRowView, 0, len(shown))
	for _, s := range shown {
		items = append(items, healthRowToView(healthListRowFromRenewalSort(s), slug, appTZ, uid))
	}
	return items, nc, true
}

func (h *Handler) healthSortDefault(w http.ResponseWriter, r *http.Request, ctx context.Context, lp db.ListHealthScoresParams, uid int64, slug string) ([]panel.HealthScoreRowView, string, bool) {
	rows, err := h.q(ctx).ListHealthScores(ctx, lp)
	if err != nil {
		h.Log.Error("health-scores: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPage(rows, func(row db.ListHealthScoresRow) (pgtype.Timestamptz, int64) {
		return row.CreatedAt, row.ID
	})
	items := make([]panel.HealthScoreRowView, 0, len(shown))
	for _, row := range shown {
		items = append(items, healthRowToView(healthListRowFromDefault(row), slug, appTZ, uid))
	}
	return items, nc, true
}
