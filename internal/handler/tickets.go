package handler

import (
	"net/http"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
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
//
// Fungsi sort per-kolom (village/subject/priority/default di sort_a.go,
// status/agent/sla di sort_b.go) dipisah krn ambang File Health yang sama.

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
	var ok bool
	switch sortCol {
	case "village":
		items, nextCursor, ok = h.ticketsSortByVillage(w, r, ctx, filter, uid, filterStatus, filterSLABreached, filterSLAAtRisk, query, dir)
	case "subject":
		items, nextCursor, ok = h.ticketsSortBySubject(w, r, ctx, filter, uid, filterStatus, filterSLABreached, filterSLAAtRisk, query, dir)
	case "priority":
		items, nextCursor, ok = h.ticketsSortByPriority(w, r, ctx, filter, uid, filterStatus, filterSLABreached, filterSLAAtRisk, query, dir)
	case "status":
		items, nextCursor, ok = h.ticketsSortByStatus(w, r, ctx, filter, uid, filterStatus, filterSLABreached, filterSLAAtRisk, query, dir)
	case "agent":
		items, nextCursor, ok = h.ticketsSortByAgent(w, r, ctx, filter, uid, filterStatus, filterSLABreached, filterSLAAtRisk, query, dir)
	case "sla":
		items, nextCursor, ok = h.ticketsSortBySla(w, r, ctx, filter, uid, filterStatus, filterSLABreached, filterSLAAtRisk, query, dir)
	default:
		items, nextCursor, ok = h.ticketsSortDefault(w, r, ctx, filter, uid, filterStatus, filterSLABreached, filterSLAAtRisk, query)
	}
	if !ok {
		return
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
