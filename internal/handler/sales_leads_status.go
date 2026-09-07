package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// sales_leads_status.go — AKSI ubah status Lead (BL-83). Transisi status =
// aksi tersendiri, dipisah dari sunting profil (LeadUpdate), cermin DealStage
// (sales_deals_stage.go). Kontrol ada di detail Lead, bukan di form profil.
//
// Keputusan BL-83: transisi BEBAS antar-status manual (New/Contacted/Qualified/
// Unqualified) — tanpa urutan wajib. 'Converted' TETAP sistem-only: tak ada di
// validLeadStatuses (ditolak di sini) DAN query di-guard `AND NOT converted`
// (baris hasil konversi tak tersentuh). "Alasan Unqualified" (BL-80) ikut ke
// kontrol status ini — hanya bermakna saat status Unqualified; nilai basi dibuang.

// LeadStatus — POST /w/{workspace}/leads/{id}/status. Memindah status lead
// (aksi tersendiri, bukan efek edit profil). Menegakkan ownership (F3) via
// loadOwnedLead. Lead yang sudah dikonversi tak bisa diubah statusnya (kontrol
// disembunyikan di view + query menolak). Audit `lead.status.changed` (dari→ke).
func (h *Handler) LeadStatus(w http.ResponseWriter, r *http.Request) {
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
	idStr := strconv.FormatInt(id, 10)

	// Lead terkonversi = status terminal 'Converted'; tak boleh diputar balik lewat
	// kontrol status. Query juga menolak (AND NOT converted); ini menolak lebih awal
	// dengan pesan yang bisa dimengerti alih-alih diam-diam no-op.
	if l.Converted {
		wsRedirect(w, r, "/leads/"+idStr, "lead_status")
		return
	}

	status := strings.TrimSpace(r.FormValue("lead_status"))
	if _, valid := validLeadStatuses[status]; !valid {
		wsRedirect(w, r, "/leads/"+idStr, "lead_status")
		return
	}

	// Alasan Unqualified (BL-80) HANYA bermakna saat status Unqualified — field
	// disembunyikan klien (data-show) untuk status lain TETAP terkirim; backend
	// penegak: buang nilai basi agar tak ada "alasan yatim" pada lead non-Unqualified.
	reason := optTrim(r.FormValue("unqualified_reason"))
	if status != "Unqualified" {
		reason = nil
	}

	// Idempoten & tanpa jejak palsu: status tak berubah → tak menulis/audit.
	if status == l.LeadStatus && derefEq(reason, l.UnqualifiedReason) {
		wsRedirectOK(w, r, "/leads/"+idStr, "saved")
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).UpdateLeadStatus(ctx, db.UpdateLeadStatusParams{
		LeadStatus:        status,
		UnqualifiedReason: reason,
		UpdatedBy:         &uid,
		ID:                id,
	}); err != nil {
		h.Log.Error("leads: status", "err", err)
		wsRedirect(w, r, "/leads/"+idStr, "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "lead.status.changed", session.TenantID(ctx), map[string]string{
		"lead_id": idStr, "from": l.LeadStatus, "to": status,
	})
	wsRedirectOK(w, r, "/leads/"+idStr, "saved")
}

// derefEq membandingkan dua *string sebagai nilai (nil == nil, nil != ""). Dipakai
// deteksi "tak ada perubahan" alasan Unqualified agar transisi no-op tak beraudit.
func derefEq(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}
