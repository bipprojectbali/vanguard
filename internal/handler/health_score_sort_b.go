package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// health_score_sort_b.go — lanjutan health_score_sort_a.go (engagement/support/
// trend; renewal/default di health_score_sort_c.go).

func (h *Handler) healthSortByEngagement(w http.ResponseWriter, r *http.Request, ctx context.Context, lp db.ListHealthScoresParams, uid int64, segment, filterStatus, query, dir, slug string) ([]panel.HealthScoreRowView, string, bool) {
	cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	var cursorVal int16
	if hasCursor && !isNull {
		n, perr := strconv.ParseInt(cursorRaw, 10, 16)
		if perr != nil {
			hasCursor = false
		} else {
			cursorVal = int16(n)
		}
	}
	rows, err := h.q(ctx).ListHealthScoresSortByEngagement(ctx, db.ListHealthScoresSortByEngagementParams{
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
		h.Log.Error("health-scores: list sort engagement", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageTextNullable(rows, func(s db.ListHealthScoresSortByEngagementRow) (string, int64, bool) {
		if s.EngagementScore == nil {
			return "", s.ID, true
		}
		return strconv.FormatInt(int64(*s.EngagementScore), 10), s.ID, false
	})
	items := make([]panel.HealthScoreRowView, 0, len(shown))
	for _, s := range shown {
		items = append(items, healthRowToView(healthListRowFromEngagementSort(s), slug, appTZ, uid))
	}
	return items, nc, true
}

func (h *Handler) healthSortBySupport(w http.ResponseWriter, r *http.Request, ctx context.Context, lp db.ListHealthScoresParams, uid int64, segment, filterStatus, query, dir, slug string) ([]panel.HealthScoreRowView, string, bool) {
	cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	var cursorVal int16
	if hasCursor && !isNull {
		n, perr := strconv.ParseInt(cursorRaw, 10, 16)
		if perr != nil {
			hasCursor = false
		} else {
			cursorVal = int16(n)
		}
	}
	rows, err := h.q(ctx).ListHealthScoresSortBySupport(ctx, db.ListHealthScoresSortBySupportParams{
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
		h.Log.Error("health-scores: list sort support", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageTextNullable(rows, func(s db.ListHealthScoresSortBySupportRow) (string, int64, bool) {
		if s.SupportScore == nil {
			return "", s.ID, true
		}
		return strconv.FormatInt(int64(*s.SupportScore), 10), s.ID, false
	})
	items := make([]panel.HealthScoreRowView, 0, len(shown))
	for _, s := range shown {
		items = append(items, healthRowToView(healthListRowFromSupportSort(s), slug, appTZ, uid))
	}
	return items, nc, true
}

func (h *Handler) healthSortByTrend(w http.ResponseWriter, r *http.Request, ctx context.Context, lp db.ListHealthScoresParams, uid int64, segment, filterStatus, query, dir, slug string) ([]panel.HealthScoreRowView, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListHealthScoresSortByTrend(ctx, db.ListHealthScoresSortByTrendParams{
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
		h.Log.Error("health-scores: list sort trend", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageTextNullable(rows, func(s db.ListHealthScoresSortByTrendRow) (string, int64, bool) {
		if s.ScoreTrend == nil {
			return "", s.ID, true
		}
		return *s.ScoreTrend, s.ID, false
	})
	items := make([]panel.HealthScoreRowView, 0, len(shown))
	for _, s := range shown {
		items = append(items, healthRowToView(healthListRowFromTrendSort(s), slug, appTZ, uid))
	}
	return items, nc, true
}
