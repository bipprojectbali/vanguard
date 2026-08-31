package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// engagements_new.go — form KOSONG create Engagement/Check-in (Modul 6 slice
// 6.5). Dipisah dari engagements.go (halaman daftar) agar tiap file di bawah
// ambang Route/Handler (150). Submit ditangani EngagementCreate
// (engagements_save.go); gerbang F2/F3 sama dengan halaman daftar.
// EngagementNew — GET /w/{workspace}/engagements/new. Form kosong untuk
// engagement baru. Hanya canWriteEngagements → Admin, Manager, CSM.
func (h *Handler) EngagementNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteEngagements(ctx) {
		h.renderEngagementsForbidden(w, r)
		return
	}

	slug := slugFromRequest(r)
	base := wsPath(slug, "")
	dataScope := session.BusinessDataScope(ctx)
	uid := session.UserID(ctx)
	filter := db.EngagementsListFilterFor(dataScope)

	// Accounts dropdown: gunakan scope user sendiri (CSM hanya melihat desanya,
	// Admin/Manager melihat semua). Beda dari tickets yang selalu scope_all=true
	// karena ticket bisa dibuat untuk desa manapun oleh Support; engagement dibuat
	// CSM untuk desanya sendiri.
	at, cid := firstPageCursor()
	accts, err := h.q(ctx).ListAccounts(ctx, db.ListAccountsParams{
		CursorCreatedAt: at, CursorID: cid,
		ScopeAll: filter.ScopeAll,
		IsSales:  filter.IsOwn,
		IsCsm:    filter.IsOwn,
		Uid:      &uid,
		PageSize: engagementsDropdownLimit,
	})
	if err != nil {
		h.Log.Error("engagements: list accounts for form", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	accountOpts := make([]panel.EngagementAccountOption, 0, len(accts))
	for _, a := range accts {
		accountOpts = append(accountOpts, panel.EngagementAccountOption{ID: a.ID, Name: a.VillageName})
	}

	// Members dropdown: untuk owner_id dropdown (opsional).
	memberRows, err := h.q(ctx).ListMembersByTenant(ctx, session.TenantID(ctx))
	if err != nil {
		h.Log.Error("engagements: list members for form", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	memberOpts := make([]panel.EngagementMemberOption, 0, len(memberRows))
	for _, m := range memberRows {
		name := m.Email
		if m.Name != nil && *m.Name != "" {
			name = *m.Name
		}
		memberOpts = append(memberOpts, panel.EngagementMemberOption{ID: m.UserID, Name: name})
	}

	h.renderWorkspaceShell(w, r, "Engagement Baru", "/engagements", panel.EngagementForm(panel.EngagementFormView{
		Base:     base,
		Action:   base + "/engagements",
		Err:      engagementsErrMsg(r.URL.Query().Get("err")),
		Accounts: accountOpts,
		Members:  memberOpts,
		Types:    engagementTypeValues,
		Statuses: engagementStatusValues,
		Channels: engagementChannelValues,
	}))
}
