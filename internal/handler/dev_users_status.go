package handler

import (
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"
)

// dev_users_status.go — aksi lintas-workspace DevUserSetStatus & DevUserDelete
// (siklus hidup akun dari panel /dev). Dipisah dari dev_users.go (daftar +
// DevUserSetRole) agar keduanya di bawah ambang tipe Route/Handler (150).
// DevUserSetStatus — POST /dev/users/{id}/status. Disable/block/aktifkan.
func (h *Handler) DevUserSetStatus(w http.ResponseWriter, r *http.Request) {
	targetID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	newStatus := r.FormValue("status")
	switch newStatus {
	case "active", "disabled", "blocked":
	default:
		h.devFlash(w, r, false, "Status tidak valid")
		return
	}
	actor, target, err := h.loadActorTarget(r.Context(), targetID, session.TenantID(r.Context()))
	if err != nil {
		h.devFlashErr(w, r, err)
		return
	}
	if err := authz.GuardMutateStatus(actor, target, newStatus); err != nil {
		h.devFlashErr(w, r, err)
		return
	}
	if err := h.q(r.Context()).UpdateUserStatus(r.Context(), db.UpdateUserStatusParams{ID: targetID, Status: newStatus}); err != nil {
		h.Log.Error("dev users: update status", "err", err)
		h.devFlash(w, r, false, "Gagal menyimpan perubahan")
		return
	}
	h.audit(r.Context(), actor.ID, "user.status.update", targetID, map[string]string{"to": newStatus})
	// SENGAJA tanpa notifikasi. Status menutup pintu LOGIN (gate di startIdentity),
	// jadi notifikasi in-app tak akan pernah terbaca oleh yang di-disable/block —
	// ia hanya menumpuk untuk dibaca kalau statusnya kelak dipulihkan, yaitu saat
	// kabarnya sudah basi. Yang mengaktifkan kembali pun tak perlu diberi tahu:
	// ia melihatnya sendiri saat berhasil masuk.
	h.devRowUpdated(w, r, targetID, "Status diubah ke "+newStatus)
}

// DevUserDelete — POST /dev/users/{id}/delete. Soft-delete (di-guard + audit).
func (h *Handler) DevUserDelete(w http.ResponseWriter, r *http.Request) {
	targetID, ok := h.parseTargetID(w, r)
	if !ok {
		return
	}
	actor, target, err := h.loadActorTarget(r.Context(), targetID, session.TenantID(r.Context()))
	if err != nil {
		h.devFlashErr(w, r, err)
		return
	}
	if err := authz.GuardDelete(actor, target); err != nil {
		h.devFlashErr(w, r, err)
		return
	}
	if err := h.q(r.Context()).SoftDeleteUser(r.Context(), targetID); err != nil {
		h.Log.Error("dev users: delete", "err", err)
		h.devFlash(w, r, false, "Gagal menghapus user")
		return
	}
	h.audit(r.Context(), actor.ID, "user.delete", targetID, nil)
	// SENGAJA tanpa notifikasi, alasan yang sama dengan status: akun terhapus tak
	// bisa login, jadi tak ada yang akan membacanya.
	h.devRowRemoved(w, r, targetID, "User dihapus")
}
