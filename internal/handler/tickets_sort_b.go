package handler

import (
	"context"
	"net/http"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// tickets_sort_b.go — fungsi sort kolom status/agent/sla untuk TicketsList
// (tickets.go). Dipisah krn ambang File Health (Route/Handler 150 baris);
// badan tiap fungsi = isi case switch semula APA ADANYA, hanya dibungkus
// wrapper bertipe.

func (h *Handler) ticketsSortByStatus(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.TicketsListFilter, uid int64, filterStatus string, filterSLABreached, filterSLAAtRisk bool,
	query, dir string,
) ([]panel.TicketRow, string, bool) {
	// Status diurut RAW (baru/diproses/menunggu/selesai) — beda dari
	// Renewals (BL-157g) yang mengecualikan Status krn derivasi penuh; di
	// sini status ADALAH kolom mentah, aman disortir langsung.
	cursorVal, cursorSortID, hasCursor := pageCursorText(r)
	rows, err := h.q(ctx).ListTicketsSortByStatus(ctx, db.ListTicketsSortByStatusParams{
		HasCursor: hasCursor, Dir: dir, CursorVal: cursorVal, CursorID: cursorSortID,
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		FilterStatus: filterStatus, FilterSlaBreached: filterSLABreached, FilterSlaAtRisk: filterSLAAtRisk,
		Search: query, PageSize: pageSize + 1,
	})
	if err != nil {
		h.Log.Error("tickets: list sort status", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageText(rows, func(r db.ListTicketsSortByStatusRow) (string, int64) {
		return r.Status, r.ID
	})
	items := make([]panel.TicketRow, 0, len(shown))
	for _, row := range shown {
		items = append(items, ticketRowView(ticketListRowFromStatusSort(row)))
	}
	return items, nc, true
}

func (h *Handler) ticketsSortByAgent(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.TicketsListFilter, uid int64, filterStatus string, filterSLABreached, filterSLAAtRisk bool,
	query, dir string,
) ([]panel.TicketRow, string, bool) {
	// Kunci cursor = u.name PLAIN (bukan COALESCE dgn email spt CSM
	// Subscriptions) — ticketRowView tampilkan AssignedToName apa adanya.
	cursorVal, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	rows, err := h.q(ctx).ListTicketsSortByAgent(ctx, db.ListTicketsSortByAgentParams{
		HasCursor: hasCursor, Dir: dir, CursorIsNull: isNull, CursorVal: cursorVal, CursorID: cursorSortID,
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		FilterStatus: filterStatus, FilterSlaBreached: filterSLABreached, FilterSlaAtRisk: filterSLAAtRisk,
		Search: query, PageSize: pageSize + 1,
	})
	if err != nil {
		h.Log.Error("tickets: list sort agent", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageTextNullable(rows, func(r db.ListTicketsSortByAgentRow) (string, int64, bool) {
		if r.AssignedToName == nil {
			return "", r.ID, true
		}
		return *r.AssignedToName, r.ID, false
	})
	items := make([]panel.TicketRow, 0, len(shown))
	for _, row := range shown {
		items = append(items, ticketRowView(ticketListRowFromAgentSort(row)))
	}
	return items, nc, true
}

func (h *Handler) ticketsSortBySla(w http.ResponseWriter, r *http.Request, ctx context.Context,
	filter db.TicketsListFilter, uid int64, filterStatus string, filterSLABreached, filterSLAAtRisk bool,
	query, dir string,
) ([]panel.TicketRow, string, bool) {
	// SLA diurut RAW sla_deadline_at — bukan label turunan formatSLALabel
	// (Terpenuhi/Terlanggar/"Nj Mm lagi"), sama prinsip Status Subscriptions:
	// jangan duplikasi derivasi ke SQL. Cursor perlu PRESISI PENUH (bukan
	// dateTimeLayout menit-saja punya optDateTime) agar tiket berbagi menit
	// yang sama tak salah lompat baris.
	cursorRaw, cursorSortID, isNull, hasCursor := pageCursorTextNullable(r)
	cursorVal, code := ticketSlaCursorVal(cursorRaw)
	if code != "" {
		hasCursor = false
	}
	rows, err := h.q(ctx).ListTicketsSortBySla(ctx, db.ListTicketsSortBySlaParams{
		HasCursor: hasCursor, Dir: dir, CursorIsNull: isNull, CursorVal: cursorVal, CursorID: cursorSortID,
		ScopeAll: filter.ScopeAll, IsOwn: filter.IsOwn, Uid: &uid,
		FilterStatus: filterStatus, FilterSlaBreached: filterSLABreached, FilterSlaAtRisk: filterSLAAtRisk,
		Search: query, PageSize: pageSize + 1,
	})
	if err != nil {
		h.Log.Error("tickets: list sort sla", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return nil, "", false
	}
	shown, nc := splitPageTextNullable(rows, func(r db.ListTicketsSortBySlaRow) (string, int64, bool) {
		if !r.SlaDeadlineAt.Valid {
			return "", r.ID, true
		}
		return ticketSlaCursorStr(r.SlaDeadlineAt), r.ID, false
	})
	items := make([]panel.TicketRow, 0, len(shown))
	for _, row := range shown {
		items = append(items, ticketRowView(ticketListRowFromSlaSort(row)))
	}
	return items, nc, true
}

// ticketSlaCursorVal mengurai nilai cursor SLA (RFC3339Nano, presisi penuh) —
// BEDA dari optDateTime/dateTimeLayout (dipotong ke menit utk <input
// datetime-local>): cursor keyset butuh presisi penuh agar dua tiket yang
// berbagi sla_deadline_at ke menit yang sama tak salah lompat/duplikat baris.
func ticketSlaCursorVal(s string) (pgtype.Timestamptz, string) {
	if s == "" {
		return pgtype.Timestamptz{}, "datetime"
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return pgtype.Timestamptz{}, "datetime"
	}
	return pgtype.Timestamptz{Time: t.UTC(), Valid: true}, ""
}

// ticketSlaCursorStr memformat Timestamptz ke teks cursor (cermin ticketSlaCursorVal).
func ticketSlaCursorStr(t pgtype.Timestamptz) string {
	if !t.Valid {
		return ""
	}
	return t.Time.UTC().Format(time.RFC3339Nano)
}
