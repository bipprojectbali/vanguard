package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"
)

// playbooks_status.go — AKSI status katalog: jadikan draf / aktifkan kembali
// playbook. Dipisah dari sunting profil (playbooks.go) karena is_active
// adalah aksi bisnis tersendiri (SetPlaybookActive), bukan efek samping edit
// — menjadikan draf sebuah playbook menghapusnya dari picker baru tanpa
// mengubah riwayat pemakaian lama. Meniru sla_policies_status.go (slice A1).
// Gerbang = requirePlaybookWrite.
//
// Wording "Draf" (bukan "Pensiun" seperti A1) mengikuti wireframe Penpot
// "CS — 6.7 Playbooks" — mekanisme is_active toggle IDENTIK A1, hanya label
// yang berbeda per konvensi wireframe modul ini.

// PlaybookDraft — POST /w/{workspace}/playbooks/{id}/draft. Jadikan draf
// playbook (is_active=false). Idempotent di DB; playbook hilang dari
// ListPlaybooks (picker) tapi tetap tampil di katalog kelola
// (ListPlaybooksAll) ditandai badge.
func (h *Handler) PlaybookDraft(w http.ResponseWriter, r *http.Request) {
	h.setPlaybookActive(w, r, false, "playbook.draft", "drafted")
}

// PlaybookActivate — POST /w/{workspace}/playbooks/{id}/activate. Aktifkan
// kembali playbook draf (is_active=true) → tampil lagi di picker.
func (h *Handler) PlaybookActivate(w http.ResponseWriter, r *http.Request) {
	h.setPlaybookActive(w, r, true, "playbook.activate", "activated")
}

// setPlaybookActive = jalur bersama draf/aktifkan: gate tulis → parse id →
// muat (menegakkan keberadaan/RLS) → SetPlaybookActive → audit → 303. active
// menentukan draf (false) / aktifkan (true); auditAction & okCode dioper
// pemanggil agar jejak & pesan spesifik per aksi.
func (h *Handler) setPlaybookActive(w http.ResponseWriter, r *http.Request, active bool, auditAction, okCode string) {
	if !h.requirePlaybookWrite(w, r) {
		return
	}
	ctx := r.Context()
	id, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	if _, ok := h.loadPlaybook(w, r, id); !ok {
		return
	}

	uid := session.UserID(ctx)
	if err := h.q(ctx).SetPlaybookActive(ctx, db.SetPlaybookActiveParams{
		IsActive:  active,
		UpdatedBy: &uid,
		ID:        id,
	}); err != nil {
		h.Log.Error("playbooks: set active", "err", err, "active", active)
		wsRedirect(w, r, "/playbooks", "failed")
		return
	}

	h.auditWorkspace(ctx, uid, auditAction, session.TenantID(ctx), map[string]string{
		"playbook_id": strconv.FormatInt(id, 10),
	})
	wsRedirectOK(w, r, "/playbooks", okCode)
}
