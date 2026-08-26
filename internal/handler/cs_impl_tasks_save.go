package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// cs_impl_tasks_save.go — aksi tulis Implementation Tracker: buat task baru +
// ubah status. Halaman baca & form kosong di cs_impl_tasks.go.
// Gerbang tulis: canWriteImplTasks (F2 write via "crm:journey" = Admin,
// Manager, CSM).
//
// F3 pada CSImplTaskUpdateStatus: GetCSImplTask + filter.Allows() memastikan
// aktor hanya mengubah task yang ia lihat (fail-closed, satu sumber kebenaran
// dengan ListCSImplTasks). Berbeda dari Engagements: di sini Allows() adalah
// method CSImplTasksListFilter (ownership_cs_onboarding.go), bukan
// free-function terpisah.

// requireCSImplTaskWrite = gerbang tulis bersama. Menulis 403 bila tak berhak.
func (h *Handler) requireCSImplTaskWrite(w http.ResponseWriter, r *http.Request) bool {
	if !canWriteImplTasks(r.Context()) {
		h.renderImplTasksForbidden(w, r)
		return false
	}
	return true
}

// CSImplTaskCreate — POST /w/{workspace}/impl-tasks. Membuat task baru.
// Status lahir 'to_do'.
func (h *Handler) CSImplTaskCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requireCSImplTaskWrite(w, r) {
		return
	}
	ctx := r.Context()

	form, errCode := parseCSImplTaskForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/impl-tasks/new", errCode)
		return
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	t, err := h.q(ctx).CreateCSImplTask(ctx, db.CreateCSImplTaskParams{
		TenantID:   tenantID,
		AccountID:  form.AccountID,
		TaskName:   form.TaskName,
		TaskStatus: "to_do",
		OwnerID:    form.OwnerID,
		DueDate:    form.DueDate,
		CreatedBy:  &uid,
	})
	if err != nil {
		h.Log.Error("cs_impl_tasks: create", "err", err)
		wsRedirect(w, r, "/impl-tasks/new", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "cs_impl_task.create", tenantID, map[string]string{
		"task_id": strconv.FormatInt(t.ID, 10),
	})
	wsRedirectOK(w, r, "/impl-tasks", "created")
}

// CSImplTaskUpdateStatus — POST /w/{workspace}/impl-tasks/{id}/status. Mengubah
// status task + opsional due_date dan owner_id (reassign). F3 ditegakkan via
// GetCSImplTask + filter.Allows().
func (h *Handler) CSImplTaskUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireCSImplTaskWrite(w, r) {
		return
	}
	ctx := r.Context()

	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}

	status, dueDate, ownerID, errCode := parseCSImplTaskStatusForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/impl-tasks", errCode)
		return
	}

	// Muat task untuk F3 — verifikasi keberadaan + kepemilikan akun.
	t, err := h.q(ctx).GetCSImplTask(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		h.Log.Error("cs_impl_tasks: get for status update", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	dataScope := session.BusinessDataScope(ctx)
	filter := db.CSImplTasksListFilterFor(dataScope)
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
			h.Log.Error("cs_impl_tasks: get account for F3", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !filter.Allows(uid, acct.AccountOwner, acct.AssignedCsm, acct.BackupCsm) {
			http.NotFound(w, r)
			return
		}
	}

	if _, err := h.q(ctx).UpdateCSImplTaskStatus(ctx, db.UpdateCSImplTaskStatusParams{
		TaskStatus: status,
		DueDate:    dueDate,
		OwnerID:    ownerID,
		UpdatedBy:  &uid,
		ID:         id,
	}); err != nil {
		h.Log.Error("cs_impl_tasks: update status", "err", err)
		wsRedirect(w, r, "/impl-tasks", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "cs_impl_task.status_update", session.TenantID(ctx), map[string]string{
		"task_id": strconv.FormatInt(id, 10),
		"status":  status,
	})
	wsRedirectOK(w, r, "/impl-tasks", "updated")
}
