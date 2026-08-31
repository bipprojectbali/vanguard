package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// sla_policies.go — AKSI atas katalog SLA Policies: form buat/sunting, create,
// update. Pensiun/aktifkan (SetSLAPolicyActive) di sla_policies_status.go;
// halaman baca di sla_policies_page.go. Dipisah karena aksi tumbuh dengan
// aturan TULIS (F2 write = manager/admin), halaman dengan aturan LIHAT (F2
// read). Meniru plans.go (Modul 5).
//
// Katalog master milik WORKSPACE (bukan per-desa) → TANPA F3 ownership: RLS
// (h.q ber-tenant) satu-satunya pengurung. sla_policies TANPA entity_code &
// TANPA soft-delete (is_active=false = pensiun). TANPA unique constraint selain
// PK (beda dari plans.plan_code) → tak ada writeErr khusus, galat DB lain
// ditangani generik sebagai "failed".

// requireSLAPolicyWrite = gerbang tulis bersama. false & menulis penolakan
// bila aktor tak berhak (izin F2 write; read-only workspace ditolak lebih
// awal dgn pesan jelas).
func (h *Handler) requireSLAPolicyWrite(w http.ResponseWriter, r *http.Request) bool {
	if !canWriteSLAPoliciesPerm(r.Context()) {
		h.renderSLAPoliciesForbidden(w, r)
		return false
	}
	return true
}

// SLAPolicyNew — GET /w/{workspace}/sla-policies/new. Form kosong untuk
// kebijakan baru.
func (h *Handler) SLAPolicyNew(w http.ResponseWriter, r *http.Request) {
	if !h.requireSLAPolicyWrite(w, r) {
		return
	}
	base := wsPath(slugFromRequest(r), "")
	v := panel.SLAPolicyFormView{
		Base:            base,
		Action:          base + "/sla-policies",
		IsEdit:          false,
		Err:             slaPoliciesErrMsg(r.URL.Query().Get("err")),
		Fields:          panel.SLAPolicyFormFields{},
		Priorities:      slaPriorityOptions,
		BusinessHoursOp: slaBusinessHoursOptions,
	}
	h.renderWorkspaceShell(w, r, "Tambah Kebijakan SLA", "/sla-policies", panel.SLAPolicyForm(v))
}

// SLAPolicyCreate — POST /w/{workspace}/sla-policies. Membuat kebijakan SLA
// katalog. Lahir is_active=true (aktif & langsung tampil di picker tiket).
// tenant_id dari sesi (RLS WITH CHECK memverifikasinya = GUC).
func (h *Handler) SLAPolicyCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requireSLAPolicyWrite(w, r) {
		return
	}
	ctx := r.Context()
	form, errCode := parseSLAPolicyForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/sla-policies/new", errCode)
		return
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	p, err := h.q(ctx).CreateSLAPolicy(ctx, db.CreateSLAPolicyParams{
		TenantID:                   tenantID,
		SlaName:                    form.SLAName,
		AppliesToPriority:          form.AppliesToPriority,
		FirstResponseTargetMinutes: form.FirstResponseTargetMinutes,
		ResolutionTargetMinutes:    form.ResolutionTargetMinutes,
		BusinessHours:              form.BusinessHours,
		EscalationRule:             form.EscalationRule,
		IsActive:                   true, // kebijakan baru langsung aktif (bisa dipensiunkan kemudian)
		CreatedBy:                  &uid,
	})
	if err != nil {
		h.Log.Error("sla_policies: create", "err", err)
		wsRedirect(w, r, "/sla-policies/new", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "sla_policy.create", tenantID, map[string]string{
		"sla_policy_id": strconv.FormatInt(p.ID, 10),
	})
	wsRedirectOK(w, r, "/sla-policies", "created")
}

// SLAPolicyEdit — GET /w/{workspace}/sla-policies/{id}/edit. Form terisi
// profil kebijakan (bukan is_active — pensiun/aktifkan jalur tersendiri).
func (h *Handler) SLAPolicyEdit(w http.ResponseWriter, r *http.Request) {
	if !h.requireSLAPolicyWrite(w, r) {
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	p, ok := h.loadSLAPolicy(w, r, id)
	if !ok {
		return
	}

	base := wsPath(slugFromRequest(r), "")
	v := panel.SLAPolicyFormView{
		Base:            base,
		Action:          base + "/sla-policies/" + strconv.FormatInt(p.ID, 10),
		IsEdit:          true,
		Err:             slaPoliciesErrMsg(r.URL.Query().Get("err")),
		Fields:          slaPolicyFormFields(p),
		Priorities:      slaPriorityOptions,
		BusinessHoursOp: slaBusinessHoursOptions,
	}
	h.renderWorkspaceShell(w, r, "Sunting Kebijakan SLA", "/sla-policies", panel.SLAPolicyForm(v))
}
