package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// health_score_sort_a.go — fungsi sort per-kolom utk HealthScoreList
// (village/score/adoption; lanjutan engagement/support/trend di
// health_score_sort_b.go, renewal/default di health_score_sort_c.go).
// Diekstrak dari HealthScoreList (health_score.go) krn ambang File Health
// (Route/Handler 150 baris) — badan tiap fungsi SALINAN case asli, tak ada
// perubahan logika.

func (h *Handler) healthSortByVillage(w http.ResponseWriter, r *http.Request, ctx context.Context, lp db.ListHealthScoresParams, uid int64, segment, filterStatus, query, dir, slug string) ([]panel.HealthScoreRowView, string, bool) {
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListHealthScoresSortByVillage(ctx, db.ListHealthScoresSortByVillageParams{
		HasCursor:    hasCursor,
		Dir:          dir,
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
		h.Log.Error("health-scores: list sort village", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageText(rows, func(s db.ListHealthScoresSortByVillageRow) (string, int64) {
		return s.AccountName, s.ID
	})
	items := make([]panel.HealthScoreRowView, 0, len(shown))
	for _, s := range shown {
		items = append(items, healthRowToView(healthListRowFromVillageSort(s), slug, appTZ, uid))
	}
	return items, nc, true
}

func (h *Handler) healthSortByScore(w http.ResponseWriter, r *http.Request, ctx context.Context, lp db.ListHealthScoresParams, uid int64, segment, filterStatus, query, dir, slug string) ([]panel.HealthScoreRowView, string, bool) {
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
	rows, err := h.q(ctx).ListHealthScoresSortByScore(ctx, db.ListHealthScoresSortByScoreParams{
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
		h.Log.Error("health-scores: list sort score", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageTextNullable(rows, func(s db.ListHealthScoresSortByScoreRow) (string, int64, bool) {
		if s.OverallHealthScore == nil {
			return "", s.ID, true
		}
		return strconv.FormatInt(int64(*s.OverallHealthScore), 10), s.ID, false
	})
	items := make([]panel.HealthScoreRowView, 0, len(shown))
	for _, s := range shown {
		items = append(items, healthRowToView(healthListRowFromScoreSort(s), slug, appTZ, uid))
	}
	return items, nc, true
}

func (h *Handler) healthSortByAdoption(w http.ResponseWriter, r *http.Request, ctx context.Context, lp db.ListHealthScoresParams, uid int64, segment, filterStatus, query, dir, slug string) ([]panel.HealthScoreRowView, string, bool) {
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
	rows, err := h.q(ctx).ListHealthScoresSortByAdoption(ctx, db.ListHealthScoresSortByAdoptionParams{
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
		h.Log.Error("health-scores: list sort adoption", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageTextNullable(rows, func(s db.ListHealthScoresSortByAdoptionRow) (string, int64, bool) {
		if s.AdoptionScore == nil {
			return "", s.ID, true
		}
		return strconv.FormatInt(int64(*s.AdoptionScore), 10), s.ID, false
	})
	items := make([]panel.HealthScoreRowView, 0, len(shown))
	for _, s := range shown {
		items = append(items, healthRowToView(healthListRowFromAdoptionSort(s), slug, appTZ, uid))
	}
	return items, nc, true
}
