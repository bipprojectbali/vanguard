package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/codes"
	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"
)

// sales_leads.go — AKSI atas Lead: gerbang tulis bersama + form buat/create.
// Sunting/hapus ada di sales_leads_update.go & sales_leads_helpers.go (dipecah
// lebih lanjut untuk file health, package sama). Halaman baca ada di
// sales_leads_page.go/sales_leads_detail.go. Konversi lead → account+contact+
// deal ada di sales_convert*.go (inti modul, tx atomik). Meniru accounts.go.
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
