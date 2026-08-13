package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// sales_leads_update.go — AKSI sunting Lead: form edit + simpan. Dipecah dari
// sales_leads.go (buat) semata untuk file health; gerbang & konvensi sama persis.

// LeadEdit — GET /w/{workspace}/leads/{id}/edit. Form terisi. Mensyaratkan F3:
// hanya yang boleh MELIHAT baris yang boleh membuka form suntingnya (404 di luar cakupan).
func (h *Handler) LeadEdit(w http.ResponseWriter, r *http.Request) {
	if !h.requireLeadWrite(w, r) {
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	l, ok := h.loadOwnedLead(w, r, id)
	if !ok {
		return
	}

	base := wsPath(slugFromRequest(r), "")
	idStr := strconv.FormatInt(l.ID, 10)
	v := panel.LeadFormView{
		Base:     base,
		Action:   base + "/leads/" + idStr,
		IsEdit:   true,
		Err:      wsErrMsg(r.URL.Query().Get("err")),
		Fields:   leadFormFields(l),
		Statuses: leadStatusOptions,
		Ratings:  leadRatingOptions,
	}
	h.renderWorkspaceShell(w, r, "Sunting Lead", "/leads", panel.LeadForm(v))
}

// LeadUpdate — POST /w/{workspace}/leads/{id}. Menyimpan sunting. entity_code &
// lead_owner & converted_* TAK disentuh (kode identitas & efek konversi terpisah);
// owner dipertahankan apa adanya agar edit tak diam-diam memindah kepemilikan.
func (h *Handler) LeadUpdate(w http.ResponseWriter, r *http.Request) {
	if !h.requireLeadWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	l, ok := h.loadOwnedLead(w, r, id)
	if !ok {
		return
	}

	form, errCode := parseLeadForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/leads/"+strconv.FormatInt(id, 10)+"/edit", errCode)
		return
	}

	uid := session.UserID(ctx)
	if _, err := h.q(ctx).UpdateLead(ctx, db.UpdateLeadParams{
		LeadName:          form.LeadName,
		LeadOwner:         l.LeadOwner, // pertahankan pemilik; reassign jalur terpisah
		ContactPerson:     form.ContactPerson,
		JobTitle:          form.JobTitle,
		LeadSource:        form.LeadSource,
		LeadStatus:        form.LeadStatus,
		Rating:            form.Rating,
		UnqualifiedReason: form.UnqualifiedReason,
		EstimatedValue:    form.EstimatedValue,
		Province:          form.Province,
		Regency:           form.Regency,
		District:          form.District,
		MobilePhone:       form.MobilePhone,
		Whatsapp:          form.Whatsapp,
		Email:             form.Email,
		UpdatedBy:         &uid,
		ID:                id,
	}); err != nil {
		h.Log.Error("leads: update", "err", err)
		wsRedirect(w, r, "/leads/"+strconv.FormatInt(id, 10)+"/edit", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "lead.update", session.TenantID(ctx), map[string]string{
		"lead_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/leads/"+strconv.FormatInt(id, 10), "saved")
}
