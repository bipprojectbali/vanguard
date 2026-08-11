package handler

import (
	"errors"
	"net/http"
	"strconv"

	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// sales_leads.go — AKSI atas Lead: form buat/sunting, create, update, soft-delete.
// Dipisah dari sales_leads_page.go (baca): aksi tumbuh dengan aturan TULIS (F2
// write), halaman dengan aturan LIHAT (F2 read + F3). Konversi lead → account+
// contact+deal ada di sales_convert.go (inti modul, tx atomik). Meniru accounts.go.
//
// Gerbang SEMUA aksi = canWriteLeadsPerm (crm:leads write). Ownership F3 menjaga
// baris yang di-EDIT/HAPUS lewat loadOwnedLead (404 di luar cakupan) — beda dari
// accounts M2-3 yang tak menjaga tulis per-baris. RLS tetap mengurung workspace.

// requireLeadWrite = gerbang tulis bersama. false & menulis penolakan bila aktor
// tak berhak (izin F2 write; read-only workspace ditolak lebih awal dgn pesan jelas).
func (h *Handler) requireLeadWrite(w http.ResponseWriter, r *http.Request) bool {
	if !canWriteLeadsPerm(r.Context()) {
		h.renderLeadsForbidden(w, r)
		return false
	}
	return true
}

// LeadNew — GET /w/{workspace}/leads/new. Form kosong untuk lead baru.
func (h *Handler) LeadNew(w http.ResponseWriter, r *http.Request) {
	if !h.requireLeadWrite(w, r) {
		return
	}
	base := wsPath(slugFromRequest(r), "")
	v := panel.LeadFormView{
		Base:     base,
		Action:   base + "/leads",
		IsEdit:   false,
		Err:      wsErrMsg(r.URL.Query().Get("err")),
		Statuses: leadStatusOptions,
		Ratings:  leadRatingOptions,
	}
	h.renderWorkspaceShell(w, r, "Tambah Lead", "/leads", panel.LeadForm(v))
}

// LeadCreate — POST /w/{workspace}/leads. Membuat lead. entity_code dialokasikan
// DALAM tx ber-tenant yang sama (h.q) → atomik: nomor tak terbakar untuk baris gagal.
func (h *Handler) LeadCreate(w http.ResponseWriter, r *http.Request) {
	if !h.requireLeadWrite(w, r) {
		return
	}
	ctx := r.Context()
	form, errCode := parseLeadForm(r.FormValue)
	if errCode != "" {
		wsRedirect(w, r, "/leads/new", errCode)
		return
	}

	uid := session.UserID(ctx)
	tenantID := session.TenantID(ctx)

	code, err := h.q(ctx).GenerateEntityCode(ctx, tenantID, codes.EntityLead)
	if err != nil {
		h.Log.Error("leads: generate code", "err", err)
		wsRedirect(w, r, "/leads/new", "failed")
		return
	}

	l, err := h.q(ctx).CreateLead(ctx, db.CreateLeadParams{
		TenantID:          tenantID,
		EntityCode:        &code,
		LeadName:          form.LeadName,
		LeadOwner:         &uid, // pembuat = pemilik awal (dasar F3 ScopeOwn)
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
		CreatedBy:         &uid,
	})
	if err != nil {
		h.Log.Error("leads: create", "err", err)
		wsRedirect(w, r, "/leads/new", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "lead.create", tenantID, map[string]string{
		"lead_id": strconv.FormatInt(l.ID, 10), "code": code,
	})
	wsRedirectOK(w, r, "/leads/"+strconv.FormatInt(l.ID, 10), "created")
}

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

// LeadDelete — POST /w/{workspace}/leads/{id}/delete. Soft-delete (reversibel di
// DB via deleted_at; jejak audit mencatat siapa & kapan).
func (h *Handler) LeadDelete(w http.ResponseWriter, r *http.Request) {
	if !h.requireLeadWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadOwnedLead(w, r, id); !ok {
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).SoftDeleteLead(ctx, db.SoftDeleteLeadParams{
		UpdatedBy: &uid, ID: id,
	}); err != nil {
		h.Log.Error("leads: delete", "err", err)
		wsRedirect(w, r, "/leads/"+strconv.FormatInt(id, 10), "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "lead.delete", session.TenantID(ctx), map[string]string{
		"lead_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/leads", "deleted")
}

// loadOwnedLead memuat satu lead & menegakkan F3: di luar cakupan aktor → 404
// (kembaran LeadDetail; "tampil di daftar" & "boleh disentuh" satu jawaban).
// Mengembalikan (lead, true) bila boleh, atau menulis 404/500 & (zero, false).
func (h *Handler) loadOwnedLead(w http.ResponseWriter, r *http.Request, id int64) (db.Lead, bool) {
	ctx := r.Context()
	l, err := h.q(ctx).GetLead(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return db.Lead{}, false
		}
		h.Log.Error("leads: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return db.Lead{}, false
	}
	filter := db.LeadsListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), l.LeadOwner) {
		http.NotFound(w, r)
		return db.Lead{}, false
	}
	return l, true
}

// leadFormFields memetakan Lead → nilai prefill form (semua string; nil → "").
func leadFormFields(l db.Lead) panel.LeadFormFields {
	return panel.LeadFormFields{
		LeadName:          l.LeadName,
		ContactPerson:     deref(l.ContactPerson),
		JobTitle:          deref(l.JobTitle),
		LeadSource:        deref(l.LeadSource),
		LeadStatus:        l.LeadStatus,
		Rating:            deref(l.Rating),
		UnqualifiedReason: deref(l.UnqualifiedReason),
		EstimatedValue:    numericStr(l.EstimatedValue),
		Province:          deref(l.Province),
		Regency:           deref(l.Regency),
		District:          deref(l.District),
		MobilePhone:       deref(l.MobilePhone),
		Whatsapp:          deref(l.Whatsapp),
		Email:             deref(l.Email),
	}
}
