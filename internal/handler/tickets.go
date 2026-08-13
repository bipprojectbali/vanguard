package handler

import (
	"net/http"

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
		CanWrite:   canWrite,
		NextCursor: nextCursor,
		Err:        ticketsErrMsg(r.URL.Query().Get("err")),
		Msg:        ticketsMsg(r.URL.Query().Get("ok")),
	}))
}

// TicketNew — GET /w/{workspace}/tickets/new. Form kosong untuk tiket baru.
// Hanya canWriteTickets → F2 write = Support, Manager, Admin. CSM/Sales (read
// only) tidak bisa membuat tiket.
func (h *Handler) TicketNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteTickets(ctx) {
		h.renderTicketsForbidden(w, r)
		return
	}

	base := wsPath(slugFromRequest(r), "")

	// Accounts dropdown: semua desa aktif tanpa filter ownership — ticket writer
	// bisa membuat tiket untuk desa manapun di workspace.
	at, cid := firstPageCursor()
	accts, err := h.q(ctx).ListAccounts(ctx, db.ListAccountsParams{
		CursorCreatedAt: at, CursorID: cid,
		ScopeAll: true, PageSize: ticketsDropdownLimit,
	})
	if err != nil {
		h.Log.Error("tickets: list accounts for form", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	accountOpts := make([]panel.TicketAccountOption, 0, len(accts))
	for _, a := range accts {
		accountOpts = append(accountOpts, panel.TicketAccountOption{ID: a.ID, Name: a.VillageName})
	}

	// Members dropdown: semua anggota workspace (untuk dropdown agen).
	uid := session.UserID(ctx)
	memberRows, err := h.q(ctx).ListMembersByTenant(ctx, session.TenantID(ctx))
	if err != nil {
		h.Log.Error("tickets: list members for form", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	_ = uid // dipakai di tickets_save.go; ditahan agar konsisten
	memberOpts := make([]panel.TicketMemberOption, 0, len(memberRows))
	for _, m := range memberRows {
		name := m.Email
		if m.Name != nil && *m.Name != "" {
			name = *m.Name
		}
		memberOpts = append(memberOpts, panel.TicketMemberOption{ID: m.UserID, Name: name})
	}

	// SLA policies dropdown: hanya aktif.
	slaPolicies, err := h.q(ctx).ListSLAPolicies(ctx)
	if err != nil {
		h.Log.Error("tickets: list sla_policies for form", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	slaopts := make([]panel.TicketSLAOption, 0, len(slaPolicies))
	for _, p := range slaPolicies {
		slaopts = append(slaopts, panel.TicketSLAOption{ID: p.ID, Name: p.SlaName})
	}

	h.renderWorkspaceShell(w, r, "Tiket Baru", "/tickets", panel.TicketForm(panel.TicketFormView{
		Base:        base,
		Action:      base + "/tickets",
		Err:         ticketsErrMsg(r.URL.Query().Get("err")),
		Accounts:    accountOpts,
		Members:     memberOpts,
		SLAPolicies: slaopts,
		Priorities:  ticketPriorityValues,
	}))
}
