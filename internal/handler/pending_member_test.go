package handler

import "testing"

// pending_member_test.go — gerbang onboarding Opsi A (BL-105). Menguji keputusan
// murni "siapa yang menunggu peran" + bahwa navnya menyembunyikan seluruh menu
// CRM (hanya Dashboard). Session-context tak dirangkai: logika inti sengaja
// diekstrak jadi fungsi murni agar teruji tanpa middleware.

func TestPendingHolding(t *testing.T) {
	cases := []struct {
		name         string
		canManage    bool
		businessRole string
		want         bool
	}{
		{"member tanpa peran CRM → menunggu", false, "", true},
		{"member sudah punya peran → tidak menunggu", false, "sales", false},
		{"pengelola tanpa peran → tidak (dapat banner CRMOnboard)", true, "", false},
		{"pengelola dengan peran → tidak", true, "admin", false},
	}
	for _, c := range cases {
		if got := pendingHolding(c.canManage, c.businessRole); got != c.want {
			t.Errorf("%s: pendingHolding(%v, %q) = %v, mau %v",
				c.name, c.canManage, c.businessRole, got, c.want)
		}
	}
}

func TestPendingMemberNav_HanyaDashboard(t *testing.T) {
	nav := pendingMemberNav("desa")
	if len(nav) != 1 {
		t.Fatalf("anggota menunggu peran hanya boleh lihat 1 menu (Dashboard), dapat %d: %+v", len(nav), nav)
	}
	if nav[0].Label != "Dashboard" {
		t.Errorf("menu tunggal harus Dashboard, dapat %q", nav[0].Label)
	}
	if want := wsPath("desa", ""); nav[0].Href != want {
		t.Errorf("Href Dashboard harus %q, dapat %q", want, nav[0].Href)
	}
	// Tak boleh ada menu CRM apa pun (Accounts/Sales/Reports dst).
	for _, it := range nav {
		if it.Label != "Dashboard" {
			t.Errorf("menu CRM %q tak boleh muncul untuk anggota menunggu peran", it.Label)
		}
	}
}
