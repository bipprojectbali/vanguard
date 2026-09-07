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

	ctx := r.Context()
	base := wsPath(slugFromRequest(r), "")
	idStr := strconv.FormatInt(l.ID, 10)
	v := panel.LeadFormView{
		Base:        base,
		Action:      base + "/leads/" + idStr,
		IsEdit:      true,
		Err:         wsErrMsg(r.URL.Query().Get("err")),
		RegionsJSON: h.regionsJSON(ctx),
		Fields:      leadFormFields(l, canEditPhone(ctx)),
		Ratings:     leadRatingOptions,
		Sources:     leadSourceOptions,
	}
	h.renderWorkspaceShell(w, r, "Sunting Lead", "/leads", panel.LeadForm(v))
}

// LeadUpdate — POST /w/{workspace}/leads/{id}. Menyimpan sunting PROFIL. entity_code
// & lead_owner & converted_* TAK disentuh (kode identitas & efek konversi terpisah);
// owner dipertahankan apa adanya agar edit tak diam-diam memindah kepemilikan.
// BL-83: lead_status & unqualified_reason JUGA dipertahankan apa adanya — transisi
// status kini aksi tersendiri (LeadStatus, sales_leads_status.go), tak lagi bagian
// form profil. Form profil tak menyertakan field status; parseLeadForm memulangkan
// default "New", maka di sini nilai lama (l.LeadStatus/UnqualifiedReason) yang dipakai
// agar sunting profil tak diam-diam mereset status.
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

	// F4: nomor HP/WhatsApp dipertahankan (bukan diambil dari form) bila role tak
	// berhak sunting (!canEditPhone) — form GET (LeadEdit) sudah menyamarkan
	// field ini (leadFormFields) buat role itu, jadi POST-nya selalu berisi mask.
	// Tanpa penjagaan ini, save oleh Manager akan menimpa nomor asli dgn literal
	// flsHidden — kebocoran F4 lewat jalur tulis. Diperbaiki audit FLS M9-1
	// (simetris dgn ActivityUpdate/Notes & AccountUpdate/ContactPhone).
	mobile, whatsapp := form.MobilePhone, form.Whatsapp
	if !canEditPhone(ctx) {
		mobile, whatsapp = l.MobilePhone, l.Whatsapp
	}

	uid := session.UserID(ctx)
	if _, err := h.q(ctx).UpdateLead(ctx, db.UpdateLeadParams{
		LeadName:          form.LeadName,
		LeadOwner:         l.LeadOwner, // pertahankan pemilik; reassign jalur terpisah
		ContactPerson:     form.ContactPerson,
		JobTitle:          form.JobTitle,
		LeadSource:        form.LeadSource,
		LeadStatus:        l.LeadStatus, // BL-83: status via LeadStatus, bukan form profil
		Rating:            form.Rating,
		UnqualifiedReason: l.UnqualifiedReason, // BL-83: terkopel status, dipertahankan di sini
		EstimatedValue:    form.EstimatedValue,
		DistrictID:        form.DistrictID,
		MobilePhone:       mobile,
		Whatsapp:          whatsapp,
		Email:             form.Email,
		UpdatedBy:         &uid,
		ID:                id,
	}); err != nil {
		if code, ok := leadWriteErr(err); ok {
			wsRedirect(w, r, "/leads/"+strconv.FormatInt(id, 10)+"/edit", code)
			return
		}
		h.Log.Error("leads: update", "err", err)
		wsRedirect(w, r, "/leads/"+strconv.FormatInt(id, 10)+"/edit", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "lead.update", session.TenantID(ctx), map[string]string{
		"lead_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/leads/"+strconv.FormatInt(id, 10), "saved")
}
