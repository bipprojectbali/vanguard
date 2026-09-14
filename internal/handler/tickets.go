package handler

import (
	"net/http"
	"strings"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// tickets.go — HALAMAN baca Tickets / Cases (Modul 6 Customer Success, slice
// B2, wireframe 6.9). Aksi tulis (create, status update) di tickets_save.go —
// dipisah karena keduanya tumbuh dengan aturan berbeda.
//
// Akses lewat tiga sumbu orthogonal:
//   - F2 (Casbin bisnis): canViewTickets/canWriteTickets — GERBANG modul.
//     write mencakup read; semua peran CRM lolos read (termasuk Sales & CSM).
//   - F3 (ownership): TicketsListFilterFor(dataScope, canWrite) — baris mana
//     yang tampil. Support istimewa: data_scope='none' tapi canWrite=true
//     → ScopeAll (override di ownership.go). Sumber tunggal dengan CountTicketKPIs.
//   - RLS: h.q ber-tenant → mengisolasi workspace di bawah semua ini.

// ticketsDropdownLimit = batas opsi dropdown per-tipe di form tiket baru
// (guardrail: tak ada full-scan tanpa batas; workspace realistis punya ≤200
// desa aktif). Cermin pola activityTargetPickerLimit.
const ticketsDropdownLimit = 200

// TicketsList — GET /w/{workspace}/tickets. Daftar tiket + KPI header + filter
// tab. canViewTickets=false → 403. F3 lewat TicketsListFilterFor.
func (h *Handler) TicketsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewTickets(ctx) {
		h.renderTicketsForbidden(w, r)
		return
	}

	dataScope := session.BusinessDataScope(ctx)
	canWrite := canWriteTickets(ctx)
	uid := session.UserID(ctx)
	filter := db.TicketsListFilterFor(dataScope, canWrite)

	// Tab → parameter query (tab="" = semua status).
	tab := r.URL.Query().Get("tab")
	var filterStatus string
	var filterSLABreached, filterSLAAtRisk bool
	switch tab {
	case "baru", "diproses", "menunggu", "selesai":
		filterStatus = tab
	case "sla-risiko":
		filterSLAAtRisk = true
	case "sla-langgar":
		filterSLABreached = true
	default:
		tab = "" // normalisasi nilai liar → "" (semua)
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))

	// sort/dir (BL-157h): whitelist 6 kolom sortable (ticketSortableColumns).
	// Kombinasi tak dikenal → jatuh ke jalur default (created_at DESC), TAK
	// error — URL lama/disunting tetap render halaman yang benar.
	sortCol := r.URL.Query().Get("sort")
	if !ticketSortableColumns[sortCol] {
		sortCol = ""
	}
	dir := r.URL.Query().Get("dir")
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}

	cursorAt, cursorID := pageCursor(r)

	// KPI header: cakupan scope sama persis ListTickets.
	kpis, err := h.q(ctx).CountTicketKPIs(ctx, db.CountTicketKPIsParams{
		ScopeAll: filter.ScopeAll,
		IsOwn:    filter.IsOwn,
		Uid:      &uid,
	})
	if err != nil {
		h.Log.Error("tickets: count kpis", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var items []panel.TicketRow
	var nextCursor string
	switch sortCol {
	case "village":
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
			return
		}
		shown, nc := splitPageText(rows, func(r db.ListTicketsSortByVillageRow) (string, int64) {
			return r.AccountName, r.ID
		})
		nextCursor = nc
		items = make([]panel.TicketRow, 0, len(shown))
		for _, row := range shown {
			items = append(items, ticketRowView(ticketListRowFromVillageSort(row)))
		}
	case "subject":
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
			return
		}
		shown, nc := splitPageText(rows, func(r db.ListTicketsSortBySubjectRow) (string, int64) {
			return r.Subject, r.ID
		})
		nextCursor = nc
		items = make([]panel.TicketRow, 0, len(shown))
		for _, row := range shown {
			items = append(items, ticketRowView(ticketListRowFromSubjectSort(row)))
		}
	case "priority":
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
			return
		}
		shown, nc := splitPageText(rows, func(r db.ListTicketsSortByPriorityRow) (string, int64) {
			return r.Priority, r.ID
		})
		nextCursor = nc
		items = make([]panel.TicketRow, 0, len(shown))
		for _, row := range shown {
			items = append(items, ticketRowView(ticketListRowFromPrioritySort(row)))
		}
	case "status":
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
			return
		}
		shown, nc := splitPageText(rows, func(r db.ListTicketsSortByStatusRow) (string, int64) {
			return r.Status, r.ID
		})
		nextCursor = nc
		items = make([]panel.TicketRow, 0, len(shown))
		for _, row := range shown {
			items = append(items, ticketRowView(ticketListRowFromStatusSort(row)))
		}
	case "agent":
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
			return
		}
		shown, nc := splitPageTextNullable(rows, func(r db.ListTicketsSortByAgentRow) (string, int64, bool) {
			if r.AssignedToName == nil {
				return "", r.ID, true
			}
			return *r.AssignedToName, r.ID, false
		})
		nextCursor = nc
		items = make([]panel.TicketRow, 0, len(shown))
		for _, row := range shown {
			items = append(items, ticketRowView(ticketListRowFromAgentSort(row)))
		}
	case "sla":
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
			return
		}
		shown, nc := splitPageTextNullable(rows, func(r db.ListTicketsSortBySlaRow) (string, int64, bool) {
			if !r.SlaDeadlineAt.Valid {
				return "", r.ID, true
			}
			return ticketSlaCursorStr(r.SlaDeadlineAt), r.ID, false
		})
		nextCursor = nc
		items = make([]panel.TicketRow, 0, len(shown))
		for _, row := range shown {
			items = append(items, ticketRowView(ticketListRowFromSlaSort(row)))
		}
	default:
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
			return
		}
		shown, nc := splitPage(rows, func(r db.ListTicketsRow) (pgtype.Timestamptz, int64) {
			return r.CreatedAt, r.ID
		})
		nextCursor = nc
		items = make([]panel.TicketRow, 0, len(shown))
		for _, row := range shown {
			items = append(items, ticketRowView(ticketListRowFromDefault(row)))
		}
	}

	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, "Tickets / Cases", "/tickets", panel.TicketsList(panel.TicketsListView{
		Base:       base,
		KPIs:       ticketKPIView(kpis),
		Items:      items,
		Tab:        tab,
		Query:      query,
		CanWrite:   canWrite,
		NextCursor: nextCursor,
		After:      r.URL.Query().Get("after"),
		Trail:      pageTrail(r),
		Sort:       sortCol,
		Dir:        dir,
		Err:        ticketsErrMsg(r.URL.Query().Get("err")),
		Msg:        ticketsMsg(r.URL.Query().Get("ok")),
	}))
}

// ticketSortableColumns = whitelist kolom yang boleh diminta lewat ?sort=
// (BL-157h: 6 kolom tabel Tickets). ?sort= di luar daftar ini diperlakukan
// seolah absen (jatuh ke default created_at DESC), TAK error.
var ticketSortableColumns = map[string]bool{
	"village":  true,
	"subject":  true,
	"priority": true,
	"status":   true,
	"agent":    true,
	"sla":      true,
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
