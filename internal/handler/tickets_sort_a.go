package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// tickets_sort_a.go — fungsi sort kolom village/subject/priority + jalur
// default (created_at DESC) untuk TicketsList (tickets.go). Dipisah krn
// ambang File Health (Route/Handler 150 baris); badan tiap fungsi = isi
// case switch semula APA ADANYA, hanya dibungkus wrapper bertipe.

func (h *Handler) ticketsSortByVillage(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.TicketsListFilter, uid int64, filterStatus string, filterSLABreached, filterSLAAtRisk bool,
	query, dir string,
) ([]panel.TicketRow, string, bool) {
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListTicketsSortByVillage(ctx, db.ListTicketsSortByVillageParams{
		HasCursor: hasCursor, Dir: dir, CursorVal: cursorVal, CursorID: cursorSortID,
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		FilterStatus: filterStatus, FilterSlaBreached: filterSLABreached, FilterSlaAtRisk: filterSLAAtRisk,
		Search: query, PageSize: pageSize + 1,
	})
	if err != nil {
		h.Log.Error("tickets: list sort village", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageText(rows, func(r db.ListTicketsSortByVillageRow) (string, int64) {
		return r.AccountName, r.ID
	})
	items := make([]panel.TicketRow, 0, len(shown))
	for _, row := range shown {
		items = append(items, ticketRowView(ticketListRowFromVillageSort(row)))
	}
	return items, nc, true
}

func (h *Handler) ticketsSortBySubject(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.TicketsListFilter, uid int64, filterStatus string, filterSLABreached, filterSLAAtRisk bool,
	query, dir string,
) ([]panel.TicketRow, string, bool) {
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListTicketsSortBySubject(ctx, db.ListTicketsSortBySubjectParams{
		HasCursor: hasCursor, Dir: dir, CursorVal: cursorVal, CursorID: cursorSortID,
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		FilterStatus: filterStatus, FilterSlaBreached: filterSLABreached, FilterSlaAtRisk: filterSLAAtRisk,
		Search: query, PageSize: pageSize + 1,
	})
	if err != nil {
		h.Log.Error("tickets: list sort subject", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageText(rows, func(r db.ListTicketsSortBySubjectRow) (string, int64) {
		return r.Subject, r.ID
	})
	items := make([]panel.TicketRow, 0, len(shown))
	for _, row := range shown {
		items = append(items, ticketRowView(ticketListRowFromSubjectSort(row)))
	}
	return items, nc, true
}

func (h *Handler) ticketsSortByPriority(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.TicketsListFilter, uid int64, filterStatus string, filterSLABreached, filterSLAAtRisk bool,
	query, dir string,
) ([]panel.TicketRow, string, bool) {
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListTicketsSortByPriority(ctx, db.ListTicketsSortByPriorityParams{
		HasCursor: hasCursor, Dir: dir, CursorVal: cursorVal, CursorID: cursorSortID,
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		FilterStatus: filterStatus, FilterSlaBreached: filterSLABreached, FilterSlaAtRisk: filterSLAAtRisk,
		Search: query, PageSize: pageSize + 1,
	})
	if err != nil {
		h.Log.Error("tickets: list sort priority", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageText(rows, func(r db.ListTicketsSortByPriorityRow) (string, int64) {
		return r.Priority, r.ID
	})
	items := make([]panel.TicketRow, 0, len(shown))
	for _, row := range shown {
		items = append(items, ticketRowView(ticketListRowFromPrioritySort(row)))
	}
	return items, nc, true
}

func (h *Handler) ticketsSortDefault(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.TicketsListFilter, uid int64, filterStatus string, filterSLABreached, filterSLAAtRisk bool,
	query string,
) ([]panel.TicketRow, string, bool) {
	cursorAt, cursorID := pageCursor(r)
	rows, err := h.q(ctx).ListTickets(ctx, db.ListTicketsParams{
		CursorCreatedAt:   cursorAt,
		CursorID:          cursorID,
		ScopeAll:          filter.ScopeAll,
		IsOwn:             filter.IsOwn,
		Uid:               &uid,
		FilterStatus:      filterStatus,
		FilterSlaBreached: filterSLABreached,
		FilterSlaAtRisk:   filterSLAAtRisk,
		Search:            query,
		PageSize:          pageSize + 1,
	})
	if err != nil {
		h.Log.Error("tickets: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPage(rows, func(r db.ListTicketsRow) (pgtype.Timestamptz, int64) {
		return r.CreatedAt, r.ID
	})
	items := make([]panel.TicketRow, 0, len(shown))
	for _, row := range shown {
		items = append(items, ticketRowView(ticketListRowFromDefault(row)))
	}
	return items, nc, true
}
