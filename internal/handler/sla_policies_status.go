package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// sla_policies_status.go — AKSI status katalog: pensiunkan / aktifkan kembali
// kebijakan SLA. Dipisah dari sunting profil (sla_policies.go) karena
// is_active adalah aksi bisnis tersendiri (SetSLAPolicyActive), bukan efek
// samping edit — pensiun kebijakan menghapusnya dari picker tiket baru tanpa
// mengubah tiket lama. Meniru plans_status.go. Gerbang = requireSLAPolicyWrite.

// SLAPolicyRetire — POST /w/{workspace}/sla-policies/{id}/retire. Pensiunkan
// kebijakan (is_active=false). Idempotent di DB; kebijakan hilang dari
// ListSLAPolicies (picker) tapi tetap tampil di katalog kelola
// (ListSLAPoliciesAll) ditandai badge.
func (h *Handler) SLAPolicyRetire(w http.ResponseWriter, r *http.Request) {
	h.setSLAPolicyActive(w, r, false, "sla_policy.retire", "retired")
}

// SLAPolicyActivate — POST /w/{workspace}/sla-policies/{id}/activate.
// Aktifkan kembali kebijakan pensiun (is_active=true) → tampil lagi di
// picker tiket.
func (h *Handler) SLAPolicyActivate(w http.ResponseWriter, r *http.Request) {
	h.setSLAPolicyActive(w, r, true, "sla_policy.activate", "activated")
}

// setSLAPolicyActive = jalur bersama pensiun/aktifkan: gate tulis → parse id
// → muat (menegakkan keberadaan/RLS) → SetSLAPolicyActive → audit → 303.
// active menentukan pensiun (false) / aktifkan (true); auditAction & okCode
// dioper pemanggil agar jejak & pesan spesifik per aksi.
func (h *Handler) setSLAPolicyActive(w http.ResponseWriter, r *http.Request, active bool, auditAction, okCode string) {
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

	uid := session.UserID(ctx)
	if err := h.q(ctx).SetSLAPolicyActive(ctx, db.SetSLAPolicyActiveParams{
		IsActive:  active,
		UpdatedBy: &uid,
		ID:        id,
	}); err != nil {
		h.Log.Error("sla_policies: set active", "err", err, "active", active)
		wsRedirect(w, r, "/sla-policies", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, auditAction, session.TenantID(ctx), map[string]string{
		"sla_policy_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/sla-policies", okCode)
}
