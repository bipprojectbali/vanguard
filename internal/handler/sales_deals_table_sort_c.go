package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_deals_table_sort_c.go — fungsi sort kolom close/owner + jalur
// default (created_at DESC) untuk dealsTable (sales_deals_table_page.go).
// Dipisah krn ambang File Health (Route/Handler 150 baris); badan tiap
// fungsi = isi case switch semula APA ADANYA, hanya dibungkus wrapper
// bertipe.

func (h *Handler) dealsSortByCloseDate(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.DealsListFilter, uid int64, mineOnly bool, stageFilter, query, dir string,
	names map[int64]string,
) ([]db.Deal, string, bool) {
	cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	// cursor Perkiraan Tutup kanonik = ISO tanggal apa adanya (optDate ada
	// di sales_format.go), mirror Renewal Date Subscriptions.
	cursorVal, code := optDate(cursorRaw)
	if code != "" {
		hasCursor = false
	}
	rows, err := h.q(ctx).ListDealsSortByCloseDate(ctx, db.ListDealsSortByCloseDateParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorIsNull: isNull,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     filter.ScopeAll,
		IsOwn:        filter.IsOwn,
		Uid:          &uid,
		MineOnly:     mineOnly,
		StageFilter:  stageFilter,
		Search:       query,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("deals: list sort close date", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPageTextNullable(rows, func(d db.Deal) (string, int64, bool) {
		if !d.ExpectedCloseDate.Valid {
			return "", d.ID, true
		}
		return dateStr(d.ExpectedCloseDate), d.ID, false
	})
	return shown, nextCursor, true
}

func (h *Handler) dealsSortByOwner(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.DealsListFilter, uid int64, mineOnly bool, stageFilter, query, dir string,
	names map[int64]string,
) ([]db.Deal, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListDealsSortByOwner(ctx, db.ListDealsSortByOwnerParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorIsNull: isNull,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     filter.ScopeAll,
		IsOwn:        filter.IsOwn,
		Uid:          &uid,
		MineOnly:     mineOnly,
		StageFilter:  stageFilter,
		Search:       query,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("deals: list sort owner", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	// Kunci cursor Pemilik = nama/email resolusi peta anggota (ownerName),
	// PERSIS yang ditampilkan — bukan owner mentah. NULL-nya mengikuti
	// deal_owner asli (bukan string kosong hasil ownerName), sama dengan
	// kondisi NULL di kunci sort SQL (LEFT JOIN users).
	shown, nextCursor := splitPageTextNullable(rows, func(d db.Deal) (string, int64, bool) {
		if d.DealOwner == nil {
			return "", d.ID, true
		}
		return ownerName(d.DealOwner, names), d.ID, false
	})
	return shown, nextCursor, true
}

func (h *Handler) dealsSortDefault(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.DealsListFilter, uid int64, mineOnly bool, stageFilter, query string,
	names map[int64]string,
) ([]db.Deal, string, bool) {
	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListDeals(ctx, db.ListDealsParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		MineOnly:        mineOnly,
		StageFilter:     stageFilter,
		Search:          query,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("deals: table", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPage(rows, func(d db.Deal) (pgtype.Timestamptz, int64) {
		return d.CreatedAt, d.ID
	})
	return shown, nextCursor, true
}
