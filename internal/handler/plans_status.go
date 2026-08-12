package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// plans_status.go — AKSI status katalog: pensiunkan / aktifkan kembali plan.
// Dipisah dari sunting profil (plans.go) karena is_active adalah aksi bisnis
// tersendiri (SetPlanActive), bukan efek samping edit — pensiun plan menghapusnya
// dari picker quote baru tanpa mengubah quote/langganan lama (snapshot harga aman).
// Meniru DealStage (transisi status sbg aksi tersendiri). Gerbang = requirePlanWrite.

// PlanRetire — POST /w/{workspace}/plans/{id}/retire. Pensiunkan plan (is_active=
// false). Idempotent di DB; plan hilang dari ListPlans (picker) tapi tetap tampil
// di katalog kelola (ListPlansAll) ditandai badge.
func (h *Handler) PlanRetire(w http.ResponseWriter, r *http.Request) {
	h.setPlanActive(w, r, false, "plan.retire", "retired")
}

// PlanActivate — POST /w/{workspace}/plans/{id}/activate. Aktifkan kembali plan
// pensiun (is_active=true) → tampil lagi di picker quote.
func (h *Handler) PlanActivate(w http.ResponseWriter, r *http.Request) {
	h.setPlanActive(w, r, true, "plan.activate", "activated")
}

// setPlanActive = jalur bersama pensiun/aktifkan: gate tulis → parse id → muat
// (menegakkan keberadaan/RLS) → SetPlanActive → audit → 303. active menentukan
// pensiun (false) / aktifkan (true); auditAction & okCode dioper pemanggil agar
// jejak & pesan spesifik per aksi.
func (h *Handler) setPlanActive(w http.ResponseWriter, r *http.Request, active bool, auditAction, okCode string) {
	if !h.requirePlanWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadPlan(w, r, id); !ok {
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).SetPlanActive(ctx, db.SetPlanActiveParams{
		IsActive:  active,
		UpdatedBy: &uid,
		ID:        id,
	}); err != nil {
		h.Log.Error("plans: set active", "err", err, "active", active)
		wsRedirect(w, r, "/plans", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, auditAction, session.TenantID(ctx), map[string]string{
		"plan_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/plans", okCode)
}
