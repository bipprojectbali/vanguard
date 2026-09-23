package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"

	"github.com/jackc/pgx/v5/pgtype"
)

// sales_leads_sort_c.go — lanjutan sales_leads_sort_a.go/_b.go (owner + jalur
// default created_at DESC).

// leadsSortByOwner — kunci cursor Pemilik = nama/email resolusi peta anggota
// (ownerName), PERSIS yang ditampilkan — bukan owner mentah. NULL-nya
// mengikuti lead_owner asli (bukan string kosong hasil ownerName), sama
// dengan kondisi NULL di kunci sort SQL (LEFT JOIN users).
func (h *Handler) leadsSortByOwner(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.LeadsListFilter, uid int64, mineOnly bool, statusFilter, query, dir string, names map[int64]string) ([]db.Lead, string, bool) {
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListLeadsSortByOwner(ctx, db.ListLeadsSortByOwnerParams{
		HasCursor:    hasCursor,
		Dir:          dir,
		CursorIsNull: isNull,
		CursorVal:    cursorVal,
		CursorID:     cursorSortID,
		ScopeAll:     filter.ScopeAll,
		IsOwn:        filter.IsOwn,
		Uid:          &uid,
		MineOnly:     mineOnly,
		StatusFilter: statusFilter,
		Search:       query,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("leads: list sort owner", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPageTextNullable(rows, func(l db.Lead) (string, int64, bool) {
		if l.LeadOwner == nil {
			return "", l.ID, true
		}
		return ownerName(l.LeadOwner, names), l.ID, false
	})
	return shown, nextCursor, true
}

func (h *Handler) leadsSortDefault(w http.ResponseWriter, r *http.Request, ctx context.Context, filter db.LeadsListFilter, uid int64, mineOnly bool, statusFilter, query string) ([]db.Lead, string, bool) {
	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListLeads(ctx, db.ListLeadsParams{
		CursorCreatedAt: cursorAt,
		CursorID:        cursorID,
		ScopeAll:        filter.ScopeAll,
		IsOwn:           filter.IsOwn,
		Uid:             &uid,
		MineOnly:        mineOnly,
		StatusFilter:    statusFilter,
		Search:          query,
		PageSize:        pageSize + 1,
	})
	if err != nil {
		h.Log.Error("leads: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nextCursor := splitPage(rows, func(l db.Lead) (pgtype.Timestamptz, int64) {
		return l.CreatedAt, l.ID
	})
	return shown, nextCursor, true
}
