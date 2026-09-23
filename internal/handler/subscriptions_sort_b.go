package handler

import (
	"context"
	"net/http"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// subscriptions_sort_b.go — lanjutan subscriptions_sort_a.go (mrr/renewal/csm).

func (h *Handler) subsSortByMrr(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.SubscriptionsListFilter, uid int64, statusFilter, query, dir string, names map[int64]string, canARR bool, now time.Time) ([]panel.SubRow, string, bool) {
	cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	// cursor MRR kanonik = teks desimal apa adanya (optNumeric ada di
	// sales_format_number.go) — urutan SUNGGUHAN terjadi di SQL atas
	// cursor_val bertipe numeric asli, jadi teks cursor tak perlu
	// menjaga urutan leksikal, cukup bolak-balik lossless.
	cursorVal, code := optNumeric(cursorRaw, "")
	if code != "" {
		hasCursor = false
	}
	rows, err := h.q(ctx).ListSubscriptionsSortByMrr(ctx, db.ListSubscriptionsSortByMrrParams{
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
		h.Log.Error("subscriptions: list sort mrr", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageTextNullable(rows, func(s db.ListSubscriptionsSortByMrrRow) (string, int64, bool) {
		if !s.Mrr.Valid {
			return "", s.ID, true
		}
		return numericStr(s.Mrr), s.ID, false
	})
	items := make([]panel.SubRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, subRowView(subListRowFromMrrSort(s), names, canARR, now))
	}
	return items, nc, true
}

func (h *Handler) subsSortByRenewal(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.SubscriptionsListFilter, uid int64, statusFilter, query, dir string, names map[int64]string, canARR bool, now time.Time) ([]panel.SubRow, string, bool) {
	cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	cursorVal, code := optDate(cursorRaw)
	if code != "" {
		hasCursor = false
	}
	rows, err := h.q(ctx).ListSubscriptionsSortByRenewal(ctx, db.ListSubscriptionsSortByRenewalParams{
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
		h.Log.Error("subscriptions: list sort renewal", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageTextNullable(rows, func(s db.ListSubscriptionsSortByRenewalRow) (string, int64, bool) {
		if !s.EndDate.Valid {
			return "", s.ID, true
		}
		return dateStr(s.EndDate), s.ID, false
	})
	items := make([]panel.SubRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, subRowView(subListRowFromRenewalSort(s), names, canARR, now))
	}
	return items, nc, true
}

func (h *Handler) subsSortByCsm(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.SubscriptionsListFilter, uid int64, statusFilter, query, dir string, names map[int64]string, canARR bool, now time.Time) ([]panel.SubRow, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListSubscriptionsSortByCsm(ctx, db.ListSubscriptionsSortByCsmParams{
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
		h.Log.Error("subscriptions: list sort csm", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	// Kunci cursor CSM = nama/email resolusi peta anggota (ownerName), PERSIS
	// yang ditampilkan — bukan owner mentah. NULL-nya mengikuti
	// subscription_owner asli (bukan string kosong hasil ownerName), sama
	// dengan kondisi NULL di kunci sort SQL (LEFT JOIN users).
	shown, nc := splitPageTextNullable(rows, func(s db.ListSubscriptionsSortByCsmRow) (string, int64, bool) {
		if s.SubscriptionOwner == nil {
			return "", s.ID, true
		}
		return ownerName(s.SubscriptionOwner, names), s.ID, false
	})
	items := make([]panel.SubRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, subRowView(subListRowFromCsmSort(s), names, canARR, now))
	}
	return items, nc, true
}
