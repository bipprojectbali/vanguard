package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// engagements_save.go — aksi tulis Engagements / Check-ins: buat engagement baru
// + ubah status. Halaman baca & form kosong di engagements.go.
// Gerbang tulis: canWriteEngagements (F2 write = Admin, Manager, CSM).
//
// F3 pada UpdateEngagementStatus: GetEngagement + filter.Allows memastikan aktor
// hanya mengubah engagement yang ia lihat (fail-closed, satu sumber kebenaran
// dengan ListEngagements).

// requireEngagementWrite = gerbang tulis bersama. Menulis 403 bila tak berhak.
func (h *Handler) requireEngagementWrite(w http.ResponseWriter, r *http.Request) bool {
	if !canWriteEngagements(r.Context()) {
		h.renderEngagementsForbidden(w, r)
		return false
	}
	return true
}

// EngagementCreate — POST /w/{workspace}/engagements. Membuat engagement baru.
// Status lahir 'planned'.
func (h *Handler) EngagementCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requireEngagementWrite(w, r) {
		return
	}
	ctx := r.Context()

	form, errCode := parseEngagementForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/engagements/new", errCode)
		return
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	e, err := h.q(ctx).CreateEngagement(ctx, db.CreateEngagementParams{
		TenantID:       tenantID,
		AccountID:      form.AccountID,
		Subject:        form.Subject,
		EngagementType: form.EngagementType,
		Frequency:      form.Frequency,
		ScheduledAt:    form.ScheduledAt,
		Status:         form.Status,
		Channel:        form.Channel,
		Outcome:        form.Outcome,
		NextDueDate:    form.NextDueDate,
		OwnerID:        form.OwnerID,
		CreatedBy:      &uid,
	})
	if err != nil {
		h.Log.Error("engagements: create", "err", err)
		wsRedirect(w, r, "/engagements/new", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "engagement.create", tenantID, map[string]string{
		"engagement_id": strconv.FormatInt(e.ID, 10),
	})
	wsRedirectOK(w, r, "/engagements", "created")
}

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

	status, outcome, nextDue, errCode := parseEngagementStatusForm(r.FormValue)
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
		Status:      status,
		Outcome:     outcome,
		NextDueDate: nextDue,
		UpdatedBy:   &uid,
		ID:          id,
	}); err != nil {
		h.Log.Error("engagements: update status", "err", err)
		wsRedirect(w, r, "/engagements", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "engagement.status_update", session.TenantID(ctx), map[string]string{
		"engagement_id": strconv.FormatInt(id, 10),
		"status":        status,
	})
	wsRedirectOK(w, r, "/engagements", "updated")
}

// engagementAllows melaporkan apakah aktor (uid) boleh MELIHAT/MENGUBAH satu
// engagement — cermin filter.Allows dari TicketsListFilter. Inline di sini
// karena EngagementsListFilter belum punya method Allows tersendiri (ukuran
// ownership.go sudah ketat).
func engagementAllows(f db.EngagementsListFilter, uid int64, owner, csm, backupCSM *int64) bool {
	if f.ScopeAll {
		return true
	}
	if f.IsOwn {
		if owner != nil && *owner == uid {
			return true
		}
		if csm != nil && *csm == uid {
			return true
		}
		if backupCSM != nil && *backupCSM == uid {
			return true
		}
	}
	return false
}
