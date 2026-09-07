package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
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
		Fields:         panel.PlanFormFields{}, // currency tak lagi field form (BL-90)
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
