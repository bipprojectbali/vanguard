package authz

import "testing"

// business_defaults_test.go — bukti ARRGateObjects/ModuleARRGate (gate lintas
// modul checkbox "Lihat Nilai Kontrak", fix bug: checkbox tak terbuka walau
// modul lain sudah "Kelola") berisi persis daftar yg dimaksud, dan SENGAJA
// mengecualikan crm:dashboard (granted default di semua peran bawaan → akan
// meniadakan gunanya sbg gate) & crm:reports_support (nol field finance).

func TestARRGateObjects_ContainsExpectedModules(t *testing.T) {
	want := []string{
		"crm:subscriptions",
		"crm:deals",
		"crm:leads",
		"crm:accounts",
		"crm:renewals",
		"crm:churn",
		"crm:reports_sales",
		"crm:reports_cs",
		"crm:reports_subscriptions",
	}
	if len(ARRGateObjects) != len(want) {
		t.Fatalf("ARRGateObjects punya %d entri, want %d: %v", len(ARRGateObjects), len(want), ARRGateObjects)
	}
	for _, obj := range want {
		if !ModuleARRGate(obj) {
			t.Errorf("ARRGateObjects harus memuat %q", obj)
		}
	}
}

func TestARRGateObjects_ExcludesDashboardAndReportsSupport(t *testing.T) {
	if ModuleARRGate("crm:dashboard") {
		t.Error("crm:dashboard tak boleh jadi gate ARR — granted default di semua peran bawaan, akan meniadakan gunanya")
	}
	if ModuleARRGate("crm:reports_support") {
		t.Error("crm:reports_support tak boleh jadi gate ARR — halaman itu tak punya field finance apa pun")
	}
}

func TestModuleARRGate_UnknownObjFalse(t *testing.T) {
	if ModuleARRGate("crm:nonexistent") {
		t.Error("objek tak dikenal harus false")
	}
}

// TestModuleWriteEnforced_UnenforcedModules: audit 2026-09 — 7 modul ini TAK
// PERNAH dicek CanBusiness(ctx,obj,"write") di mana pun (hanya "read"), jadi
// editor (role_edit_levels.go) tak boleh menawarkan "Kelola" untuknya. Daftar
// dikunci persis agar penambahan enforcement "write" baru di masa depan WAJIB
// mengubah test ini dulu (menandakan modul itu perlu dipindah ke enforced).
func TestModuleWriteEnforced_UnenforcedModules(t *testing.T) {
	want := []string{
		"crm:dashboard",
		"crm:subscriptions",
		"crm:activities",
		"crm:reports_sales",
		"crm:reports_cs",
		"crm:reports_support",
		"crm:reports_subscriptions",
	}
	for _, obj := range want {
		if ModuleWriteEnforced(obj) {
			t.Errorf("%q harus WriteEnforced=false (tak ada titik enforcement write)", obj)
		}
	}
	gotFalse := 0
	for _, m := range CRMModules() {
		if !m.WriteEnforced {
			gotFalse++
		}
	}
	if gotFalse != len(want) {
		t.Errorf("harus persis %d modul WriteEnforced=false, got %d", len(want), gotFalse)
	}
}

// TestModuleWriteEnforced_EnforcedModulesSample: pembanding positif — beberapa
// modul dengan titik enforcement write nyata (Accounts, Deals, Renewals) harus
// tetap WriteEnforced=true, agar test di atas tak lolos dgn menandai SEMUA
// modul false.
func TestModuleWriteEnforced_EnforcedModulesSample(t *testing.T) {
	for _, obj := range []string{"crm:accounts", "crm:deals", "crm:renewals", "crm:members"} {
		if !ModuleWriteEnforced(obj) {
			t.Errorf("%q harus WriteEnforced=true", obj)
		}
	}
}

func TestModuleWriteEnforced_UnknownObjFalse(t *testing.T) {
	if ModuleWriteEnforced("crm:nonexistent") {
		t.Error("objek tak dikenal harus false (fail-closed)")
	}
}

// TestCRMModules_Members: BL-171 — crm:members ("User Management") harus valid
// sbg sel matriks (ValidModuleObj), boleh "Kelola" (WriteEnforced), TAPI tak
// punya approve/ARR (bukan Deals/Renewals/Subscriptions) & sengaja TAK ikut
// ARRGateObjects (bukan modul finansial).
func TestCRMModules_Members(t *testing.T) {
	if !ValidModuleObj("crm:members") {
		t.Fatal("crm:members harus jadi modul matriks yang sah")
	}
	if !ModuleWriteEnforced("crm:members") {
		t.Error("crm:members harus WriteEnforced=true (Kelola membuka aksi nyata)")
	}
	if ModuleCanApprove("crm:members") {
		t.Error("crm:members tak punya alur approve")
	}
	if ModuleCanARR("crm:members") {
		t.Error("crm:members tak punya kolom Lihat ARR")
	}
	if ModuleARRGate("crm:members") {
		t.Error("crm:members bukan modul finansial — tak boleh ikut ARRGateObjects")
	}
}
