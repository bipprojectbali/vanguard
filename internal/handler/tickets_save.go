package handler

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// tickets_save.go — aksi tulis Tickets / Cases: buat tiket baru + ubah status.
// Halaman baca & form kosong di tickets.go. Gerbang tulis: canWriteTickets (F2
// write = Support, Manager, Admin). CSM/Sales (F2 read only) diblokir di sini.
//
// F3 pada UpdateTicketStatus: GetTicket + filter.Allows memastikan aktor
// hanya mengubah tiket yang ia lihat (fail-closed: filter.Allows memeriksa
// kepemilikan SAMA dengan ListTickets, satu sumber kebenaran).
//
// SLA snapshot: sla_deadline_at = now() + resolutionMinutes — dihitung handler
// bukan SQL agar SLA policy bisa di-fetch + validasi lebih dulu.

// requireTicketWrite = gerbang tulis bersama. menulis 403 bila tak berhak.
func (h *Handler) requireTicketWrite(w http.ResponseWriter, r *http.Request) bool {
	if !canWriteTickets(r.Context()) {
		h.renderTicketsForbidden(w, r)
		return false
	}
	return true
}

// TicketCreate — POST /w/{workspace}/tickets. Membuat tiket baru. Status lahir
// 'baru'. SLA deadline di-snapshot saat ini bila SLA policy dipilih.
func (h *Handler) TicketCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requireTicketWrite(w, r) {
		return
	}
	ctx := r.Context()

	form, errCode := parseTicketForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/tickets/new", errCode)
		return
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	// Hitung sla_deadline_at: fetch SLA policy terlebih dahulu bila dipilih.
	var slaDeadline pgtype.Timestamptz // nil/invalid = tanpa SLA
	if form.SlaPolicyID != nil {
		p, err := h.q(ctx).GetSLAPolicy(ctx, *form.SlaPolicyID)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			h.Log.Error("tickets: get sla_policy", "err", err)
			wsRedirect(w, r, "/tickets/new", "failed")
			return
		}
		if err == nil && p.ResolutionTargetMinutes != nil && *p.ResolutionTargetMinutes > 0 {
			// Snapshot: now + menit resolusi. Waktu server, bukan waktu klien.
			// (Gotcha #14: simpan UTC; AT TIME ZONE hanya di agregasi laporan.)
			deadline := time.Now().UTC().Add(time.Duration(*p.ResolutionTargetMinutes) * time.Minute)
			slaDeadline = pgtype.Timestamptz{Time: deadline, Valid: true}
		}
	}

	t, err := h.q(ctx).CreateTicket(ctx, db.CreateTicketParams{
		TenantID:      tenantID,
		AccountID:     form.AccountID,
		Subject:       form.Subject,
		Description:   form.Description,
		Priority:      form.Priority,
		AssignedTo:    form.AssignedTo,
		SlaPolicyID:   form.SlaPolicyID,
		SlaDeadlineAt: slaDeadline,
		CreatedBy:     &uid,
	})
	if err != nil {
		h.Log.Error("tickets: create", "err", err)
		wsRedirect(w, r, "/tickets/new", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "ticket.create", tenantID, map[string]string{
		"ticket_id": strconv.FormatInt(t.ID, 10),
	})
	wsRedirectOK(w, r, "/tickets", "created")
}

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
