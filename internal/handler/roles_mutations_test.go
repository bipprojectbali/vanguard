package handler

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"go_starter/internal/db"
)

// roles_mutations_test.go — bagian sunting matriks & hapus peran dari
// roles_test.go, dipisah agar tiap file di bawah ambang tipe Test (400).
// Setup, helper, dan gerbang read/create tetap di roles_test.go (paket sama).

// TestRoles_UpdateMatrixAndReload: sunting matriks peran langsung mengubah izin
// (bukti reload enforcer set-penuh), tersimpan di DB, cakupan data ikut berubah,
// audit tercatat. write MENCAKUP read; approve independen.
func TestRoles_UpdateMatrixAndReload(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "own", false)

	// Sebelum: peran finance belum punya izin apa pun di enforcer.
	if env.canBiz(uid, "finance", "crm:accounts", "read") {
		t.Fatal("prasyarat: finance belum boleh apa-apa sebelum disunting")
	}

	form := roleFormValues("Keuangan", "all")
	form.Set("level.crm:accounts", "write")
	form.Set("approve.crm:deals", "1")
	req := rolesReq(http.MethodPost, "/w/test/roles/finance", form, "finance")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=saved") {
		t.Fatalf("harus ok=saved, got %q (status %d)\n%s", loc, rec.Code, rec.Body.String())
	}
	// DB: matriks & cakupan tersimpan.
	if !env.hasPerm(t, "finance", "crm:accounts", "write") {
		t.Error("crm:accounts write harus tersimpan")
	}
	if !env.hasPerm(t, "finance", "crm:deals", "approve") {
		t.Error("crm:deals approve harus tersimpan")
	}
	got, _ := env.q.GetBusinessRole(t.Context(), db.GetBusinessRoleParams{
		TenantID: env.tenantID, Name: "finance",
	})
	if got.DataScope != "all" {
		t.Errorf("data_scope harus berubah ke all, got %q", got.DataScope)
	}
	// Enforcer: reload seketika. write ⊇ read.
	if !env.canBiz(uid, "finance", "crm:accounts", "write") {
		t.Error("reload gagal — finance harus boleh write crm:accounts")
	}
	if !env.canBiz(uid, "finance", "crm:accounts", "read") {
		t.Error("write harus mencakup read (matcher bisnis)")
	}
	if !env.canBiz(uid, "finance", "crm:deals", "approve") {
		t.Error("approve crm:deals harus berlaku setelah reload")
	}
	// Peran default lain TAK terhapus oleh reload set-penuh.
	if !env.canBiz(uid, "sales", "crm:accounts", "read") {
		t.Error("reload set-penuh tak boleh menghapus izin peran lain (sales)")
	}
	env.assertAudited(t, "crm.role.update")
}

// TestRoles_UpdateSystemRejected: peran sistem (admin) kebal sunting → err,
// nama tampilannya tak berubah.
func TestRoles_UpdateSystemRejected(t *testing.T) {
	env, uid := setupRoles(t)
	form := roleFormValues("Diretas", "none")
	req := rolesReq(http.MethodPost, "/w/test/roles/admin", form, "admin")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleUpdate)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=role_system") {
		t.Errorf("sunting peran sistem harus err=role_system, got %q", loc)
	}
	got, _ := env.q.GetBusinessRole(t.Context(), db.GetBusinessRoleParams{
		TenantID: env.tenantID, Name: "admin",
	})
	if got.DisplayName == "Diretas" {
		t.Error("peran sistem tak boleh berubah")
	}
}

// --- delete ----------------------------------------------------------------

// TestRoles_DeleteSuccess: hapus peran → 303 ok=deleted, peran hilang, anggota
// pemegangnya di-unassign (business_role NULL), audit tercatat.
func TestRoles_DeleteSuccess(t *testing.T) {
	env, uid := setupRoles(t)
	env.seedRole(t, "finance", "Keuangan", "own", false)
	member := env.seedMember(t, "fin@local", "member", 0).ID
	fin := "finance"
	if err := env.q.UpdateMemberBusinessRole(t.Context(), db.UpdateMemberBusinessRoleParams{
		UserID: member, TenantID: env.tenantID, BusinessRole: &fin,
	}); err != nil {
		t.Fatalf("assign role: %v", err)
	}

	req := rolesReq(http.MethodPost, "/w/test/roles/finance/delete", url.Values{}, "finance")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleDelete)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "ok=deleted") {
		t.Errorf("harus ok=deleted, got %q (status %d)", loc, rec.Code)
	}
	if env.hasRole(t, "finance") {
		t.Error("peran harus terhapus")
	}
	m, err := env.q.GetMembership(t.Context(), db.GetMembershipParams{
		UserID: member, TenantID: env.tenantID,
	})
	if err != nil {
		t.Fatalf("get membership: %v", err)
	}
	if m.BusinessRole != nil {
		t.Errorf("anggota pemegang peran terhapus harus di-unassign, got %v", *m.BusinessRole)
	}
	env.assertAudited(t, "crm.role.delete")
}

// TestRoles_DeleteSystemRejected: peran sistem (admin) kebal hapus → err, peran
// tetap ada.
func TestRoles_DeleteSystemRejected(t *testing.T) {
	env, uid := setupRoles(t)
	req := rolesReq(http.MethodPost, "/w/test/roles/admin/delete", url.Values{}, "admin")
	rec := env.runAccount(uid, "owner", "admin", req, env.h.RoleDelete)

	if loc := rec.Header().Get("Location"); !strings.Contains(loc, "err=role_system") {
		t.Errorf("hapus peran sistem harus err=role_system, got %q", loc)
	}
	if !env.hasRole(t, "admin") {
		t.Error("peran sistem tak boleh terhapus")
	}
}

// anyRole = true bila ada baris izin milik role di daftar.
func anyRole(rows []db.ListBusinessRolePermissionsByTenantRow, role string) bool {
	for _, r := range rows {
		if r.Role == role {
			return true
		}
	}
	return false
}
