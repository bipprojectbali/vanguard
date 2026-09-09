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

const (
	arrValue  = "Rp 120.000.000"
	noteValue = "Kades sulit dihubungi; perpanjangan berisiko."
)

// TestFLS_ARR — terbuka bagi semua KECUALI Support. Manager SENGAJA lolos (§8.3:
// butuh angka untuk menyetujui diskon) — satu-satunya pengecualian FLS Manager.
func TestFLS_ARR(t *testing.T) {
	cases := []struct {
		role string
		see  bool
	}{
		{"admin", true},
		{"manager", true}, // pengecualian §8.3
		{"sales", true},
		{"csm", true},
		{"support", false}, // §5: agen tiket tak perlu nilai komersial
		{"", false},        // bukan pemegang peran CRM → deny-default
	}
	for _, c := range cases {
		got := maskARR(arrValue, c.role)
		if c.see && got != arrValue {
			t.Errorf("ARR untuk %q harus tampil utuh, got %q", c.role, got)
		}
		if !c.see && got == arrValue {
			t.Errorf("ARR untuk %q BOCOR — nilai asli lolos ke yang tak berhak", c.role)
		}
	}
}

// TestFLS_InternalNotes — HANYA Admin & CSM (§5). Non-berhak menerima string
// KOSONG, bukan penanda: keberadaan catatan itu sendiri sudah sinyal.
func TestFLS_InternalNotes(t *testing.T) {
	cases := []struct {
		role string
		see  bool
	}{
		{"admin", true},
		{"csm", true},
		{"sales", false},
		{"manager", false}, // §5 mengikat Manager juga
		{"support", false},
		{"", false},
	}
	for _, c := range cases {
		got := maskInternalNotes(noteValue, c.role)
		if c.see && got != noteValue {
			t.Errorf("catatan untuk %q harus tampil, got %q", c.role, got)
		}
		if !c.see {
			if got == noteValue {
				t.Errorf("catatan untuk %q BOCOR — penilaian internal lolos", c.role)
			}
			if got != "" {
				t.Errorf("catatan untuk %q harus KOSONG (bukan penanda), got %q", c.role, got)
			}
		}
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
// pun — wewenang platform ≠ melihat field komersial/PII (§3).
func TestFLS_TegakLurusRolePlatform(t *testing.T) {
	for _, role := range []string{"", "super_admin", "owner", "staff"} {
		if maskARR(arrValue, role) == arrValue && role != "" {
			// "super_admin" dll BUKAN business_role valid → harus tersembunyi.
			t.Errorf("ARR lolos untuk role non-bisnis %q — sumbu tercampur", role)
		}
		// Phone konfigurabel per-tenant: pada tenant belum-dikonfigurasi (tenantID 0),
		// default = Sales+Admin. Role platform (super_admin/owner/staff) & "" BUKAN
		// business_role → CanViewPhone false. Sumbu F4 tegak lurus dari role platform.
		if fls.CanViewPhone(0, role) {
			t.Errorf("HP lolos untuk role non-bisnis %q — sumbu tercampur", role)
		}
		if maskInternalNotes(noteValue, role) == noteValue {
			t.Errorf("catatan lolos untuk role non-bisnis %q — sumbu tercampur", role)
		}
	}
}
