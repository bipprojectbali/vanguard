package handler

import (
	"context"
	"net/http"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// subscriptions_renewals_sort_b.go — fungsi sort kolom type/mrr + jalur
// default (created_at DESC) untuk SubscriptionRenewals (subscriptions_
// renewals.go). Dipisah krn ambang File Health (Route/Handler 150 baris);
// badan tiap fungsi = isi case switch semula APA ADANYA, hanya dibungkus
// wrapper bertipe.

func (h *Handler) renewalsSortByType(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.SubscriptionsListFilter, uid int64, window string, today pgtype.Date, dir string,
	now time.Time, canARR bool,
) ([]panel.RenewalRow, string, bool) {
	// Kunci cursor = renewalTypeLabel PERSIS logika tampil (COALESCE
	// renewal_type/auto_renew) — TAK PERNAH NULL (auto_renew NOT NULL) →
	// pola non-nullable walau nilainya computed.
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListRenewalsSortByType(ctx, db.ListRenewalsSortByTypeParams{
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
		h.Log.Error("subscriptions: renewals sort type", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageText(rows, func(s db.ListRenewalsSortByTypeRow) (string, int64) {
		return renewalTypeLabel(s.RenewalType, s.AutoRenew), s.ID
	})
	items := make([]panel.RenewalRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, renewalRowView(renewalListRowFromTypeSort(s), now, canARR))
	}
	return items, nc, true
}

func (h *Handler) renewalsSortByMrr(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.SubscriptionsListFilter, uid int64, window string, today pgtype.Date, dir string,
	now time.Time, canARR bool,
) ([]panel.RenewalRow, string, bool) {
	cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	cursorVal, code := optNumeric(cursorRaw, "")
	if code != "" {
		hasCursor = false
	}
	rows, err := h.q(ctx).ListRenewalsSortByMrr(ctx, db.ListRenewalsSortByMrrParams{
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
		h.Log.Error("subscriptions: renewals sort mrr", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageTextNullable(rows, func(s db.ListRenewalsSortByMrrRow) (string, int64, bool) {
		if !s.Mrr.Valid {
			return "", s.ID, true
		}
		return numericStr(s.Mrr), s.ID, false
	})
	items := make([]panel.RenewalRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, renewalRowView(renewalListRowFromMrrSort(s), now, canARR))
	}
	return items, nc, true
}

func (h *Handler) renewalsSortDefault(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.SubscriptionsListFilter, uid int64, window string, today pgtype.Date,
	now time.Time, canARR bool,
) ([]panel.RenewalRow, string, bool) {
	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListRenewals(ctx, db.ListRenewalsParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		WindowFilter:    window,
		Today:           today,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("subscriptions: renewals", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPage(rows, func(s db.ListRenewalsRow) (pgtype.Timestamptz, int64) {
		return s.CreatedAt, s.ID
	})
	items := make([]panel.RenewalRow, 0, len(shown))
	for _, s := range shown {
		items = append(items, renewalRowView(renewalListRowFromDefault(s), now, canARR))
	}
	return items, nc, true
}

// renewalWindowOptions menyalin daftar jendela ke tipe view (handler pemilik enum).
func renewalWindowOptions() []panel.RenewalWindow {
	out := make([]panel.RenewalWindow, 0, len(renewalWindows))
	for _, wnd := range renewalWindows {
		out = append(out, panel.RenewalWindow{Key: wnd.Key, Label: wnd.Label})
	}
	return out
}
