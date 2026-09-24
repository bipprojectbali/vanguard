package handler

import (
	"context"
	"net/http"

	"go_starter/internal/db"
)

// roles_update_scope.go — applyRoleMemberScope (Cakupan Jenis Anggota,
// BL-171). Dipisah dari RoleUpdate (roles_update.go) agar file itu di bawah
// ambang tipe Route/Handler (150).

// applyRoleMemberScope melebur ke form RoleUpdate yang sama (mscope_present,
// pola persis fsec_present di sana) — beda dari Field Security, gatenya SAMA
// dengan matriks (canManageRoles, sudah diperiksa di awal RoleUpdate), tak
// perlu diperiksa ulang. Level crm:members bukan "read"/"write" (atau field
// level absen) → hapus baris (CHECK DB msp_at_least_one_chk menolak
// false/false, jadi "nonaktif" direpresentasikan lewat ABSENnya baris, bukan
// dua kolom false — lihat migrasi 00052). Kedua checkbox tak tercentang saat
// level aktif → coerce true/true (default terbuka, opt-in) alih-alih menolak
// submit (JS sudah mencegahnya, ini lapis server).
func (h *Handler) applyRoleMemberScope(ctx context.Context, r *http.Request, tenantID, actorID int64, name string) {
	if r.FormValue("mscope_present") != "1" {
		return
	}
	if lvl := r.FormValue("level.crm:members"); lvl == "read" || lvl == "write" {
		viewInternal := r.FormValue("mscope_internal") == "1"
		viewExternal := r.FormValue("mscope_external") == "1"
		if !viewInternal && !viewExternal {
			viewInternal, viewExternal = true, true
		}
		if err := h.q(ctx).UpsertMemberScopePolicy(ctx, db.UpsertMemberScopePolicyParams{
			TenantID: tenantID, BusinessRole: name,
			CanViewInternal: viewInternal, CanViewExternal: viewExternal,
			CreatedBy: &actorID,
		}); err != nil {
			h.Log.Error("roles: save member scope", "err", err)
		}
	} else if err := h.q(ctx).DeleteMemberScopePolicyForRole(ctx, db.DeleteMemberScopePolicyForRoleParams{
		TenantID: tenantID, BusinessRole: name,
	}); err != nil {
		h.Log.Error("roles: clear member scope", "err", err)
	}
}
