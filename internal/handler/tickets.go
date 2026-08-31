package handler

import (
	"net/http"
	"strings"

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
	case "baru", "ditugaskan", "eskalasi", "selesai":
		filterStatus = tab
	case "sla-risiko":
		filterSLAAtRisk = true
	case "sla-langgar":
		filterSLABreached = true
	default:
		tab = "" // normalisasi nilai liar → "" (semua)
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))

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

	shown, nextCursor := splitPage(rows, func(r db.ListTicketsRow) (pgtype.Timestamptz, int64) {
		return r.CreatedAt, r.ID
	})

	items := make([]panel.TicketRow, 0, len(shown))
	for _, row := range shown {
		items = append(items, ticketRowView(row))
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
		Err:        ticketsErrMsg(r.URL.Query().Get("err")),
		Msg:        ticketsMsg(r.URL.Query().Get("ok")),
	}))
}
