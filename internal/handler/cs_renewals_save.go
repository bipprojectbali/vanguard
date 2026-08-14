package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// cs_renewals_save.go — aksi tulis Renewal Management CS (Modul 6 slice 6.6).
// Gate F2: canWriteCSRenewalsPerm (crm:renewal_mgmt write = Admin/Manager/CSM).
// F3 diwariskan loadCSRenewal: pemanggil menolak 404 bila di luar cakupan.

// CSRenewalUpdate — POST /w/{slug}/renewal-management/{id}.
// Memperbarui field AKSI CS pada satu langganan. PRG → redirect ke daftar.
func (h *Handler) CSRenewalUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canWriteCSRenewalsPerm(ctx) {
		h.renderCSRenewalsForbidden(w, r)
		return
	}

	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}

	// F3 enforcement: tolak bila di luar cakupan akun.
	if _, _, ok := h.loadCSRenewal(w, r, id); !ok {
		return
	}

	// Parse form.
	stage := r.FormValue("renewal_stage")
	risk := r.FormValue("renewal_risk")
	actionPlan := r.FormValue("renewal_action_plan")
	nextActionRaw := r.FormValue("renewal_next_action_date")
	ownerRaw := r.FormValue("renewal_owner")

	// Validasi stage (nilai kosong = hapus stage, diterima).
	if stage != "" && !isValidCSRenewalStage(stage) {
		editRedirect(w, r, id, "stage")
		return
	}

	// Validasi risk (nilai kosong = hapus risk, diterima).
	if risk != "" && !isValidCSRenewalRisk(risk) {
		editRedirect(w, r, id, "risk")
		return
	}

	// Parse tanggal tindakan berikutnya (opsional).
	nextActionDate, errCode := optDate(nextActionRaw)
	if errCode != "" {
		editRedirect(w, r, id, "date")
		return
	}

	// Parse owner ID (opsional, kosong = hapus owner).
	var ownerID *int64
	if ownerRaw != "" {
		n, err := strconv.ParseInt(ownerRaw, 10, 64)
		if err != nil || n <= 0 {
			editRedirect(w, r, id, "owner")
			return
		}
		ownerID = &n
	}

	// Siapkan pointer string untuk field nullable.
	var stagePtr, riskPtr, planPtr *string
	if stage != "" {
		stagePtr = &stage
	}
	if risk != "" {
		riskPtr = &risk
	}
	if actionPlan != "" {
		planPtr = &actionPlan
	}

	uid := session.UserID(ctx)
	if _, err := h.q(ctx).UpdateCSRenewalAction(ctx, db.UpdateCSRenewalActionParams{
		RenewalStage:          stagePtr,
		RenewalRisk:           riskPtr,
		RenewalActionPlan:     planPtr,
		RenewalNextActionDate: nextActionDate,
		RenewalOwner:          ownerID,
		UpdatedBy:             &uid,
		ID:                    id,
	}); err != nil {
		h.Log.Error("cs-renewals: update", "err", err)
		editRedirect(w, r, id, "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "cs_renewal.update", session.TenantID(ctx), map[string]string{
		"subscription_id": strconv.FormatInt(id, 10),
		"stage":           stage,
		"risk":            risk,
	})
	wsRedirectOK(w, r, "/renewal-management", "updated")
}

// editRedirect mengarahkan ulang ke form edit dengan kode error.
func editRedirect(w http.ResponseWriter, r *http.Request, id int64, errCode string) {
	wsRedirect(w, r, "/renewal-management/"+strconv.FormatInt(id, 10)+"/edit", errCode)
}
