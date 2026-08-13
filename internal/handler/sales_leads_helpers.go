package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// sales_leads_helpers.go — AKSI hapus Lead + helper bersama (F3 loader, prefill
// form) dipakai sales_leads.go/sales_leads_update.go/sales_leads_page.go/
// sales_leads_detail.go. Dipecah dari sales_leads.go semata untuk file health.

// LeadDelete — POST /w/{workspace}/leads/{id}/delete. Soft-delete (reversibel di
// DB via deleted_at; jejak audit mencatat siapa & kapan).
func (h *Handler) LeadDelete(w http.ResponseWriter, r *http.Request) {
	if !h.requireLeadWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadOwnedLead(w, r, id); !ok {
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).SoftDeleteLead(ctx, db.SoftDeleteLeadParams{
		UpdatedBy: &uid, ID: id,
	}); err != nil {
		h.Log.Error("leads: delete", "err", err)
		wsRedirect(w, r, "/leads/"+strconv.FormatInt(id, 10), "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "lead.delete", session.TenantID(ctx), map[string]string{
		"lead_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/leads", "deleted")
}

// loadOwnedLead memuat satu lead & menegakkan F3: di luar cakupan aktor → 404
// (kembaran LeadDetail; "tampil di daftar" & "boleh disentuh" satu jawaban).
// Mengembalikan (lead, true) bila boleh, atau menulis 404/500 & (zero, false).
func (h *Handler) loadOwnedLead(w http.ResponseWriter, r *http.Request, id int64) (db.Lead, bool) {
	ctx := r.Context()
	l, err := h.q(ctx).GetLead(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.Lead{}, false
		}
		h.Log.Error("leads: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.Lead{}, false
	}
	filter := db.LeadsListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), l.LeadOwner) {
		http.NotFound(w, r)
		return db.Lead{}, false
	}
	return l, true
}

// leadFormFields memetakan Lead → nilai prefill form (semua string; nil → "").
func leadFormFields(l db.Lead) panel.LeadFormFields {
	return panel.LeadFormFields{
		LeadName:          l.LeadName,
		ContactPerson:     deref(l.ContactPerson),
		JobTitle:          deref(l.JobTitle),
		LeadSource:        deref(l.LeadSource),
		LeadStatus:        l.LeadStatus,
		Rating:            deref(l.Rating),
		UnqualifiedReason: deref(l.UnqualifiedReason),
		EstimatedValue:    numericStr(l.EstimatedValue),
		Province:          deref(l.Province),
		Regency:           deref(l.Regency),
		District:          deref(l.District),
		MobilePhone:       deref(l.MobilePhone),
		Whatsapp:          deref(l.Whatsapp),
		Email:             deref(l.Email),
	}
}
