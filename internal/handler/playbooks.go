package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// playbooks.go — AKSI atas katalog Playbooks: form buat/sunting, create,
// update. Jadikan-draf/aktifkan (SetPlaybookActive) di playbooks_status.go;
// halaman baca di playbooks_page.go. Dipisah karena aksi tumbuh dengan
// aturan TULIS (F2 write = manager/csm), halaman dengan aturan LIHAT (F2
// read). Meniru sla_policies.go (slice A1).
//
// Katalog master milik WORKSPACE (bukan per-desa) → TANPA F3 ownership: RLS
// (h.q ber-tenant) satu-satunya pengurung. playbooks TANPA entity_code &
// TANPA soft-delete (is_active=false = draf). TANPA unique constraint selain
// PK → tak ada writeErr khusus, galat DB lain ditangani generik sebagai
// "failed".

// requirePlaybookWrite = gerbang tulis bersama. false & menulis penolakan
// bila aktor tak berhak (izin F2 write; read-only workspace ditolak lebih
// awal dgn pesan jelas).
func (h *Handler) requirePlaybookWrite(w http.ResponseWriter, r *http.Request) bool {
	if !canWritePlaybooksPerm(r.Context()) {
		h.renderPlaybooksForbidden(w, r)
		return false
	}
	return true
}

// PlaybookNew — GET /w/{workspace}/playbooks/new. Form kosong untuk
// playbook baru.
func (h *Handler) PlaybookNew(w http.ResponseWriter, r *http.Request) {
	if !h.requirePlaybookWrite(w, r) {
		return
	}
	base := wsPath(slugFromRequest(r), "")
	v := panel.PlaybookFormView{
		Base:              base,
		Action:            base + "/playbooks",
		IsEdit:            false,
		Err:               playbooksErrMsg(r.URL.Query().Get("err")),
		Fields:            panel.PlaybookFormFields{},
		TriggerScenarios:  playbookTriggerScenarioOptions,
		RecommendedOwners: playbookRecommendedOwnerOptions,
	}
	h.renderWorkspaceShell(w, r, "Tambah Playbook", "/playbooks", panel.PlaybookForm(v))
}

// PlaybookCreate — POST /w/{workspace}/playbooks. Membuat playbook katalog.
// Lahir is_active=true (aktif & langsung tampil di picker). tenant_id dari
// sesi (RLS WITH CHECK memverifikasinya = GUC).
func (h *Handler) PlaybookCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requirePlaybookWrite(w, r) {
		return
	}
	ctx := r.Context()
	form, errCode := parsePlaybookForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/playbooks/new", errCode)
		return
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	p, err := h.q(ctx).CreatePlaybook(ctx, db.CreatePlaybookParams{
		TenantID:         tenantID,
		PlaybookName:     form.PlaybookName,
		TriggerScenario:  form.TriggerScenario,
		Description:      form.Description,
		Steps:            form.Steps,
		RecommendedOwner: form.RecommendedOwner,
		IsActive:         true, // playbook baru langsung aktif (bisa dijadikan draf kemudian)
		CreatedBy:        &uid,
	})
	if err != nil {
		h.Log.Error("playbooks: create", "err", err)
		wsRedirect(w, r, "/playbooks/new", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "playbook.create", tenantID, map[string]string{
		"playbook_id": strconv.FormatInt(p.ID, 10),
	})
	wsRedirectOK(w, r, "/playbooks", "created")
}

// PlaybookEdit — GET /w/{workspace}/playbooks/{id}/edit. Form terisi profil
// playbook (bukan is_active — draf/aktifkan jalur tersendiri).
func (h *Handler) PlaybookEdit(w http.ResponseWriter, r *http.Request) {
	if !h.requirePlaybookWrite(w, r) {
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	p, ok := h.loadPlaybook(w, r, id)
	if !ok {
		return
	}

	base := wsPath(slugFromRequest(r), "")
	v := panel.PlaybookFormView{
		Base:              base,
		Action:            base + "/playbooks/" + strconv.FormatInt(p.ID, 10),
		IsEdit:            true,
		Err:               playbooksErrMsg(r.URL.Query().Get("err")),
		Fields:            playbookFormFields(p),
		TriggerScenarios:  playbookTriggerScenarioOptions,
		RecommendedOwners: playbookRecommendedOwnerOptions,
	}
	h.renderWorkspaceShell(w, r, "Sunting Playbook", "/playbooks", panel.PlaybookForm(v))
}
