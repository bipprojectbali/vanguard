package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// sales_activities_status.go — AKSI ubah status Task (ActivityStatus) & soft-delete
// (ActivityDelete), dipisah dari sales_activities_update.go (ActivityUpdate) demi
// ambang tipe Route/Handler (150). Package sama; gerbang requireActivityWrite identik.

// ActivityStatus — POST /w/{workspace}/activities/{id}/status. Memindah status
// Task (aksi tersendiri, cermin DealStage). Enum divalidasi allowlist di sini +
// CHECK DB jaring terakhir. Hanya Task ber-status di UI v1.
func (h *Handler) ActivityStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireActivityWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadOwnedActivity(w, r, id); !ok {
		return
	}
	idStr := strconv.FormatInt(id, 10)

	status := strings.TrimSpace(r.FormValue("status"))
	if _, valid := validTaskStatuses[status]; !valid {
		wsRedirect(w, r, "/activities/"+idStr, "activity_status")
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).UpdateActivityStatus(ctx, db.UpdateActivityStatusParams{
		Status: &status, UpdatedBy: &uid, ID: id,
	}); err != nil {
		h.Log.Error("activities: status", "err", err)
		wsRedirect(w, r, "/activities/"+idStr, "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "activity.status", session.TenantID(ctx), map[string]string{
		"activity_id": idStr, "status": status,
	})
	wsRedirectOK(w, r, "/activities/"+idStr, "status")
}

// ActivityDelete — POST /w/{workspace}/activities/{id}/delete. Soft-delete
// (reversibel di DB via deleted_at; audit mencatat siapa & kapan).
func (h *Handler) ActivityDelete(w http.ResponseWriter, r *http.Request) {
	if !h.requireActivityWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadOwnedActivity(w, r, id); !ok {
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).SoftDeleteActivity(ctx, db.SoftDeleteActivityParams{
		UpdatedBy: &uid, ID: id,
	}); err != nil {
		h.Log.Error("activities: delete", "err", err)
		wsRedirect(w, r, "/activities/"+strconv.FormatInt(id, 10), "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "activity.delete", session.TenantID(ctx), map[string]string{
		"activity_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/activities", "deleted")
}
