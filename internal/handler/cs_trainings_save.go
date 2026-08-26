package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// cs_trainings_save.go — aksi tulis Training Schedule: buat jadwal baru +
// ubah status. Halaman baca & form kosong di cs_trainings.go.
// Gerbang tulis: canWriteTrainings (F2 write via "crm:journey" = Admin,
// Manager, CSM).
//
// F3 pada CSTrainingUpdateStatus: GetCSTraining + filter.Allows() memastikan
// aktor hanya mengubah training yang ia lihat (fail-closed, satu sumber
// kebenaran dengan ListCSTrainings). Sama pola cs_impl_tasks_save.go —
// Allows() adalah method CSTrainingsListFilter (ownership_cs_onboarding.go).

// requireCSTrainingWrite = gerbang tulis bersama. Menulis 403 bila tak berhak.
func (h *Handler) requireCSTrainingWrite(w http.ResponseWriter, r *http.Request) bool {
	if !canWriteTrainings(r.Context()) {
		h.renderTrainingsForbidden(w, r)
		return false
	}
	return true
}

// CSTrainingCreate — POST /w/{workspace}/trainings. Membuat jadwal training
// baru. Status lahir 'scheduled'.
func (h *Handler) CSTrainingCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requireCSTrainingWrite(w, r) {
		return
	}
	ctx := r.Context()

	form, errCode := parseCSTrainingForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/trainings/new", errCode)
		return
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	t, err := h.q(ctx).CreateCSTraining(ctx, db.CreateCSTrainingParams{
		TenantID:       tenantID,
		AccountID:      form.AccountID,
		TrainingTopic:  form.TrainingTopic,
		TrainingDate:   form.TrainingDate,
		TrainerID:      form.TrainerID,
		Participants:   form.Participants,
		TrainingStatus: "scheduled",
		CreatedBy:      &uid,
	})
	if err != nil {
		h.Log.Error("cs_trainings: create", "err", err)
		wsRedirect(w, r, "/trainings/new", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "cs_training.create", tenantID, map[string]string{
		"training_id": strconv.FormatInt(t.ID, 10),
	})
	wsRedirectOK(w, r, "/trainings", "created")
}

// CSTrainingUpdateStatus — POST /w/{workspace}/trainings/{id}/status. Mengubah
// status training + opsional attendance dan participants. F3 ditegakkan via
// GetCSTraining + filter.Allows().
func (h *Handler) CSTrainingUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireCSTrainingWrite(w, r) {
		return
	}
	ctx := r.Context()

	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}

	status, attendance, participants, errCode := parseCSTrainingStatusForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/trainings", errCode)
		return
	}

	// Muat training untuk F3 — verifikasi keberadaan + kepemilikan akun.
	t, err := h.q(ctx).GetCSTraining(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		h.Log.Error("cs_trainings: get for status update", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	dataScope := session.BusinessDataScope(ctx)
	filter := db.CSTrainingsListFilterFor(dataScope)
	uid := session.UserID(ctx)

	if !filter.ScopeAll {
		if !filter.IsOwn {
			http.NotFound(w, r)
			return
		}
		acct, err := h.q(ctx).GetAccount(ctx, t.AccountID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.NotFound(w, r)
				return
			}
			h.Log.Error("cs_trainings: get account for F3", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !filter.Allows(uid, acct.AccountOwner, acct.AssignedCsm, acct.BackupCsm) {
			http.NotFound(w, r)
			return
		}
	}

	if _, err := h.q(ctx).UpdateCSTrainingStatus(ctx, db.UpdateCSTrainingStatusParams{
		TrainingStatus: status,
		Attendance:     attendance,
		Participants:   participants,
		UpdatedBy:      &uid,
		ID:             id,
	}); err != nil {
		h.Log.Error("cs_trainings: update status", "err", err)
		wsRedirect(w, r, "/trainings", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "cs_training.status_update", session.TenantID(ctx), map[string]string{
		"training_id": strconv.FormatInt(id, 10),
		"status":      status,
	})
	wsRedirectOK(w, r, "/trainings", "updated")
}
