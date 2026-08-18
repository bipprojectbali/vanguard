package handler

import (
	"strings"
	"testing"
)

// fls_test.go — Field-Level Security per business_role (sistem-dan-role.md §5, §8.3).
// Yang dijaga, bila rusak, tak terlihat di layar tapi bocor di source: nilai
// sensitif yang lolos ke role yang tak berhak. Maka setiap kasus menegakkan DUA
// arah — yang berhak menerima nilai ASLI, yang tak berhak TIDAK PERNAH menerimanya.

const (
	arrValue   = "Rp 120.000.000"
	phoneValue = "0812-3456-7890"
	noteValue  = "Kades sulit dihubungi; perpanjangan berisiko."
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

// TestFLS_Phone — utuh HANYA untuk Sales; semua role lain (termasuk Admin &
// Manager, §8.3) tersamar. Nomor kosong tetap kosong.
func TestFLS_Phone(t *testing.T) {
	cases := []struct {
		role string
		full bool
	}{
		{"sales", true},
		{"admin", false},
		{"manager", false}, // §8.3: nomor HP tetap tersamar bagi Manager
		{"csm", false},
		{"support", false},
		{"", false},
	}
	for _, c := range cases {
		got := maskPhone(phoneValue, c.role)
		if c.full && got != phoneValue {
			t.Errorf("HP untuk %q harus utuh, got %q", c.role, got)
		}
		if !c.full && got == phoneValue {
			t.Errorf("HP untuk %q BOCOR — PII lolos ke role non-Sales", c.role)
		}
	}
	if got := maskPhone("", "support"); got != "" {
		t.Errorf("nomor kosong harus tetap kosong, got %q", got)
	}
}

// TestFLS_Phone_TakBocorkanDigit: penyamaran = sembunyikan PENUH. Membocorkan
// digit awal/akhir tetap membocorkan nomor + panjangnya (§5: batasi sebaran PII).
func TestFLS_Phone_TakBocorkanDigit(t *testing.T) {
	got := maskPhone(phoneValue, "support")
	for _, d := range []string{"0812", "7890", "3456"} {
		if strings.Contains(got, d) {
			t.Errorf("HP tersamar %q masih memuat potongan digit %q", got, d)
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

// TestFLS_SubscriptionARR — kebijakan KHUSUS subscriptions (spec M5-4,
// subscriptions_view.go): ARR hanya admin+manager, BEDA dari canSeeARR umum
// (yang meloloskan sales & csm juga). Sales/CSM tetap lihat MRR (canSeeARR)
// tapi TIDAK ARR subscription — dua kebijakan berlainan pada dua field yang
// terlihat mirip. Predikat ini sebelumnya tanpa test predikat langsung (hanya
// tersentuh lewat test HTTP di dashboard_test.go/subscriptions_test.go);
// gap M9-2.
func TestFLS_SubscriptionARR(t *testing.T) {
	cases := []struct {
		role string
		see  bool
	}{
		{"admin", true},
		{"manager", true},
		{"sales", false}, // BEDA dari canSeeARR umum: sales lihat MRR, bukan ARR
		{"csm", false},   // BEDA dari canSeeARR umum: csm lihat MRR, bukan ARR
		{"support", false},
		{"", false},
	}
	for _, c := range cases {
		gotCan := canSeeSubscriptionARR(c.role)
		if gotCan != c.see {
			t.Errorf("canSeeSubscriptionARR(%q) = %v, want %v", c.role, gotCan, c.see)
		}
		got := maskSubscriptionARR(arrValue, c.role)
		if c.see && got != arrValue {
			t.Errorf("ARR subscription utk %q harus tampil utuh, got %q", c.role, got)
		}
		if !c.see && got == arrValue {
			t.Errorf("ARR subscription utk %q BOCOR — nilai asli lolos ke yang tak berhak", c.role)
		}
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
		if maskPhone(phoneValue, role) == phoneValue {
			t.Errorf("HP lolos untuk role non-bisnis %q — sumbu tercampur", role)
		}
		if maskInternalNotes(noteValue, role) == noteValue {
			t.Errorf("catatan lolos untuk role non-bisnis %q — sumbu tercampur", role)
		}
	}
}
