package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// cs_impl_tasks.go — HALAMAN baca Implementation Tracker (Modul 6 Customer
// Success, sub-item Onboarding 6.2.1.1). Aksi tulis (create, status update)
// di cs_impl_tasks_save.go. Meniru engagements.go.
//
// Akses lewat dua sumbu orthogonal:
//   - F2 (Casbin bisnis): canViewImplTasks — REUSE objek "crm:journey"
//     (Journey/Onboarding), bukan objek baru (task ini sub-fitur onboarding
//     yang sama dgn customer_success.onboarding_*).
//   - F3 (ownership): CSImplTasksListFilterFor(dataScope) — baris mana yang
//     tampil. Admin/Manager (ScopeAll) lihat semua; CSM (ScopeOwn) lihat task
//     desa binaannya.
//   - RLS: h.q ber-tenant → mengisolasi workspace di bawah semua ini.

// csImplTasksDropdownLimit = batas opsi dropdown desa di form task baru.
// Guardrail: tak ada full-scan tanpa batas.
const csImplTasksDropdownLimit = 200

// CSImplTasksList — GET /w/{workspace}/impl-tasks. Daftar task + KPI header +
// filter tab. canViewImplTasks=false → 403.
func (h *Handler) CSImplTasksList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewImplTasks(ctx) {
		h.renderImplTasksForbidden(w, r)
		return
	}

	dataScope := session.BusinessDataScope(ctx)
	canWrite := canWriteImplTasks(ctx)
	uid := session.UserID(ctx)
	filter := db.CSImplTasksListFilterFor(dataScope)

	// Tab → filter status. Nilai liar dinormalisasi ke "" (semua).
	tab := r.URL.Query().Get("tab")
	var filterStatus string
	switch tab {
	case "to_do", "in_progress", "done", "blocked":
		filterStatus = tab
	default:
		tab = ""
	}

	cursorAt, cursorID := pageCursor(r)

	kpis, err := h.q(ctx).CountCSImplTaskKPIs(ctx, db.CountCSImplTaskKPIsParams{
		ScopeAll: filter.ScopeAll,
		IsOwn:    filter.IsOwn,
		Uid:      &uid,
	})
	if err != nil {
		h.Log.Error("cs_impl_tasks: count kpis", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rows, err := h.q(ctx).ListCSImplTasks(ctx, db.ListCSImplTasksParams{
		CursorAt:     cursorAt,
		CursorID:     cursorID,
		ScopeAll:     filter.ScopeAll,
		IsOwn:        filter.IsOwn,
		Uid:          &uid,
		FilterStatus: filterStatus,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("cs_impl_tasks: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	shown, nextCursor := splitPage(rows, func(r db.ListCSImplTasksRow) (pgtype.Timestamptz, int64) {
		return r.CreatedAt, r.ID
	})

	slug := slugFromRequest(r)
	items := make([]panel.CSImplTaskRow, 0, len(shown))
	for _, row := range shown {
		items = append(items, csImplTaskRowView(row, slug))
	}

	base := wsPath(slug, "")
	h.renderWorkspaceShell(w, r, "Implementation Tracker", "/impl-tasks", panel.CSImplTasksList(panel.CSImplTasksListView{
		Base:       base,
		KPIs:       csImplTaskKPIView(kpis),
		Items:      items,
		Tab:        tab,
		CanWrite:   canWrite,
		NextCursor: nextCursor,
		Err:        csImplTasksErrMsg(r.URL.Query().Get("err")),
		Msg:        csImplTasksMsg(r.URL.Query().Get("ok")),
	}))
}

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
		accountOpts = append(accountOpts, panel.CSImplTaskAccountOption{ID: a.ID, Name: a.VillageName})
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
