package handler

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/session"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// roles.go — AKSI atas peran CRM workspace: buat, sunting matriks izin, hapus.
// Halaman/editor-nya ada di roles_page.go. Ketiga aksi berbagi satu invarian:
// peran is_system (admin) TAK bisa disunting/dihapus, dan setiap perubahan yang
// menyentuh matriks me-reload enforcer tenant agar berlaku seketika tanpa restart.

const (
	roleDisplayMax = 60  // batas nama tampilan peran (label layar, bukan identitas)
	roleDescMax    = 200 // batas deskripsi peran (kalimat singkat, kolom wireframe 9.2)
)

// RoleCreate — POST /w/{workspace}/roles. Buat peran CRM baru (matriks kosong;
// izinnya disunting kemudian lewat editor). Nama = identitas mesin (subject
// Casbin), display_name = label layar, data_scope = cakupan F3.
func (h *Handler) RoleCreate(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !canManageRoles(ctx) {
		wsRedirect(w, r, "/roles", "forbidden")
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if !authz.ValidBusinessRoleName(name) {
		wsRedirect(w, r, "/roles", "role_name")
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
	// Deskripsi opsional (boleh kosong) — hanya panjangnya dibatasi. TrimSpace agar
	// spasi belaka = kosong, bukan "deskripsi berisi spasi".
	description := strings.TrimSpace(r.FormValue("description"))
	if len(description) > roleDescMax {
		wsRedirect(w, r, "/roles", "role_desc")
		return
	}
	tenantID := session.TenantID(ctx)
	actorID := session.UserID(ctx)
	// CreateBusinessRole = ON CONFLICT DO NOTHING RETURNING: nama sudah dipakai →
	// zero rows → pgx.ErrNoRows. Itu bukan galat internal, tapi "nama bentrok".
	if _, err := h.q(ctx).CreateBusinessRole(ctx, db.CreateBusinessRoleParams{
		TenantID: tenantID, Name: name, DisplayName: display, Description: description,
		DataScope: scope, IsSystem: false, CreatedBy: &actorID,
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			wsRedirect(w, r, "/roles", "role_exists")
			return
		}
		h.Log.Error("roles: create", "err", err)
		wsRedirect(w, r, "/roles", "failed")
		return
	}
	// Tak ada reload enforcer: peran baru bermatriks KOSONG → nol baris policy,
	// tak ada yang berubah di enforcer. Ia mulai deny-default sampai disunting.
	h.auditWorkspace(ctx, actorID, "crm.role.create", tenantID, map[string]string{"role": name})
	wsRedirectOK(w, r, "/roles", "created")
}

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
	description := strings.TrimSpace(r.FormValue("description"))
	if len(description) > roleDescMax {
		wsRedirect(w, r, "/roles", "role_desc")
		return
	}
	perms := readRoleMatrix(r)
	actorID := session.UserID(ctx)

	if err := h.q(ctx).UpdateBusinessRole(ctx, db.UpdateBusinessRoleParams{
		TenantID: tenantID, Name: name, DisplayName: display, Description: description,
		DataScope: scope, UpdatedBy: &actorID,
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
	h.auditWorkspace(ctx, actorID, "crm.role.update", tenantID, map[string]string{"role": name})
	wsRedirectOK(w, r, "/roles", "saved")
}

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
// untuk modul lain — penjaga agar sel tak bermakna tak jadi p-rule.
//
// write TAK menulis read tambahan: matcher bisnis membuat write mencakup read
// (business.conf), jadi satu baris (obj,write) sudah memberi keduanya.
func readRoleMatrix(r *http.Request) []rolePerm {
	mods := authz.CRMModules()
	perms := make([]rolePerm, 0, len(mods))
	for _, m := range mods {
		switch r.FormValue("level." + m.Obj) {
		case "read":
			perms = append(perms, rolePerm{m.Obj, "read"})
		case "write":
			perms = append(perms, rolePerm{m.Obj, "write"})
		}
		if m.CanApprove && r.FormValue("approve."+m.Obj) == "1" {
			perms = append(perms, rolePerm{m.Obj, "approve"})
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
