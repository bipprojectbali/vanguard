package handler

import (
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// cs_trainings_new.go — form KOSONG create Training Schedule (Modul 6 Customer
// Success, Onboarding 6.2.1.2). Dipisah dari cs_trainings.go (halaman daftar)
// agar tiap file di bawah ambang Route/Handler (150). Submit ditangani di
// cs_trainings_save.go; gerbang F2 (crm:journey) & F3 ownership sama.
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
		accountOpts = append(accountOpts, panel.CSTrainingAccountOption{ID: a.ID, Name: accountPickerLabel(a.VillageCode, a.VillageName)})
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
