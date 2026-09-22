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
