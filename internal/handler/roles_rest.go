package handler

import (
	"context"
	"net/http"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/go-chi/chi/v5"
)

// roles_rest.go — konstanta peran, aksi hapus (RoleDelete), dan helper pembaca
// matriks izin (rolePerm/readRoleMatrix) + reloadBusinessTenant. Dipisah dari
// roles.go (buat/sunting) agar keduanya di bawah ambang tipe Route/Handler (150);
// satu paket, invarian is_system & reload enforcer tetap berlaku sama.
const (
	roleDisplayMax = 60  // batas nama tampilan peran (label layar, bukan identitas)
	roleDescMax    = 200 // batas deskripsi peran (kalimat singkat, kolom wireframe 9.2)
)

// RoleDelete — POST /w/{workspace}/roles/{name}/delete. Hapus peran. is_system
// ditolak. Anggota yang memegangnya di-unassign LEBIH DULU (business_role→NULL)
// agar penghapusan peran bukan penghapusan orang; izinnya ikut lewat FK CASCADE.
func (h *Handler) RoleDelete(w http.ResponseWriter, r *http.Request) {
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
	// Cabut dari semua anggota SEBELUM menghapus perannya (business_role→NULL) —
	// tanpa ini anggota memegang nama peran yang tak ada lagi (CanBusiness deny,
	// tapi identitasnya menggantung). Idempoten.
	if err := h.q(ctx).UnassignBusinessRole(ctx, db.UnassignBusinessRoleParams{
		TenantID: tenantID, BusinessRole: &name,
	}); err != nil {
		h.Log.Error("roles: unassign", "err", err)
		wsRedirect(w, r, "/roles", "failed")
		return
	}
	if err := h.q(ctx).DeleteBusinessRole(ctx, db.DeleteBusinessRoleParams{
		TenantID: tenantID, Name: name,
	}); err != nil {
		h.Log.Error("roles: delete", "err", err)
		wsRedirect(w, r, "/roles", "failed")
		return
	}
	if err := h.reloadBusinessTenant(ctx, tenantID); err != nil {
		h.Log.Error("roles: reload enforcer", "err", err)
		wsRedirect(w, r, "/roles", "failed")
		return
	}
	h.auditWorkspace(ctx, session.UserID(ctx), "crm.role.delete", tenantID, map[string]string{"role": name})
	wsRedirectOK(w, r, "/roles", "deleted")
}

// rolePerm = satu sel matriks yang akan ditulis (obj × act). Tipe lokal, bukan
// db.*: readRoleMatrix murni memetakan form → niat, penulisannya di pemanggil.
type rolePerm struct{ Obj, Act string }

// readRoleMatrix menurunkan set izin dari form editor. Per modul CRM (daftar
// tertutup CRMModules — objek liar dari form tak bisa menyelinap): Level "read"→
// (obj,read), "write"→(obj,write); "none"/nilai lain→lewati. Approve ditulis
// HANYA untuk modul yang mendukungnya (ModuleCanApprove) walau form mengirimnya
// untuk modul lain — penjaga agar sel tak bermakna tak jadi p-rule. arr (BL-58,
// visibilitas ARR) sama: hanya modul ber-CanARR (Subscriptions).
//
// write TAK menulis read tambahan: matcher bisnis membuat write mencakup read
// (business.conf), jadi satu baris (obj,write) sudah memberi keduanya.
//
// hasLevel (BL-145 subtask 0): approve HANYA ditulis bila modul yg sama punya
// level read/write. Tanpa guard ini form bisa kirim level=none + approve=1
// (state form stale, replay) → tersimpan kapabilitas approve tanpa akses baca
// sama sekali ke modulnya — kelas celah sama dgn BL-166 (mismatch mekanisme
// F2), tapi di sisi TULIS kebijakan. Floornya "none" saja, bebas di "read"
// MAUPUN "write" (approve sengaja tak butuh "write" — pola maker-checker,
// business.conf).
//
// arr TIDAK pakai hasLevel modulnya sendiri — dipakai arrGateOpen (guard OR
// lintas authz.ARRGateObjects): (crm:subscriptions, arr) sejak BL-169
// menggerbangi Nilai Kontrak lintas modul (canSeeARR, fls.go), jadi role yg
// levelnya "none" di Subscriptions tapi granted di salah satu gate lain
// (mis. Leads="Kelola") tetap boleh menyimpan izin arr itu. Tanpa guard OR ini,
// checkbox "Lihat Nilai Kontrak" bisa dicentang aktif di editor tapi diam-diam
// DIBUANG saat submit — mismatch UI/backend yang jadi laporan awal celah ini.
func readRoleMatrix(r *http.Request) []rolePerm {
	mods := authz.CRMModules()
	arrGateOpen := false
	for _, obj := range authz.ARRGateObjects {
		if lvl := r.FormValue("level." + obj); lvl == "read" || lvl == "write" {
			arrGateOpen = true
			break
		}
	}
	perms := make([]rolePerm, 0, len(mods))
	for _, m := range mods {
		level := r.FormValue("level." + m.Obj)
		switch level {
		case "read":
			perms = append(perms, rolePerm{m.Obj, "read"})
		case "write":
			// !WriteEnforced (dashboard/subscriptions/activities/4 Reports,
			// role_edit_levels.go tak merender "Kelola") → simpan "read".
			act := "write"
			if !m.WriteEnforced {
				act = "read"
			}
			perms = append(perms, rolePerm{m.Obj, act})
		}
		hasLevel := level == "read" || level == "write"
		if hasLevel && m.CanApprove && r.FormValue("approve."+m.Obj) == "1" {
			perms = append(perms, rolePerm{m.Obj, "approve"})
		}
		if arrGateOpen && m.CanARR && r.FormValue("arr."+m.Obj) == "1" {
			perms = append(perms, rolePerm{m.Obj, "arr"})
		}
	}
	return perms
}

// reloadBusinessTenant menyegarkan enforcer bisnis untuk SATU tenant dari DB —
// dipanggil setelah matriks/keberadaan peran berubah agar CanBusiness memantul
// seketika tanpa restart. Membaca SELURUH izin tenant (bukan satu peran):
// ReloadBusinessTenant mengganti policy tenant secara utuh, jadi set parsial akan
// menghapus izin peran lain. Baca berlangsung di tx Scope yang sama → melihat
// tulisan yang belum commit (read-your-writes), jadi cerminannya mutakhir.
func (h *Handler) reloadBusinessTenant(ctx context.Context, tenantID int64) error {
	rows, err := h.q(ctx).ListBusinessRolePermissionsByTenant(ctx, tenantID)
	if err != nil {
		return err
	}
	perms := make([]authz.BusinessPerm, len(rows))
	for i, row := range rows {
		perms[i] = authz.BusinessPerm{TenantID: row.TenantID, Role: row.Role, Obj: row.Obj, Act: row.Act}
	}
	return authz.ReloadBusinessTenant(tenantID, perms)
}
