package handler

import (
	"context"
	"net/http"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_renewals_sort_a.go — fungsi sort kolom village/plan/date
// untuk SubscriptionRenewals (subscriptions_renewals.go). Dipisah krn ambang
// File Health (Route/Handler 150 baris); badan tiap fungsi = isi case switch
// semula APA ADANYA, hanya dibungkus wrapper bertipe.

func (h *Handler) renewalsSortByVillage(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.SubscriptionsListFilter, uid int64, window string, today pgtype.Date, dir string,
	now time.Time, canARR bool,
) ([]panel.RenewalRow, string, bool) {
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListRenewalsSortByVillage(ctx, db.ListRenewalsSortByVillageParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     filter.ScopeAll,
		IsOwn:        filter.IsOwn,
		Uid:          &uid,
		WindowFilter: window,
		Today:        today,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("subscriptions: renewals sort village", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageText(rows, func(s db.ListRenewalsSortByVillageRow) (string, int64) {
		return s.VillageName, s.ID
	})
	items := make([]panel.RenewalRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, renewalRowView(renewalListRowFromVillageSort(s), now, canARR))
	}
	return items, nc, true
}

func (h *Handler) renewalsSortByPlan(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.SubscriptionsListFilter, uid int64, window string, today pgtype.Date, dir string,
	now time.Time, canARR bool,
) ([]panel.RenewalRow, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListRenewalsSortByPlan(ctx, db.ListRenewalsSortByPlanParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorIsNull: isNull,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     filter.ScopeAll,
		IsOwn:        filter.IsOwn,
		Uid:          &uid,
		WindowFilter: window,
		Today:        today,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("subscriptions: renewals sort plan", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageTextNullable(rows, func(s db.ListRenewalsSortByPlanRow) (string, int64, bool) {
		if s.PlanName == nil {
			return "", s.ID, true
		}
		return *s.PlanName, s.ID, false
	})
	items := make([]panel.RenewalRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, renewalRowView(renewalListRowFromPlanSort(s), now, canARR))
	}
	return items, nc, true
}

func (h *Handler) renewalsSortByDate(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.SubscriptionsListFilter, uid int64, window string, today pgtype.Date, dir string,
	now time.Time, canARR bool,
) ([]panel.RenewalRow, string, bool) {
	// end_date DIJAMIN terisi di dasbor ini (ListRenewals: WHERE end_date IS
	// NOT NULL) → pola non-nullable (pageCursorText), tanpa cursor_is_null.
	cursorRaw, cursorSortID, hasCursor := pageCursorText(r)
	cursorVal, code := optDate(cursorRaw)
	if code != "" {
		hasCursor = false
	}
	rows, err := h.q(ctx).ListRenewalsSortByDate(ctx, db.ListRenewalsSortByDateParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     filter.ScopeAll,
		IsOwn:        filter.IsOwn,
		Uid:          &uid,
		WindowFilter: window,
		Today:        today,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("subscriptions: renewals sort date", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageText(rows, func(s db.ListRenewalsSortByDateRow) (string, int64) {
		return dateStr(s.EndDate), s.ID
	})
	items := make([]panel.RenewalRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, renewalRowView(renewalListRowFromDateSort(s), now, canARR))
	}
	return items, nc, true
}
