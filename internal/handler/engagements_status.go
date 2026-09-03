package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// engagements_status.go — aksi EngagementUpdateStatus (ubah status engagement +
// stempel waktu). Dipisah dari engagements_save.go (guard tulis + create +
// engagementAllows) agar keduanya di bawah ambang tipe Route/Handler (150).
// EngagementUpdateStatus — POST /w/{workspace}/engagements/{id}/status. Mengubah
// status engagement + opsional outcome dan next_due_date. F3 ditegakkan via
// GetEngagement + filter.Allows.
func (h *Handler) EngagementUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireEngagementWrite(w, r) {
		return
	}
	ctx := r.Context()

	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}

	form, errCode := parseEngagementStatusForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/engagements", errCode)
		return
	}

	// Muat engagement untuk F3 — verifikasi keberadaan + kepemilikan akun.
	e, err := h.q(ctx).GetEngagement(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		h.Log.Error("engagements: get for status update", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	dataScope := session.BusinessDataScope(ctx)
	filter := db.EngagementsListFilterFor(dataScope)
	uid := session.UserID(ctx)

	// F3: filter.ScopeAll → lolos tanpa cek akun. filter.IsOwn → cek kolom
	// kepemilikan akun dari GetAccount.
	if !filter.ScopeAll {
		if !filter.IsOwn {
			// Kedua flag false → nol cakupan → 404.
			http.NotFound(w, r)
			return
		}
		acct, err := h.q(ctx).GetAccount(ctx, e.AccountID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.NotFound(w, r)
				return
			}
			h.Log.Error("engagements: get account for F3", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		// EngagementsListFilter.Allows meniru AccountsListFilter.Allows
		// (owner / assigned_csm / backup_csm union).
		if !engagementAllows(filter, uid, acct.AccountOwner, acct.AssignedCsm, acct.BackupCsm) {
			http.NotFound(w, r)
			return
		}
	}

	if _, err := h.q(ctx).UpdateEngagementStatus(ctx, db.UpdateEngagementStatusParams{
		Status:      form.Status,
		Outcome:     form.Outcome,
		NextDueDate: form.NextDue,
		ScheduledAt: form.ScheduledAt,
		UpdatedBy:   &uid,
		ID:          id,
	}); err != nil {
		h.Log.Error("engagements: update status", "err", err)
		wsRedirect(w, r, "/engagements", "failed")
		return
	}

	meta := map[string]string{
		"engagement_id": strconv.FormatInt(id, 10),
		"status":        form.Status,
	}
	// Jejak reschedule lama→baru di metadata audit (BL-30 (c)) — engagements tak
	// punya history kolom; updated_by/at hanya siapa+kapan. e.ScheduledAt = nilai
	// LAMA (dimuat sebelum update), form.ScheduledAt = nilai BARU. UTC (gotcha #14).
	if form.Status == "rescheduled" && form.ScheduledAt.Valid {
		meta["scheduled_at_old"] = e.ScheduledAt.Time.UTC().Format(time.RFC3339)
		meta["scheduled_at_new"] = form.ScheduledAt.Time.UTC().Format(time.RFC3339)
	}
	h.auditWorkspace(ctx, uid, "engagement.status_update", session.TenantID(ctx), meta)
	wsRedirectOK(w, r, "/engagements", "updated")
}
