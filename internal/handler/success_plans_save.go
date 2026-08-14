package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// success_plans_save.go — aksi tulis Success Plans (Modul 6 slice 6.3).
// Gate F2: canWriteSuccessPlansPerm (crm:success_plans write = Admin/Manager/CSM).
// F3 diwariskan dari loadSuccessPlan pada update.

// SuccessPlanCreate — POST /w/{slug}/success-plans. Buat success plan baru.
// Validasi: plan_name wajib, plan_status valid, progress 0–100. PRG → 303.
func (h *Handler) SuccessPlanCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteSuccessPlansPerm(ctx) {
		h.renderSuccessPlansForbidden(w, r)
		return
	}

	slug := slugFromRequest(r)
	base := wsPath(slug, "")

	// Parse form.
	planName := r.FormValue("plan_name")
	objective := r.FormValue("objective")
	successMetric := r.FormValue("success_metric")
	targetDateRaw := r.FormValue("target_date")
	planStatus := r.FormValue("plan_status")
	progressRaw := r.FormValue("progress")
	ownerRaw := r.FormValue("owner_csm")
	accountRaw := r.FormValue("account_id")

	// Validasi plan_name wajib.
	if planName == "" {
		http.Redirect(w, r, base+"/success-plans/new?err=name", http.StatusSeeOther)
		return
	}

	// Validasi plan_status harus ada di enum.
	if !isValidSuccessPlanStatus(planStatus) {
		http.Redirect(w, r, base+"/success-plans/new?err=status", http.StatusSeeOther)
		return
	}

	// Validasi progress 0–100.
	progress, errCode := parseProgress(progressRaw)
	if errCode != "" {
		http.Redirect(w, r, base+"/success-plans/new?err=progress", http.StatusSeeOther)
		return
	}

	// Validasi account_id wajib.
	accountID, err := strconv.ParseInt(accountRaw, 10, 64)
	if err != nil || accountID <= 0 {
		http.Redirect(w, r, base+"/success-plans/new?err=account", http.StatusSeeOther)
		return
	}

	// Parse tanggal target (opsional).
	targetDate, errCode := optDate(targetDateRaw)
	if errCode != "" {
		http.Redirect(w, r, base+"/success-plans/new?err=date", http.StatusSeeOther)
		return
	}

	// Parse owner CSM (opsional).
	ownerID, errCode := parseOptionalID(ownerRaw)
	if errCode != "" {
		http.Redirect(w, r, base+"/success-plans/new?err=owner", http.StatusSeeOther)
		return
	}

	// Pointer opsional untuk field TEXT.
	var objPtr, metricPtr *string
	if objective != "" {
		objPtr = &objective
	}
	if successMetric != "" {
		metricPtr = &successMetric
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	_, err = h.q(ctx).CreateSuccessPlan(ctx, db.CreateSuccessPlanParams{
		TenantID:      tenantID,
		AccountID:     accountID,
		PlanName:      planName,
		Objective:     objPtr,
		SuccessMetric: metricPtr,
		TargetDate:    targetDate,
		PlanStatus:    planStatus,
		Progress:      int16(progress),
		OwnerCsm:      ownerID,
		CreatedBy:     &uid,
	})
	if err != nil {
		h.Log.Error("success_plans: create", "err", err)
		http.Redirect(w, r, base+"/success-plans/new?err=failed", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, base+"/success-plans?ok=created", http.StatusSeeOther)
}

// SuccessPlanUpdate — POST /w/{slug}/success-plans/{id}. Perbarui success plan.
// F3 diwariskan loadSuccessPlan. PRG → redirect ke daftar.
func (h *Handler) SuccessPlanUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteSuccessPlansPerm(ctx) {
		h.renderSuccessPlansForbidden(w, r)
		return
	}

	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}

	// F3 enforcement: tolak bila di luar cakupan.
	if _, _, ok := h.loadSuccessPlan(w, r, id); !ok {
		return
	}

	slug := slugFromRequest(r)
	base := wsPath(slug, "")
	errBase := base + "/success-plans/" + strconv.FormatInt(id, 10) + "/edit"

	// Parse form.
	planName := r.FormValue("plan_name")
	objective := r.FormValue("objective")
	successMetric := r.FormValue("success_metric")
	targetDateRaw := r.FormValue("target_date")
	planStatus := r.FormValue("plan_status")
	progressRaw := r.FormValue("progress")
	ownerRaw := r.FormValue("owner_csm")

	// Validasi plan_name wajib.
	if planName == "" {
		http.Redirect(w, r, errBase+"?err=name", http.StatusSeeOther)
		return
	}

	// Validasi plan_status harus ada di enum.
	if !isValidSuccessPlanStatus(planStatus) {
		http.Redirect(w, r, errBase+"?err=status", http.StatusSeeOther)
		return
	}

	// Validasi progress 0–100.
	progress, errCode := parseProgress(progressRaw)
	if errCode != "" {
		http.Redirect(w, r, errBase+"?err=progress", http.StatusSeeOther)
		return
	}

	// Parse tanggal target (opsional).
	targetDate, errCode := optDate(targetDateRaw)
	if errCode != "" {
		http.Redirect(w, r, errBase+"?err=date", http.StatusSeeOther)
		return
	}

	// Parse owner CSM (opsional, kosong = hapus owner).
	ownerID, errCode := parseOptionalID(ownerRaw)
	if errCode != "" {
		http.Redirect(w, r, errBase+"?err=owner", http.StatusSeeOther)
		return
	}

	// Pointer opsional untuk TEXT nullable.
	var objPtr, metricPtr *string
	if objective != "" {
		objPtr = &objective
	}
	if successMetric != "" {
		metricPtr = &successMetric
	}

	uid := session.UserID(ctx)

	_, err := h.q(ctx).UpdateSuccessPlan(ctx, db.UpdateSuccessPlanParams{
		ID:            id,
		PlanName:      planName,
		Objective:     objPtr,
		SuccessMetric: metricPtr,
		TargetDate:    targetDate,
		PlanStatus:    planStatus,
		Progress:      int16(progress),
		OwnerCsm:      ownerID,
		UpdatedBy:     &uid,
	})
	if err != nil {
		h.Log.Error("success_plans: update", "err", err)
		http.Redirect(w, r, errBase+"?err=failed", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, base+"/success-plans?ok=updated", http.StatusSeeOther)
}

// isValidSuccessPlanStatus melaporkan apakah s adalah nilai enum plan_status yang valid.
func isValidSuccessPlanStatus(s string) bool {
	switch s {
	case "Draft", "Active", "Achieved", "At-Risk", "Cancelled":
		return true
	}
	return false
}

// parseProgress mengurai string ke int 0–100. Mengembalikan errCode="progress" bila tidak valid.
func parseProgress(s string) (int, string) {
	if s == "" {
		return 0, ""
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 100 {
		return 0, "progress"
	}
	return n, ""
}

// parseOptionalID mengurai string ID opsional. Kosong → nil (dihapus). Error → errCode="invalid".
func parseOptionalID(s string) (*int64, string) {
	if s == "" {
		return nil, ""
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n <= 0 {
		return nil, "invalid"
	}
	return &n, ""
}
