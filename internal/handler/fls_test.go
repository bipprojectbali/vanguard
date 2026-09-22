package handler

import (
	"testing"

	"go_starter/internal/fls"
)

// fls_test.go — Field-Level Security per business_role (sistem-dan-role.md §5, §8.3).
// Yang dijaga, bila rusak, tak terlihat di layar tapi bocor di source: nilai
// sensitif yang lolos ke role yang tak berhak. Maka setiap kasus menegakkan DUA
// arah — yang berhak menerima nilai ASLI, yang tak berhak TIDAK PERNAH menerimanya.
//
// FLS phone (HP/WhatsApp) kini konfigurabel per-tenant (BL-107) → kontrak default/
// override/fail-closed murni-nya diuji di internal/fls (tanpa ctx). Di sini yang
// tersisa untuk phone: ketegaklurusan sumbu (role platform tak pernah lolos default),
// dan bocor-digit level-render diuji end-to-end via harness (contacts/sales_leads).

const arrValue = "Rp 120.000.000"

// TestFLS_ARR — kontrak masker Nilai Kontrak/ARR/MRR murni-data (BL-169): canSee=true
// → nilai utuh, false → penanda tersembunyi (flsHidden), nilai asli tak pernah bocor.
// Sejak BL-169 canSeeARR = canSeeSubscriptionARR, KAPABILITAS ter-matriks (crm:
// subscriptions/arr, BL-58), bukan cek nama role — pemetaan role→canARR (default
// admin/manager/sales/csm lolos, support tidak, peran CUSTOM ber-grant lolos) diuji
// end-to-end di subscriptions_test.go/dashboard_subscription_charts_test.go. Di sini
// dikunci maskernya sendiri, murni-data, tanpa enforcer.
func TestFLS_ARR(t *testing.T) {
	if got := maskARR(arrValue, true); got != arrValue {
		t.Errorf("canSee=true: ARR harus tampil utuh, got %q", got)
	}
	if got := maskARR(arrValue, false); got != flsHidden {
		t.Errorf("canSee=false: ARR harus tersamar (%s), got %q", flsHidden, got)
	}
	if maskARR(arrValue, false) == arrValue {
		t.Error("canSee=false: nilai asli ARR BOCOR ke yang tak berhak")
	}
}

// TestFLS_SubscriptionARR — kontrak masker ARR subscription murni-data (BL-58):
// canSee=true → nilai utuh, false → penanda tersembunyi (flsHidden); nilai asli
// tak pernah bocor. Sejak BL-58 visibilitas ARR adalah KAPABILITAS ter-matriks
// (crm:subscriptions/arr), bukan cek nama role — predikatnya (canSeeSubscriptionARR)
// membaca enforcer dari ctx, jadi diuji end-to-end di subscriptions_test.go
// (peran bawaan admin/manager melihat; sales/csm/support tidak; peran CUSTOM
// ber-grant melihat). Di sini dikunci masker bool-nya sendiri — murni-data,
// tanpa enforcer.
func TestFLS_SubscriptionARR(t *testing.T) {
	if got := maskSubscriptionARR(arrValue, true); got != arrValue {
		t.Errorf("canSee=true: ARR harus tampil utuh, got %q", got)
	}
	if got := maskSubscriptionARR(arrValue, false); got != flsHidden {
		t.Errorf("canSee=false: ARR harus tersamar (%s), got %q", flsHidden, got)
	}
	if maskSubscriptionARR(arrValue, false) == arrValue {
		t.Error("canSee=false: nilai asli ARR BOCOR ke yang tak berhak")
	}
}

// TestFLS_TegakLurusRolePlatform: FLS memakai business_role, BUKAN role tenant/
// platform. super_admin/owner yang bukan pemegang peran CRM ("") tak lolos apa
// pun — wewenang platform ≠ melihat field komersial/PII (§3). ARR sejak BL-169
// adalah kapabilitas Casbin (canSeeARR/canSeeSubscriptionARR), bukan lagi cek
// nama role di sini — tegak-lurusnya dijamin structural (business_policy.csv
// tak punya baris utk role platform → business_role "" tak pernah match) &
// diuji end-to-end di subscriptions_test.go, bukan lewat maskARR (kini bool murni).
func TestFLS_TegakLurusRolePlatform(t *testing.T) {
	for _, role := range []string{"", "super_admin", "owner", "staff"} {
		// Phone konfigurabel per-tenant: pada tenant belum-dikonfigurasi (tenantID 0),
		// default = Sales+Admin. Role platform (super_admin/owner/staff) & "" BUKAN
		// business_role → CanViewPhone false. Sumbu F4 tegak lurus dari role platform.
		if fls.CanViewPhone(0, role) {
			t.Errorf("HP lolos untuk role non-bisnis %q — sumbu tercampur", role)
		}
	}
}
