package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// success_plans_forms.go — handler render form Success Plan (New & Edit, GET).
// Dipisah dari daftar di success_plans.go agar tiap file di bawah ambang tipe
// Route/Handler (150); aksi tulis tetap di success_plans_save.go. Satu paket.

// SuccessPlanNew — GET /w/{workspace}/success-plans/new. Form kosong untuk
// plan baru. Hanya canWriteSuccessPlans → Admin, Manager, CSM.
func (h *Handler) SuccessPlanNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteSuccessPlans(ctx) {
		h.renderSuccessPlansForbidden(w, r)
		return
	}

	slug := slugFromRequest(r)
	base := wsPath(slug, "")
	dataScope := session.BusinessDataScope(ctx)
	uid := session.UserID(ctx)
	filter := db.SuccessPlansListFilterFor(dataScope)

	// Accounts dropdown: scope user sendiri (CSM hanya melihat desanya).
	at, cid := firstPageCursor()
	accts, err := h.q(ctx).ListAccounts(ctx, db.ListAccountsParams{
		CursorCreatedAt: at, CursorID: cid,
		ScopeAll: filter.ScopeAll,
		IsSales:  filter.IsOwn,
		IsCsm:    filter.IsOwn,
		Uid:      &uid,
		PageSize: successPlansDropdownLimit,
	})
	if err != nil {
		h.Log.Error("success_plans: list accounts for form", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	accountOpts := make([]panel.SuccessPlanAccountOption, 0, len(accts))
	for _, a := range accts {
		accountOpts = append(accountOpts, panel.SuccessPlanAccountOption{ID: a.ID, Name: accountPickerLabel(a.VillageCode, a.VillageName)})
	}

	// Members dropdown: owner_csm (opsional).
	memberOpts, err := h.successPlanMemberOptions(ctx)
	if err != nil {
		h.Log.Error("success_plans: list members for form", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.renderWorkspaceShell(w, r, "Buat Success Plan", "/success-plans", panel.SuccessPlanForm(panel.SuccessPlanFormView{
		Base:     base,
		Action:   base + "/success-plans",
		Err:      successPlansErrMsg(r.URL.Query().Get("err")),
		Accounts: accountOpts,
		Members:  memberOpts,
		Statuses: successPlanStatusValues,
	}))
}

// SuccessPlanEdit — GET /w/{workspace}/success-plans/{id}/edit. Form isi untuk
// edit plan. F3 ditegakkan via loadSuccessPlan.
func (h *Handler) SuccessPlanEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteSuccessPlans(ctx) {
		h.renderSuccessPlansForbidden(w, r)
		return
	}

	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}

	sp, acct, ok := h.loadSuccessPlan(w, r, id)
	if !ok {
		return
	}

	slug := slugFromRequest(r)
	base := wsPath(slug, "")

	memberOpts, err := h.successPlanMemberOptions(ctx)
	if err != nil {
		h.Log.Error("success_plans: list members for edit form", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.renderWorkspaceShell(w, r, "Edit Success Plan", "/success-plans", panel.SuccessPlanForm(panel.SuccessPlanFormView{
		Base:     base,
		Action:   base + "/success-plans/" + strconv.FormatInt(id, 10),
		Err:      successPlansErrMsg(r.URL.Query().Get("err")),
		Members:  memberOpts,
		Statuses: successPlanStatusValues,

		// Data akun (read-only di form edit — tampilkan saja, tanpa dropdown desa).
		AccountName: accountPickerLabel(acct.VillageCode, acct.VillageName),

		// Nilai saat ini.
		CurrentPlanName:      sp.PlanName,
		CurrentObjective:     deref(sp.Objective),
		CurrentSuccessMetric: deref(sp.SuccessMetric),
		CurrentTargetDate:    dateStr(sp.TargetDate),
		CurrentStatus:        sp.PlanStatus,
		CurrentProgress:      int(sp.Progress),
		CurrentOwnerCsmID:    sp.OwnerCsm,
	}))
}
