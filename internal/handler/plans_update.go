package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// plans_update.go — aksi UPDATE katalog Plan + helper loadPlan/planWriteErr.
// Dipisah dari plans.go (form buat/sunting + create) agar keduanya di bawah
// ambang tipe Route/Handler (150). Aturan sama: F2 write=admin, TANPA F3
// (katalog milik workspace, RLS satu-satunya pengurung); planWriteErr
// menerjemahkan pelanggaran keunikan plan_code jadi pesan form.
// PlanUpdate — POST /w/{workspace}/plans/{id}. Menyimpan sunting PROFIL. is_active
// TAK disentuh (SetPlanActive) → status pensiun dipertahankan. plan_code kembar →
// ?err=plan_code_dup.
func (h *Handler) PlanUpdate(w http.ResponseWriter, r *http.Request) {
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

	form, errCode := parsePlanForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/plans/"+strconv.FormatInt(id, 10)+"/edit", errCode)
		return
	}

	uid := session.UserID(ctx)
	if _, err := h.q(ctx).UpdatePlan(ctx, db.UpdatePlanParams{
		PlanName:         form.PlanName,
		PlanCode:         form.PlanCode,
		Description:      form.Description,
		PlanCategory:     form.PlanCategory,
		BasePrice:        form.BasePrice,
		BillingFrequency: form.BillingFrequency,
		SetupFee:         form.SetupFee,
		Currency:         form.Currency,
		IncludedFeatures: form.IncludedFeatures,
		UpdatedBy:        &uid,
		ID:               id,
	}); err != nil {
		if code, ok := planWriteErr(err); ok {
			wsRedirect(w, r, "/plans/"+strconv.FormatInt(id, 10)+"/edit", code)
			return
		}
		h.Log.Error("plans: update", "err", err)
		wsRedirect(w, r, "/plans/"+strconv.FormatInt(id, 10)+"/edit", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "plan.update", session.TenantID(ctx), map[string]string{
		"plan_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/plans", "saved")
}

// loadPlan memuat satu plan katalog. plans TANPA F3 (milik workspace) → cukup RLS.
// Tak ada → 404. Termasuk plan pensiun (is_active=false) agar bisa disunting/
// diaktifkan kembali. Mengembalikan (plan, true) atau menulis 404/500 & (zero, false).
func (h *Handler) loadPlan(w http.ResponseWriter, r *http.Request, id int64) (db.Plan, bool) {
	p, err := h.q(r.Context()).GetPlan(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.Plan{}, false
		}
		h.Log.Error("plans: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.Plan{}, false
	}
	return p, true
}

// planWriteErr menerjemahkan galat tulis DB jadi kode form yang bisa diperbaiki
// user. Pelanggaran unik idx_plans_code (kode plan kembar) → "plan_code_dup";
// galat lain → ("", false) agar pemanggil menanganinya sbg "failed" + log. Meniru
// accountWriteErr.
func planWriteErr(err error) (string, bool) {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == sqlStateUniqueViolation &&
		pgErr.ConstraintName == "idx_plans_code" {
		return "plan_code_dup", true
	}
	return "", false
}
