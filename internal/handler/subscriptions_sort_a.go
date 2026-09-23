package handler

import (
	"context"
	"net/http"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_sort_a.go — fungsi sort per-kolom utk SubscriptionsList
// (village/status/plan; lanjutan mrr/renewal/csm di subscriptions_sort_b.go).
// Diekstrak dari SubscriptionsList (subscriptions_page.go) krn ambang File
// Health (Route/Handler 150 baris) — badan tiap fungsi SALINAN case asli,
// tak ada perubahan logika.

func (h *Handler) subsSortByVillage(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.SubscriptionsListFilter, uid int64, statusFilter, query, dir string, names map[int64]string, canARR bool, now time.Time) ([]panel.SubRow, string, bool) {
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListSubscriptionsSortByVillage(ctx, db.ListSubscriptionsSortByVillageParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     filter.ScopeAll,
		IsOwn:        filter.IsOwn,
		Uid:          &uid,
		StatusFilter: statusFilter,
		Search:       query,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("subscriptions: list sort village", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageText(rows, func(s db.ListSubscriptionsSortByVillageRow) (string, int64) {
		return s.VillageName, s.ID
	})
	items := make([]panel.SubRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, subRowView(subListRowFromVillageSort(s), names, canARR, now))
	}
	return items, nc, true
}

func (h *Handler) subsSortByStatus(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.SubscriptionsListFilter, uid int64, statusFilter, query, dir string, names map[int64]string, canARR bool, now time.Time) ([]panel.SubRow, string, bool) {
	// Masa Berlaku diurut RAW status (Trial/Active/…, alfabetis) — keputusan
	// user, BUKAN band urgensi derivasi (subDerivedStatus). Tak nullable →
	// reuse pageCursorText/splitPageText apa adanya (pola sama "village").
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListSubscriptionsSortByStatus(ctx, db.ListSubscriptionsSortByStatusParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     filter.ScopeAll,
		IsOwn:        filter.IsOwn,
		Uid:          &uid,
		StatusFilter: statusFilter,
		Search:       query,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("subscriptions: list sort status", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageText(rows, func(s db.ListSubscriptionsSortByStatusRow) (string, int64) {
		return s.Status, s.ID
	})
	items := make([]panel.SubRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, subRowView(subListRowFromStatusSort(s), names, canARR, now))
	}
	return items, nc, true
}

func (h *Handler) subsSortByPlan(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.SubscriptionsListFilter, uid int64, statusFilter, query, dir string, names map[int64]string, canARR bool, now time.Time) ([]panel.SubRow, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListSubscriptionsSortByPlan(ctx, db.ListSubscriptionsSortByPlanParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorIsNull: isNull,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     filter.ScopeAll,
		IsOwn:        filter.IsOwn,
		Uid:          &uid,
		StatusFilter: statusFilter,
		Search:       query,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("subscriptions: list sort plan", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageTextNullable(rows, func(s db.ListSubscriptionsSortByPlanRow) (string, int64, bool) {
		if s.PlanName == nil {
			return "", s.ID, true
		}
		return *s.PlanName, s.ID, false
	})
	items := make([]panel.SubRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, subRowView(subListRowFromPlanSort(s), names, canARR, now))
	}
	return items, nc, true
}

func (h *Handler) subsSortDefault(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.SubscriptionsListFilter, uid int64, statusFilter, query string, cursorAt pgtype.Timestamptz, cursorID int64, names map[int64]string, canARR bool, now time.Time) ([]panel.SubRow, string, bool) {
	rows, err := h.q(ctx).ListSubscriptions(ctx, db.ListSubscriptionsParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		StatusFilter:    statusFilter,
		Search:          query,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("subscriptions: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPage(rows, func(s db.ListSubscriptionsRow) (pgtype.Timestamptz, int64) {
		return s.CreatedAt, s.ID
	})
	items := make([]panel.SubRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, subRowView(subListRowFromDefault(s), names, canARR, now))
	}
	return items, nc, true
}
