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
