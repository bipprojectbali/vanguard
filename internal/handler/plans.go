package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// plans.go — AKSI atas katalog Plan: form buat/sunting, create, update. Pensiun/
// aktifkan (SetPlanActive) di plans_status.go; halaman baca di plans_page.go.
// Dipisah karena aksi tumbuh dengan aturan TULIS (F2 write = admin), halaman
// dengan aturan LIHAT (F2 read). Meniru sales_deals.go.
//
// Katalog master milik WORKSPACE (bukan per-desa) → TANPA F3 ownership: RLS
// (h.q ber-tenant) satu-satunya pengurung. plan TANPA entity_code & TANPA
// soft-delete (is_active=false = pensiun). Keunikan plan_code ditegakkan DB
// (idx_plans_code) → planWriteErr menerjemahkan pelanggaran jadi pesan form.

// requirePlanWrite = gerbang tulis bersama. false & menulis penolakan bila aktor
// tak berhak (izin F2 write; read-only workspace ditolak lebih awal dgn pesan jelas).
func (h *Handler) requirePlanWrite(w http.ResponseWriter, r *http.Request) bool {
	if !canWritePlansPerm(r.Context()) {
		h.renderPlansForbidden(w, r)
		return false
	}
	return true
}

// PlanNew — GET /w/{workspace}/plans/new. Form kosong untuk plan baru.
func (h *Handler) PlanNew(w http.ResponseWriter, r *http.Request) {
	if !h.requirePlanWrite(w, r) {
		return
	}
	base := wsPath(slugFromRequest(r), "")
	v := panel.PlanFormView{
		Base:           base,
		Action:         base + "/plans",
		IsEdit:         false,
		Err:            plansErrMsg(r.URL.Query().Get("err")),
		Fields:         panel.PlanFormFields{Currency: defaultPlanCurrency},
		Categories:     planCategoryOptions,
		BillingOptions: billingFrequencyOptions,
	}
	h.renderWorkspaceShell(w, r, "Tambah Plan", "/plans", panel.PlanForm(v))
}

// PlanCreate — POST /w/{workspace}/plans. Membuat plan katalog. Lahir is_active=true
// (aktif & langsung tampil di picker quote). tenant_id dari sesi (RLS WITH CHECK
// memverifikasinya = GUC). plan_code kembar → planWriteErr → ?err=plan_code_dup.
func (h *Handler) PlanCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requirePlanWrite(w, r) {
		return
	}
	ctx := r.Context()
	form, errCode := parsePlanForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/plans/new", errCode)
		return
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	p, err := h.q(ctx).CreatePlan(ctx, db.CreatePlanParams{
		TenantID:         tenantID,
		PlanName:         form.PlanName,
		PlanCode:         form.PlanCode,
		Description:      form.Description,
		PlanCategory:     form.PlanCategory,
		IsActive:         true, // plan baru langsung aktif (bisa dipensiunkan kemudian)
		BasePrice:        form.BasePrice,
		BillingFrequency: form.BillingFrequency,
		SetupFee:         form.SetupFee,
		Currency:         form.Currency,
		IncludedFeatures: form.IncludedFeatures,
		CreatedBy:        &uid,
	})
	if err != nil {
		if code, ok := planWriteErr(err); ok {
			wsRedirect(w, r, "/plans/new", code)
			return
		}
		h.Log.Error("plans: create", "err", err)
		wsRedirect(w, r, "/plans/new", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "plan.create", tenantID, map[string]string{
		"plan_id": strconv.FormatInt(p.ID, 10), "code": form.PlanCode,
	})
	wsRedirectOK(w, r, "/plans", "created")
}

// PlanEdit — GET /w/{workspace}/plans/{id}/edit. Form terisi profil plan (bukan
// is_active — pensiun/aktifkan jalur tersendiri).
func (h *Handler) PlanEdit(w http.ResponseWriter, r *http.Request) {
	if !h.requirePlanWrite(w, r) {
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	p, ok := h.loadPlan(w, r, id)
	if !ok {
		return
	}

	base := wsPath(slugFromRequest(r), "")
	v := panel.PlanFormView{
		Base:           base,
		Action:         base + "/plans/" + strconv.FormatInt(p.ID, 10),
		IsEdit:         true,
		Err:            plansErrMsg(r.URL.Query().Get("err")),
		Fields:         planFormFields(p),
		Categories:     planCategoryOptions,
		BillingOptions: billingFrequencyOptions,
	}
	h.renderWorkspaceShell(w, r, "Sunting Plan", "/plans", panel.PlanForm(v))
}

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
