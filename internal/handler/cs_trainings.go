package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5/pgtype"
)

// cs_trainings.go — HALAMAN baca Training Schedule (Modul 6 Customer Success,
// sub-item Onboarding 6.2.1.2). Aksi tulis (create, status update) di
// cs_trainings_save.go. Meniru cs_impl_tasks.go / engagements.go.
//
// Akses lewat dua sumbu orthogonal:
//   - F2 (Casbin bisnis): canViewTrainings — REUSE objek "crm:journey"
//     (Journey/Onboarding), bukan objek baru (sub-fitur onboarding yang sama
//     dgn customer_success.onboarding_*).
//   - F3 (ownership): CSTrainingsListFilterFor(dataScope) — baris mana yang
//     tampil. Admin/Manager (ScopeAll) lihat semua; CSM (ScopeOwn) lihat
//     training desa binaannya.
//   - RLS: h.q ber-tenant → mengisolasi workspace di bawah semua ini.

// csTrainingsDropdownLimit = batas opsi dropdown desa di form training baru.
// Guardrail: tak ada full-scan tanpa batas.
const csTrainingsDropdownLimit = 200

// CSTrainingsList — GET /w/{workspace}/trainings. Daftar training + KPI
// header + filter tab. canViewTrainings=false → 403.
func (h *Handler) CSTrainingsList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewTrainings(ctx) {
		h.renderTrainingsForbidden(w, r)
		return
	}

	dataScope := session.BusinessDataScope(ctx)
	canWrite := canWriteTrainings(ctx)
	uid := session.UserID(ctx)
	filter := db.CSTrainingsListFilterFor(dataScope)

	// Tab → filter status. Nilai liar dinormalisasi ke "" (semua).
	tab := r.URL.Query().Get("tab")
	var filterStatus string
	switch tab {
	case "scheduled", "completed", "rescheduled", "cancelled":
		filterStatus = tab
	default:
		tab = ""
	}

	cursorAt, cursorID := pageCursor(r)

	kpis, err := h.q(ctx).CountCSTrainingKPIs(ctx, db.CountCSTrainingKPIsParams{
		ScopeAll: filter.ScopeAll,
		IsOwn:    filter.IsOwn,
		Uid:      &uid,
	})
	if err != nil {
		h.Log.Error("cs_trainings: count kpis", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	rows, err := h.q(ctx).ListCSTrainings(ctx, db.ListCSTrainingsParams{
		CursorAt:     cursorAt,
		CursorID:     cursorID,
		ScopeAll:     filter.ScopeAll,
		IsOwn:        filter.IsOwn,
		Uid:          &uid,
		FilterStatus: filterStatus,
		PageSize:     pageSize + 1,
	})
	if err != nil {
		h.Log.Error("cs_trainings: list", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	shown, nextCursor := splitPage(rows, func(r db.ListCSTrainingsRow) (pgtype.Timestamptz, int64) {
		return r.TrainingDate, r.ID
	})

	slug := slugFromRequest(r)
	items := make([]panel.CSTrainingRow, 0, len(shown))
	for _, row := range shown {
		items = append(items, csTrainingRowView(row, slug))
	}

	base := wsPath(slug, "")
	h.renderWorkspaceShell(w, r, "Training Schedule", "/trainings", panel.CSTrainingsList(panel.CSTrainingsListView{
		Base:       base,
		KPIs:       csTrainingKPIView(kpis),
		Items:      items,
		Tab:        tab,
		CanWrite:   canWrite,
		NextCursor: nextCursor,
		Err:        csTrainingsErrMsg(r.URL.Query().Get("err")),
		Msg:        csTrainingsMsg(r.URL.Query().Get("ok")),
	}))
}

// CSTrainingNew — GET /w/{workspace}/trainings/new. Form kosong untuk
// training baru. Hanya canWriteTrainings → Admin, Manager, CSM.
func (h *Handler) CSTrainingNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteTrainings(ctx) {
		h.renderTrainingsForbidden(w, r)
		return
	}

	slug := slugFromRequest(r)
	base := wsPath(slug, "")
	dataScope := session.BusinessDataScope(ctx)
	uid := session.UserID(ctx)
	filter := db.CSTrainingsListFilterFor(dataScope)

	// Accounts dropdown: scope user sendiri (CSM hanya desanya, Admin/Manager
	// semua) — sama pola cs_impl_tasks/engagements.
	at, cid := firstPageCursor()
	accts, err := h.q(ctx).ListAccounts(ctx, db.ListAccountsParams{
		CursorCreatedAt: at, CursorID: cid,
		ScopeAll: filter.ScopeAll,
		IsSales:  filter.IsOwn,
		IsCsm:    filter.IsOwn,
		Uid:      &uid,
		PageSize: csTrainingsDropdownLimit,
	})
	if err != nil {
		h.Log.Error("cs_trainings: list accounts for form", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	accountOpts := make([]panel.CSTrainingAccountOption, 0, len(accts))
	for _, a := range accts {
		accountOpts = append(accountOpts, panel.CSTrainingAccountOption{ID: a.ID, Name: a.VillageName})
	}

	// Members dropdown: untuk trainer_id (opsional, pengajar training).
	memberRows, err := h.q(ctx).ListMembersByTenant(ctx, session.TenantID(ctx))
	if err != nil {
		h.Log.Error("cs_trainings: list members for form", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	trainerOpts := make([]panel.CSTrainingTrainerOption, 0, len(memberRows))
	for _, m := range memberRows {
		name := m.Email
		if m.Name != nil && *m.Name != "" {
			name = *m.Name
		}
		trainerOpts = append(trainerOpts, panel.CSTrainingTrainerOption{ID: m.UserID, Name: name})
	}

	h.renderWorkspaceShell(w, r, "Jadwal Training Baru", "/trainings", panel.CSTrainingForm(panel.CSTrainingFormView{
		Base:     base,
		Action:   base + "/trainings",
		Err:      csTrainingsErrMsg(r.URL.Query().Get("err")),
		Accounts: accountOpts,
		Trainers: trainerOpts,
	}))
}
