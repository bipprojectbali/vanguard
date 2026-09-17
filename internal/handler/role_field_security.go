package handler

import (
	"context"
	"net/http"
	"strconv"

	"go_starter/internal/db"
	"go_starter/internal/fls"
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

// writeFieldSecurity menulis kebijakan FLS SATU peran (replace-all tenant,
// peran lain dibekukan ke nilai efektif saat ini) — inti bersama
// RoleFieldSecurityUpdate (form terpisah, peran sistem+kustom) dan RoleUpdate
// (form gabungan, HANYA peran kustom, lihat roles.go). `q`
// diserahkan pemanggil: RoleUpdate membungkusnya dalam db.WithSavepoint agar
// tulisannya terisolasi-gagal dari tulis matriks tanpa transaksi terpisah
// (ADR 0012).
func (h *Handler) writeFieldSecurity(ctx context.Context, q *db.Queries, tenantID, actorID int64, name string, isSystem bool, view, edit bool) (int, error) {
	// Closed set: peran DITENTUKAN dari DB (pola jalur lama) — barang siapa yang
	// tak lagi ada tak ikut ditulis ulang.
	roles, err := q.ListBusinessRoles(ctx, tenantID)
	if err != nil {
		return 0, err
	}

	// Nilai efektif SAAT INI (sebelum tulis) — sumber "pembekuan" bagi peran
	// selain yang sedang disunting: existing (kalau tenant sudah dikonfigurasi)
	// atau default terkunci fls.CanViewPhone/CanEditPhone (kalau belum).
	existingRows, err := q.ListFieldSecurityPolicies(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	existing := make(map[string]fls.Policy, len(existingRows))
	for _, p := range existingRows {
		existing[p.BusinessRole] = fls.Policy{CanViewPhone: p.CanViewPhone, CanEditPhone: p.CanEditPhone}
	}

	if edit {
		view = true
	}
	// Coercion server-side thd level Contacts/Leads (F2) — TAK cukup mengandalkan
	// disabled di klien, form bisa dikirim langsung. Peran sistem (admin) lolos:
	// diwakili glob crm:* di enforcer, tak punya matriks untuk direaksikan.
	if !isSystem {
		perms, err := q.ListBusinessRolePermissionsByTenant(ctx, tenantID)
		if err != nil {
			return 0, err
		}
		cells := permsByRole(perms)[name]
		contactsLevel := cellLevel(cells["crm:contacts"])
		leadsLevel := cellLevel(cells["crm:leads"])
		if contactsLevel == "none" && leadsLevel == "none" {
			view = false
			edit = false
		}
		if contactsLevel != "write" && leadsLevel != "write" {
			edit = false
		}
	}

	policies := make(map[string]fls.Policy, len(roles))
	for _, r2 := range roles {
		if r2.Name == name {
			policies[r2.Name] = fls.Policy{CanViewPhone: view, CanEditPhone: edit}
			continue
		}
		if p, ok := existing[r2.Name]; ok {
			policies[r2.Name] = p
			continue
		}
		policies[r2.Name] = fls.Policy{
			CanViewPhone: fls.CanViewPhone(tenantID, r2.Name),
			CanEditPhone: fls.CanEditPhone(tenantID, r2.Name),
		}
	}

	if err := q.DeleteFieldSecurityPoliciesForTenant(ctx, tenantID); err != nil {
		return 0, err
	}
	for roleName, p := range policies {
		if err := q.UpsertFieldSecurityPolicy(ctx, db.UpsertFieldSecurityPolicyParams{
			TenantID:     tenantID,
			BusinessRole: roleName,
			CanViewPhone: p.CanViewPhone,
			CanEditPhone: p.CanEditPhone,
			CreatedBy:    &actorID,
		}); err != nil {
			return 0, err
		}
	}

	fls.ReloadTenant(tenantID, policies)
	return len(policies), nil
}

// cellLevel menurunkan Level ("none"/"read"/"write") dari satu sel matriks —
// sel bisa nil (peran belum diberi izin modul itu sama sekali). Sama persis
// dgn logika roleModuleRows (roles_card.go), diulang di sini karena
// masukannya *permCell tunggal, bukan peta byRole.
func cellLevel(c *permCell) string {
	if c == nil {
		return "none"
	}
	switch {
	case c.write:
		return "write"
	case c.read:
		return "read"
	default:
		return "none"
	}
}
