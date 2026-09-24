package handler

import (
	"net/http"
	"strconv"
	"strings"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/go-chi/chi/v5"
)

// roles_update.go — RoleUpdate (ganti label/cakupan/matriks izin peran).
// Dipisah dari roles.go krn ambang File Health (Route/Handler 150 baris).
// applyRoleMemberScope (BL-171) dipisah lagi ke roles_update_scope.go agar
// file ini tetap di bawah ambang yang sama.

// RoleUpdate — POST /w/{workspace}/roles/{name}. Ganti label, cakupan, dan
// SELURUH matriks izin peran (pola replace-all: hapus semua lalu tulis set baru).
// is_system ditolak. Reload enforcer di akhir agar izin baru berlaku seketika.
func (h *Handler) RoleUpdate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageRoles(ctx) {
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
	if role.IsSystem {
		wsRedirect(w, r, "/roles", "role_system")
		return
	}
	display := strings.TrimSpace(r.FormValue("display_name"))
	if display == "" || len(display) > roleDisplayMax {
		wsRedirect(w, r, "/roles", "role_display")
		return
	}
	scope := r.FormValue("data_scope")
	if !authz.ValidDataScope(scope) {
		wsRedirect(w, r, "/roles", "role_scope")
		return
	}
	kind := r.FormValue("kind")
	if !authz.ValidKind(kind) {
		wsRedirect(w, r, "/roles", "kind")
		return
	}
	description := strings.TrimSpace(r.FormValue("description"))
	if len(description) > roleDescMax {
		wsRedirect(w, r, "/roles", "role_desc")
		return
	}
	perms := readRoleMatrix(r)
	actorID := session.UserID(ctx)

	if err := h.q(ctx).UpdateBusinessRole(ctx, db.UpdateBusinessRoleParams{
		TenantID: tenantID, Name: name, DisplayName: display, Description: description,
		DataScope: scope, Kind: kind, UpdatedBy: &actorID,
	}); err != nil {
		h.Log.Error("roles: update", "err", err)
		wsRedirect(w, r, "/roles", "failed")
		return
	}
	// Replace-all: kosongkan matriks lama lalu tulis set baru. Lebih sederhana &
	// bebas selisih daripada diff per-baris (bisnis-defaults §DeleteBusinessRole...).
	if err := h.q(ctx).DeleteBusinessRolePermissionsForRole(ctx,
		db.DeleteBusinessRolePermissionsForRoleParams{TenantID: tenantID, Role: name}); err != nil {
		h.Log.Error("roles: clear perms", "err", err)
		wsRedirect(w, r, "/roles", "failed")
		return
	}
	for _, p := range perms {
		if err := h.q(ctx).InsertBusinessRolePermission(ctx, db.InsertBusinessRolePermissionParams{
			TenantID: tenantID, Role: name, Obj: p.Obj, Act: p.Act,
		}); err != nil {
			h.Log.Error("roles: insert perm", "obj", p.Obj, "act", p.Act, "err", err)
			wsRedirect(w, r, "/roles", "failed")
			return
		}
	}
	if err := h.reloadBusinessTenant(ctx, tenantID); err != nil {
		h.Log.Error("roles: reload enforcer", "err", err)
		wsRedirect(w, r, "/roles", "failed")
		return
	}
	// Form gabungan (satu submit, BL-145 subtask 7): saat halaman detail merender
	// bagian Field Security (canManageFieldSecurity SAAT GET), ia menyertakan
	// fsec_present=1 sebagai penanda "bagian ini memang dirender" — tanpanya,
	// view/edit yang absen tak bisa dibedakan dari "form tak punya bagian FLS
	// sama sekali" (mis. panggilan lama/test yang tak tahu-menahu soal FLS).
	// Gerbang canManageFieldSecurity diperiksa ULANG di sini (bukan cuma dipercaya
	// dari GET) agar submit yang menyelipkan field itu tanpa hak F4 tak lolos.
	// Tulisnya lewat SAVEPOINT (bukan transaksi independen): tetap tx/koneksi
	// YANG SAMA dgn h.q(ctx) agar coercion membaca matriks yg BARU saja ditulis di
	// atas (read-your-writes), sekaligus gagal-lunak thd tulis matriks yg sudah
	// sukses — lihat ADR 0012 (revisi) & db.WithSavepoint.
	if canManageFieldSecurity(ctx) && r.FormValue("fsec_present") == "1" {
		view := r.FormValue("view") == "1"
		edit := r.FormValue("edit") == "1"
		var count int
		ferr := db.WithSavepoint(ctx, h.q(ctx), func(q *db.Queries) error {
			var err error
			count, err = h.writeFieldSecurity(ctx, q, tenantID, actorID, name, false, view, edit)
			return err
		})
		if ferr != nil {
			h.Log.Error("roles: save field security (savepoint)", "err", ferr)
		} else {
			h.auditWorkspace(ctx, actorID, "workspace.field_security", tenantID, map[string]string{
				"roles": strconv.Itoa(count), "role": name,
			})
		}
	}
	h.applyRoleMemberScope(ctx, r, tenantID, actorID, name)

	h.auditWorkspace(ctx, actorID, "crm.role.update", tenantID, map[string]string{"role": name})
	wsRedirectOK(w, r, "/roles", "saved")
}
