package handler

import (
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/go-chi/chi/v5"
)

// role_field_security.go — aksi simpan Field Security SATU peran, dari form di
// halaman detail (/roles/{name}, BL-145 subtask 3). Menggantikan
// WorkspaceFieldSecurityUpdate (bulk, /field-security, dihapus): matriks FLS
// kini hidup di halaman yang SAMA dengan matriks Contacts/Leads peran itu, agar
// signal Datastar bisa dipakai bersama (reaktivitas klien) — lihat ADR 0012 &
// role_edit.go. Penyimpanan tetap replace-all SATU tenant (fls.go tak berubah):
// hanya SCOPE form yang mengecil dari "semua peran" ke "satu peran", peran lain
// dibekukan ke nilai efektif saat ini sehingga perilakunya tak berubah.
//
// Gerbang di handler (canManageFieldSecurity), sama dgn jalur lama — TERPISAH
// dari canManageRoles (F2): berlaku juga untuk peran sistem (admin), yang kini
// punya form FLS sendiri walau tak punya matriks Casbin untuk direaksikan.
//
// writeFieldSecurity (penulisan replace-all) + cellLevel dipisah ke
// role_field_security_write.go agar file ini di bawah ambang tipe
// Route/Handler (150).

// RoleFieldSecurityUpdate — POST /w/{workspace}/roles/{name}/field-security.
func (h *Handler) RoleFieldSecurityUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageFieldSecurity(ctx) {
		wsRedirect(w, r, "/roles", "forbidden")
		return
	}
	if IsReadOnly(ctx) {
		wsRedirect(w, r, "/roles", "forbidden")
		return
	}
	tenantID := session.TenantID(ctx)
	name := chi.URLParam(r, "name")

	role, err := h.q(ctx).GetBusinessRole(ctx, db.GetBusinessRoleParams{TenantID: tenantID, Name: name})
	if err != nil {
		wsRedirect(w, r, "/roles", "role_notfound")
		return
	}

	view := r.FormValue("view") == "1"
	edit := r.FormValue("edit") == "1"
	uid := session.UserID(ctx)
	count, err := h.writeFieldSecurity(ctx, h.q(ctx), tenantID, uid, name, role.IsSystem, view, edit)
	if err != nil {
		h.Log.Error("role field-security: write", "err", err)
		wsRedirect(w, r, "/roles/"+name, "failed")
		return
	}

	h.auditWorkspace(ctx, uid, "workspace.field_security", tenantID, map[string]string{
		"roles": strconv.Itoa(count),
		"role":  name,
	})
	// PRG ke halaman DETAIL peran yang disunting (bukan lagi /roles bulk) — form
	// FLS kini hidup di sana.
	wsRedirectOK(w, r, "/roles/"+name, "fsec_saved")
}
