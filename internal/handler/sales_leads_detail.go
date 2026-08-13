package handler

import (
	"context"
	"errors"
	"net/http"

	"go_starter/internal/db"
	"go_starter/internal/session"
	"go_starter/internal/ui/pages/panel"

	"github.com/jackc/pgx/v5"
)

// sales_leads_detail.go — HALAMAN detail satu Lead + helper view/nama anggota.
// Dipecah dari sales_leads_page.go (daftar) semata untuk file health.

// LeadDetail — GET /w/{workspace}/leads/{id}. Satu lead. Ownership diputuskan DI
// SINI (filter.Allows) — kembaran per-baris dari list. Di luar cakupan → 404.
func (h *Handler) LeadDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canViewLeads(ctx) {
		h.renderLeadsForbidden(w, r)
		return
	}
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}

	l, err := h.q(ctx).GetLead(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			http.NotFound(w, r)
			return
		}
		h.Log.Error("leads: get", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	filter := db.LeadsListFilterFor(session.BusinessDataScope(ctx))
	if !filter.Allows(session.UserID(ctx), l.LeadOwner) {
		http.NotFound(w, r)
		return
	}

	names, err := h.memberNameMap(ctx)
	if err != nil {
		h.Log.Error("leads: members", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	base := wsPath(slugFromRequest(r), "")
	h.renderWorkspaceShell(w, r, l.LeadName, "/leads",
		panel.LeadDetail(h.leadDetailView(ctx, base, l, names)))
}

// leadDetailView merakit detail lengkap + F4 (estimated_value & telepon tersamar).
// Menerima ctx untuk business_role aktor (dasar masking) & peta nama anggota.
func (h *Handler) leadDetailView(ctx context.Context, base string, l db.Lead, names map[int64]string) panel.LeadDetailView {
	br := session.BusinessRole(ctx)
	canWrite := canWriteLeads(ctx)
	// Konversi hanya untuk lead Qualified yang belum dikonversi & aktor boleh tulis.
	canConvert := canWrite && l.LeadStatus == "Qualified" && !l.Converted
	return panel.LeadDetailView{
		Base:              base,
		ID:                l.ID,
		EntityCode:        deref(l.EntityCode),
		LeadName:          l.LeadName,
		ContactPerson:     deref(l.ContactPerson),
		JobTitle:          deref(l.JobTitle),
		LeadSource:        deref(l.LeadSource),
		Status:            l.LeadStatus,
		Rating:            deref(l.Rating),
		UnqualifiedReason: deref(l.UnqualifiedReason),
		EstValue:          maskARR(formatRupiah(l.EstimatedValue), br),
		Province:          deref(l.Province),
		Regency:           deref(l.Regency),
		District:          deref(l.District),
		MobilePhone:       maskPhone(deref(l.MobilePhone), br),
		Whatsapp:          maskPhone(deref(l.Whatsapp), br),
		Email:             deref(l.Email),
		Owner:             ownerName(l.LeadOwner, names),
		Converted:         l.Converted,
		ConvertedDealID:   int64PtrStr(l.ConvertedDealID),
		CanWrite:          canWrite,
		CanConvert:        canConvert,
	}
}

// memberNameMap = peta userID → penanda orang (nama, jatuh ke email) untuk seluruh
// anggota workspace. Satu query (bukan N+1) dipakai meresolusi kolom owner di
// daftar/detail. Kosong bila tak ada anggota.
func (h *Handler) memberNameMap(ctx context.Context) (map[int64]string, error) {
	rows, err := h.q(ctx).ListMembersByTenant(ctx, session.TenantID(ctx))
	if err != nil {
		return nil, err
	}
	m := make(map[int64]string, len(rows))
	for _, mem := range rows {
		label := mem.Email
		if mem.Name != nil && *mem.Name != "" {
			label = *mem.Name
		}
		m[mem.UserID] = label
	}
	return m, nil
}

// ownerName meresolusi *int64 pemilik → nama dari peta. nil / tak ditemukan → "".
func ownerName(owner *int64, names map[int64]string) string {
	if owner == nil {
		return ""
	}
	return names[*owner]
}
