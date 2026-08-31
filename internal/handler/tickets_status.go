package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// tickets_status.go — aksi TicketUpdateStatus (ubah status/assignment tiket).
// Dipisah dari tickets_save.go (guard tulis + TicketCreate) agar keduanya di
// bawah ambang tipe Route/Handler (150). Gate tulis F2 sama; satu paket.
// TicketUpdateStatus — POST /w/{workspace}/tickets/{id}/status. Mengubah status
// tiket. Juga dapat memperbarui assigned_to (opsional, lewat hidden input
// assigned_to). resolved_at diisi otomatis oleh SQL (CASE WHEN status='selesai'
// THEN now()). F3 ditegakkan via GetTicket + filter.Allows.
func (h *Handler) TicketUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if !h.requireTicketWrite(w, r) {
		return
	}
	ctx := r.Context()

	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}

	status := r.FormValue("status")
	if !isValidEnum(status, ticketStatusValues) {
		wsRedirect(w, r, "/tickets", "status")
		return
	}

	// Muat tiket untuk F3 — verifikasi keberadaan + kepemilikan akun.
	t, err := h.q(ctx).GetTicket(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		h.Log.Error("tickets: get for status update", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	dataScope := session.BusinessDataScope(ctx)
	canWrite := canWriteTickets(ctx)
	filter := db.TicketsListFilterFor(dataScope, canWrite)
	uid := session.UserID(ctx)

	// F3: filter.ScopeAll → lolos tanpa cek akun. filter.IsOwn → perlu kolom
	// kepemilikan akun (account_owner/assigned_csm/backup_csm) yang tidak ada
	// di GetTicketRow — fetch akun terpisah.
	if !filter.ScopeAll {
		if !filter.IsOwn {
			// Kedua flag false → nol cakupan → 404.
			http.NotFound(w, r)
			return
		}
		acct, err := h.q(ctx).GetAccount(ctx, t.AccountID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				http.NotFound(w, r)
				return
			}
			h.Log.Error("tickets: get account for F3", "err", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if !filter.Allows(uid, acct.AccountOwner, acct.AssignedCsm, acct.BackupCsm) {
			// Di luar cakupan kepemilikan → 404 (jangan beri tahu keberadaan).
			http.NotFound(w, r)
			return
		}
	}

	// assigned_to opsional — form bisa mengirim atau tidak.
	var assignedTo *int64
	if s := r.FormValue("assigned_to"); s != "" {
		if aid, err := strconv.ParseInt(s, 10, 64); err == nil && aid > 0 {
			assignedTo = &aid
		}
	}

	if _, err := h.q(ctx).UpdateTicketStatus(ctx, db.UpdateTicketStatusParams{
		Status:     status,
		AssignedTo: assignedTo,
		UpdatedBy:  &uid,
		ID:         id,
	}); err != nil {
		h.Log.Error("tickets: update status", "err", err)
		wsRedirect(w, r, "/tickets", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "ticket.status_update", session.TenantID(ctx), map[string]string{
		"ticket_id": strconv.FormatInt(id, 10),
		"status":    status,
	})
	wsRedirectOK(w, r, "/tickets", "updated")
}
