package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
)

// sla_policies_update.go — aksi UPDATE kebijakan SLA + helper loadSLAPolicy.
// Dipisah dari sla_policies.go (guard tulis + form baru/sunting + create) agar
// keduanya di bawah ambang tipe Route/Handler (150). Aturan tulis & warisan
// pemuatan baris sama; satu paket.
// SLAPolicyUpdate — POST /w/{workspace}/sla-policies/{id}. Menyimpan sunting
// PROFIL. is_active TAK disentuh (SetSLAPolicyActive) → status pensiun
// dipertahankan.
func (h *Handler) SLAPolicyUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.requireSLAPolicyWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadSLAPolicy(w, r, id); !ok {
		return
	}

	form, errCode := parseSLAPolicyForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/sla-policies/"+strconv.FormatInt(id, 10)+"/edit", errCode)
		return
	}

	uid := session.UserID(ctx)
	if _, err := h.q(ctx).UpdateSLAPolicy(ctx, db.UpdateSLAPolicyParams{
		SlaName:                    form.SLAName,
		AppliesToPriority:          form.AppliesToPriority,
		FirstResponseTargetMinutes: form.FirstResponseTargetMinutes,
		ResolutionTargetMinutes:    form.ResolutionTargetMinutes,
		BusinessHours:              form.BusinessHours,
		EscalationRule:             form.EscalationRule,
		UpdatedBy:                  &uid,
		ID:                         id,
	}); err != nil {
		h.Log.Error("sla_policies: update", "err", err)
		wsRedirect(w, r, "/sla-policies/"+strconv.FormatInt(id, 10)+"/edit", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "sla_policy.update", session.TenantID(ctx), map[string]string{
		"sla_policy_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/sla-policies", "saved")
}

// loadSLAPolicy memuat satu kebijakan SLA katalog. sla_policies TANPA F3
// (milik workspace) → cukup RLS. Tak ada → 404. Termasuk kebijakan pensiun
// (is_active=false) agar bisa disunting/diaktifkan kembali. Mengembalikan
// (policy, true) atau menulis 404/500 & (zero, false).
func (h *Handler) loadSLAPolicy(w http.ResponseWriter, r *http.Request, id int64) (db.SlaPolicy, bool) {
	p, err := h.q(r.Context()).GetSLAPolicy(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.SlaPolicy{}, false
		}
		h.Log.Error("sla_policies: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.SlaPolicy{}, false
	}
	return p, true
}
