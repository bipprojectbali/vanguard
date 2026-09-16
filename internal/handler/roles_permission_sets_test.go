package handler

import (
	"context"
	"net/http"
	"testing"

	"go_starter/internal/authz"
	"go_starter/internal/db"
	"go_starter/internal/ui/pages/panel"
)

// roles_permission_sets_test.go — turunan presentasional blok B (Permission
// Sets), blok C (Record Ownership Rules) & blok D bagian kedua (Nilai
// Kontrak/MRR) dari halaman /roles (BL-145 subtask 6). Unit test langsung atas
// fungsi murni handler (permissionSetRows/buildPermissionSetView/
// contractValueViewFor), bukan lewat HTTP round-trip — cakupan blok A (kolom +
// aksi tabel) & nil-guard render sudah diuji di paket panel
// (roles_page_test.go); di sini yang dijaga adalah TURUNAN datanya benar.

func roleRow(name, display, scope string, system bool) db.ListBusinessRolesRow {
	return db.ListBusinessRolesRow{Name: name, DisplayName: display, DataScope: scope, IsSystem: system}
}

// --- permissionSetRows -------------------------------------------------------

// TestPermissionSetRows_SystemRoleAllFull: peran sistem (admin) → SEMUA sel
// semua modul penuh ✓, tanpa memandang cells (byRole["admin"] kosong di
// praktik nyata karena admin diwakili glob crm:*; memeriksa cells akan keliru
// tampil "semua ✗" — inilah bug class yang dijaga).
func TestPermissionSetRows_SystemRoleAllFull(t *testing.T) {
	rows := permissionSetRows(roleRow("admin", "Administrator", authz.DataScopeAll, true), nil)
	if len(rows) == 0 {
		t.Fatal("harus ada baris per modul CRM")
	}
	for _, r := range rows {
		if r.View != panel.PermMarkFull || r.Create != panel.PermMarkFull || r.Edit != panel.PermMarkFull || r.Delete != panel.PermMarkFull {
			t.Errorf("modul %q: peran sistem harus semua ✓, got %+v", r.Label, r)
		}
	}
}

// TestPermissionSetRows_OwnScopeUsesTilde: peran DataScope "own" dgn level
// write pada satu modul → View/Create/Edit/Delete SEMUA tanda ~ (bukan ✓),
// diterapkan seragam (presentasional, bukan CRUD granular sungguhan).
func TestPermissionSetRows_OwnScopeUsesTilde(t *testing.T) {
	cells := map[string]*permCell{"crm:accounts": {write: true}}
	rows := permissionSetRows(roleRow("sales", "Sales", authz.DataScopeOwn, false), cells)

	var accounts *panel.PermissionSetRow
	for i := range rows {
		if rows[i].Label == "Accounts (Desa)" {
			accounts = &rows[i]
		}
	}
	if accounts == nil {
		t.Fatal("baris Accounts (Desa) harus ada")
	}
	if accounts.View != panel.PermMarkOwnOnly || accounts.Create != panel.PermMarkOwnOnly ||
		accounts.Edit != panel.PermMarkOwnOnly || accounts.Delete != panel.PermMarkOwnOnly {
		t.Errorf("scope own + level write harus semua ~, got %+v", accounts)
	}
}

// TestPermissionSetRows_ReadLevelOnlyMarksView: level "read" → hanya View
// bertanda, Create/Edit/Delete tetap ✗ (write dibutuhkan utk CRUD).
func TestPermissionSetRows_ReadLevelOnlyMarksView(t *testing.T) {
	cells := map[string]*permCell{"crm:tickets": {read: true}}
	rows := permissionSetRows(roleRow("support", "Support", authz.DataScopeNone, false), cells)

	var tickets *panel.PermissionSetRow
	for i := range rows {
		if rows[i].Label == "Tickets / Cases" {
			tickets = &rows[i]
		}
	}
	if tickets == nil {
		t.Fatal("baris Tickets / Cases harus ada")
	}
	if tickets.View != panel.PermMarkFull {
		t.Errorf("level read + scope none → View harus ✓, got %q", tickets.View)
	}
	if tickets.Create != panel.PermMarkNone || tickets.Edit != panel.PermMarkNone || tickets.Delete != panel.PermMarkNone {
		t.Errorf("level read tak boleh membuka Create/Edit/Delete, got %+v", tickets)
	}
}

// TestPermissionSetRows_NoneLevelAllDenied: modul tak diberi izin sama sekali
// → keempat kolom ✗.
func TestPermissionSetRows_NoneLevelAllDenied(t *testing.T) {
	rows := permissionSetRows(roleRow("support", "Support", authz.DataScopeNone, false), nil)
	for _, r := range rows {
		if r.View != panel.PermMarkNone || r.Create != panel.PermMarkNone || r.Edit != panel.PermMarkNone || r.Delete != panel.PermMarkNone {
			t.Errorf("modul %q tanpa izin harus semua ✗, got %+v", r.Label, r)
		}
	}
}

// --- buildPermissionSetView --------------------------------------------------

// TestBuildPermissionSetView_DefaultsToFirstRole: selectedName kosong/tak
// cocok → default roles[0] (ListBusinessRoles urut is_system dulu, admin di
// posisi pertama secara efektif).
func TestBuildPermissionSetView_DefaultsToFirstRole(t *testing.T) {
	roles := []db.ListBusinessRolesRow{
		roleRow("admin", "Administrator", authz.DataScopeAll, true),
		roleRow("sales", "Sales", authz.DataScopeOwn, false),
	}
	v := buildPermissionSetView("/w/acme", roles, nil, "")
	if v.SelectedRole != "Administrator" {
		t.Errorf("default harus roles[0] (Administrator), got %q", v.SelectedRole)
	}
	v2 := buildPermissionSetView("/w/acme", roles, nil, "ghost")
	if v2.SelectedRole != "Administrator" {
		t.Errorf("nama tak cocok harus jatuh ke default, got %q", v2.SelectedRole)
	}
}

// TestBuildPermissionSetView_SwitchesToSelectedRole: selectedName cocok →
// peran itu yang disorot, dan opsi switcher menandai Selected pada peran yang
// sama (satu-satunya).
func TestBuildPermissionSetView_SwitchesToSelectedRole(t *testing.T) {
	roles := []db.ListBusinessRolesRow{
		roleRow("admin", "Administrator", authz.DataScopeAll, true),
		roleRow("sales", "Sales", authz.DataScopeOwn, false),
	}
	v := buildPermissionSetView("/w/acme", roles, nil, "sales")
	if v.SelectedRole != "Sales" {
		t.Fatalf("harus menyorot Sales, got %q", v.SelectedRole)
	}
	if v.SelectedScopeValue != authz.DataScopeOwn {
		t.Errorf("SelectedScopeValue harus own, got %q", v.SelectedScopeValue)
	}
	selectedCount := 0
	for _, o := range v.Roles {
		if o.Selected {
			selectedCount++
			if o.Name != "sales" {
				t.Errorf("opsi Selected harus sales, got %q", o.Name)
			}
		}
	}
	if selectedCount != 1 {
		t.Errorf("tepat satu opsi harus Selected, got %d", selectedCount)
	}
}

// --- contractValueViewFor ----------------------------------------------------

// ctxAs menjalankan sebuah request kosong dalam sesi (uid, tenantRole,
// businessRole) dan mengembalikan ctx-nya — untuk memanggil fungsi handler
// yang butuh authz.CanBusiness tanpa menempuh HTTP round-trip penuh.
func (e *testEnv) ctxAs(uid int64, tenantRole, businessRole string) context.Context {
	req := rolesReq(http.MethodGet, "/w/test/roles", nil, "")
	var out context.Context
	e.runAccount(uid, tenantRole, businessRole, req, func(w http.ResponseWriter, r *http.Request) {
		out = r.Context()
	})
	return out
}

// TestContractValueViewFor_GateNilWhenUnauthorized: peninjau tanpa
// crm:field_security (sama gerbang Field Security HP/WA) → nil, section tak
// dirender.
func TestContractValueViewFor_GateNilWhenUnauthorized(t *testing.T) {
	env, uid := setupRoles(t)
	ctx := env.ctxAs(uid, "owner", "sales")
	if v := env.h.contractValueViewFor(ctx, nil); v != nil {
		t.Errorf("peninjau tanpa crm:field_security harus dapat nil, got %+v", v)
	}
}

// TestContractValueViewFor_VisiblePerRole: admin berwenang → satu baris per
// peran, Visible mengikuti canSeeARR (Support ✗ satu-satunya yang false,
// Sales/Manager/CSM/Admin ✓).
func TestContractValueViewFor_VisiblePerRole(t *testing.T) {
	env, uid := setupRoles(t)
	ctx := env.ctxAs(uid, "owner", "admin")
	roles := []db.ListBusinessRolesRow{
		roleRow(authz.BusinessRoleAdmin, "Administrator", authz.DataScopeAll, true),
		roleRow(authz.BusinessRoleSales, "Sales", authz.DataScopeOwn, false),
		roleRow(authz.BusinessRoleSupport, "Support", authz.DataScopeNone, false),
	}
	v := env.h.contractValueViewFor(ctx, roles)
	if v == nil {
		t.Fatal("admin berwenang harus dapat view non-nil")
	}
	got := make(map[string]bool, len(v.Roles))
	for _, r := range v.Roles {
		got[r.Name] = r.Visible
	}
	if !got[authz.BusinessRoleSales] {
		t.Error("Sales harus melihat Nilai Kontrak/MRR (canSeeARR true)")
	}
	if got[authz.BusinessRoleSupport] {
		t.Error("Support tak boleh melihat Nilai Kontrak/MRR (canSeeARR false)")
	}
}
