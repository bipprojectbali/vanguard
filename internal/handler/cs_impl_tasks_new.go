package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// cs_impl_tasks_new.go — form KOSONG create Implementation Task (Modul 6
// Customer Success, Journey/Onboarding). Dipisah dari cs_impl_tasks.go (halaman
// daftar) agar tiap file di bawah ambang Route/Handler (150). Submit ditangani
// di cs_impl_tasks_save.go; gerbang F2 (crm:journey) & F3 ownership sama.
// CSImplTaskNew — GET /w/{workspace}/impl-tasks/new. Form kosong untuk task
// baru. Hanya canWriteImplTasks → Admin, Manager, CSM.
func (h *Handler) CSImplTaskNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteImplTasks(ctx) {
		h.renderImplTasksForbidden(w, r)
		return
	}

	slug := slugFromRequest(r)
	base := wsPath(slug, "")
	dataScope := session.BusinessDataScope(ctx)
	uid := session.UserID(ctx)
	filter := db.CSImplTasksListFilterFor(dataScope)

	// Accounts dropdown: scope user sendiri (CSM hanya desanya, Admin/Manager
	// semua) — sama pola engagements.
	at, cid := firstPageCursor()
	accts, err := h.q(ctx).ListAccounts(ctx, db.ListAccountsParams{
		CursorCreatedAt: at, CursorID: cid,
		ScopeAll: filter.ScopeAll,
		IsSales:  filter.IsOwn,
		IsCsm:    filter.IsOwn,
		Uid:      &uid,
		PageSize: csImplTasksDropdownLimit,
	})
	if err != nil {
		h.Log.Error("cs_impl_tasks: list accounts for form", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	accountOpts := make([]panel.CSImplTaskAccountOption, 0, len(accts))
	for _, a := range accts {
		accountOpts = append(accountOpts, panel.CSImplTaskAccountOption{ID: a.ID, Name: accountPickerLabel(a.VillageCode, a.VillageName)})
	}

	// Members dropdown: untuk owner_id (opsional, penanggung jawab task).
	memberRows, err := h.q(ctx).ListMembersByTenant(ctx, session.TenantID(ctx))
	if err != nil {
		h.Log.Error("cs_impl_tasks: list members for form", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	memberOpts := make([]panel.CSImplTaskMemberOption, 0, len(memberRows))
	for _, m := range memberRows {
		name := m.Email
		if m.Name != nil && *m.Name != "" {
			name = *m.Name
		}
		memberOpts = append(memberOpts, panel.CSImplTaskMemberOption{ID: m.UserID, Name: name})
	}

	h.renderWorkspaceShell(w, r, "Task Implementasi Baru", "/impl-tasks", panel.CSImplTaskForm(panel.CSImplTaskFormView{
		Base:     base,
		Action:   base + "/impl-tasks",
		Err:      csImplTasksErrMsg(r.URL.Query().Get("err")),
		Accounts: accountOpts,
		Members:  memberOpts,
	}))
}
