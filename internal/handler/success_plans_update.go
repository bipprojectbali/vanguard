package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// success_plans_update.go — aksi tulis UPDATE Success Plan (Modul 6 slice 6.3).
// Dipisah dari success_plans_save.go (create + validator bersama) agar keduanya
// di bawah ambang tipe Route/Handler (150). Gate F2 & warisan F3 dari
// loadSuccessPlan sama; validator (isValidSuccessPlanStatus/parseProgress/
// parseOptionalID) tetap satu paket di success_plans_save.go.
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
