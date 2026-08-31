package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// tickets_new.go — halaman form buat tiket (TicketNew): dropdown desa/kontak/
// anggota + prefill. Dipisah dari tickets.go (const + TicketsList) agar keduanya
// di bawah ambang tipe Route/Handler (150). Satu paket; batas dropdown dibagi.
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
