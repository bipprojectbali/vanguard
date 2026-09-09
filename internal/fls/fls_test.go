package fls

import (
	"testing"

	"go_starter/internal/authz"
)

// fls_test.go — kontrak cache FLS phone murni (tanpa DB, tanpa ctx). Yang dijaga:
// (1) DEFAULT pra-BL-107 saat tenant belum dikonfigurasi (Sales+Admin lihat, Sales
// sunting) — pergeseran diam-diam di sini mengubah siapa yang melihat PII di SEMUA
// tenant yang belum menyentuh halaman Settings; (2) fail-closed saat tenant SUDAH
// dikonfigurasi (peran tanpa baris → tersembunyi, bukan jatuh ke default); (3)
// isolasi antar-tenant (config A tak bocor ke B). Global byTenant di-reset tiap
// kasus (Load(nil)) — JANGAN t.Parallel, negara bersama.

const (
	tA int64 = 101
	tB int64 = 202
)

// TestDefault_Unconfigured: tenant tanpa config = perilaku hardcode lama. Lihat =
// Sales+Admin; sunting = Sales saja. Peran platform tersasar & nilai liar → false
// (allow-list, sumbu F4 tegak lurus dari role tenant/platform).
func TestDefault_Unconfigured(t *testing.T) {
	Load(nil)
	view := map[string]bool{
		authz.BusinessRoleSales:   true,
		authz.BusinessRoleAdmin:   true,
		authz.BusinessRoleManager: false,
		authz.BusinessRoleCSM:     false,
		authz.BusinessRoleSupport: false,
		"":                        false,
		"super_admin":             false,
		"owner":                   false,
		"staff":                   false,
	}
	for role, want := range view {
		if got := CanViewPhone(tA, role); got != want {
			t.Errorf("default CanViewPhone(%q) = %v, want %v", role, got, want)
		}
	}
	edit := map[string]bool{
		authz.BusinessRoleSales:   true,
		authz.BusinessRoleAdmin:   false, // Admin lihat-saja (dipertahankan dari realita lama)
		authz.BusinessRoleManager: false,
		authz.BusinessRoleCSM:     false,
		authz.BusinessRoleSupport: false,
		"":                        false,
	}
	for role, want := range edit {
		if got := CanEditPhone(tA, role); got != want {
			t.Errorf("default CanEditPhone(%q) = %v, want %v", role, got, want)
		}
	}
}

// TestConfigured_Override: tenant yang dikonfigurasi memakai barisnya, bukan default.
// Manager (default: tak lihat) di-grant lihat+sunting → keduanya true.
func TestConfigured_Override(t *testing.T) {
	Load([]Row{
		{TenantID: tA, BusinessRole: authz.BusinessRoleManager, CanViewPhone: true, CanEditPhone: true},
	})
	if !CanViewPhone(tA, authz.BusinessRoleManager) {
		t.Error("Manager di-grant lihat → CanViewPhone harus true")
	}
	if !CanEditPhone(tA, authz.BusinessRoleManager) {
		t.Error("Manager di-grant sunting → CanEditPhone harus true")
	}
}

// TestConfigured_FailClosed: sekali tenant dikonfigurasi, peran TANPA baris jadi
// tersembunyi — TIDAK jatuh kembali ke default. Sales (default lihat+sunting) yang
// tak punya baris pada tenant terkonfigurasi → false di kedua sumbu.
func TestConfigured_FailClosed(t *testing.T) {
	Load([]Row{
		{TenantID: tA, BusinessRole: authz.BusinessRoleManager, CanViewPhone: true, CanEditPhone: false},
	})
	if CanViewPhone(tA, authz.BusinessRoleSales) {
		t.Error("tenant terkonfigurasi: Sales tanpa baris harus fail-closed (view=false), bukan default true")
	}
	if CanEditPhone(tA, authz.BusinessRoleSales) {
		t.Error("tenant terkonfigurasi: Sales tanpa baris harus fail-closed (edit=false)")
	}
}

// TestTenantIsolation: config tenant A tak menyentuh tenant B. B (tak dikonfigurasi)
// tetap default; A memakai barisnya.
func TestTenantIsolation(t *testing.T) {
	Load([]Row{
		{TenantID: tA, BusinessRole: authz.BusinessRoleManager, CanViewPhone: true, CanEditPhone: true},
	})
	// A: Sales fail-closed (terkonfigurasi, tak ada baris).
	if CanViewPhone(tA, authz.BusinessRoleSales) {
		t.Error("A terkonfigurasi: Sales harus fail-closed")
	}
	// B: default utuh — Sales lihat+sunting, Manager tak lihat.
	if !CanViewPhone(tB, authz.BusinessRoleSales) || !CanEditPhone(tB, authz.BusinessRoleSales) {
		t.Error("B tak dikonfigurasi: Sales harus default lihat+sunting")
	}
	if CanViewPhone(tB, authz.BusinessRoleManager) {
		t.Error("B tak dikonfigurasi: Manager tak boleh lihat (default), config A bocor?")
	}
}

// TestReloadTenant_NilResetsToDefault: ReloadTenant(nil) menghapus konfigurasi →
// tenant kembali ke default terkunci.
func TestReloadTenant_NilResetsToDefault(t *testing.T) {
	Load([]Row{
		{TenantID: tA, BusinessRole: authz.BusinessRoleManager, CanViewPhone: true, CanEditPhone: true},
	})
	ReloadTenant(tA, nil)
	if !CanViewPhone(tA, authz.BusinessRoleSales) {
		t.Error("setelah ReloadTenant(nil): Sales harus kembali default lihat=true")
	}
	if CanViewPhone(tA, authz.BusinessRoleManager) {
		t.Error("setelah ReloadTenant(nil): Manager harus kembali default lihat=false")
	}
}

// TestReloadTenant_EmptyMarksConfigured: peta kosong-tapi-non-nil menandai tenant
// TERKONFIGURASI (admin uncheck semua) — beda dari default. Semua peran fail-closed.
func TestReloadTenant_EmptyMarksConfigured(t *testing.T) {
	Load(nil)
	ReloadTenant(tA, map[string]Policy{})
	if CanViewPhone(tA, authz.BusinessRoleSales) || CanViewPhone(tA, authz.BusinessRoleAdmin) {
		t.Error("peta kosong = terkonfigurasi all-false: Sales/Admin tak boleh lihat (bukan default)")
	}
	if CanEditPhone(tA, authz.BusinessRoleSales) {
		t.Error("peta kosong = terkonfigurasi all-false: Sales tak boleh sunting")
	}
}
