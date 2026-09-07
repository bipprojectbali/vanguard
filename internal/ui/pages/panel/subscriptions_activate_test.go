package panel

import (
	"strings"
	"testing"
)

// subscriptions_activate_test.go — regresi BL-73: tombol "Aktifkan Langganan" di
// kartu aksi detail langganan. Tampil HANYA saat status Trial & CanActivate; tak
// pernah tampil di status lain, dan tak muncul tanpa kapabilitas (view murni-data,
// flag sudah dihitung handler).

func renderSubAction(t *testing.T, v SubDetailView) string {
	t.Helper()
	var sb strings.Builder
	if err := subActionCard(v).Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

// TestSubActivate_ShownWhenTrialAndGated: Trial + CanActivate → tombol & form POST
// /activate dirender.
func TestSubActivate_ShownWhenTrialAndGated(t *testing.T) {
	out := renderSubAction(t, SubDetailView{
		Base: "/w/desa", ID: 9, Status: "Trial", CanActivate: true,
	})
	if !strings.Contains(out, "Aktifkan Langganan") {
		t.Errorf("tombol Aktifkan Langganan harus tampil saat Trial+gate:\n%s", out)
	}
	if !strings.Contains(out, `action="/w/desa/subscriptions/9/activate"`) {
		t.Errorf("form harus POST ke /activate:\n%s", out)
	}
}

// TestSubActivate_HiddenWhenNotTrial: status Active (bukan Trial) → tombol aktivasi
// TAK dirender walau CanActivate true (aksi pasti ditolak backend sub_not_trial).
func TestSubActivate_HiddenWhenNotTrial(t *testing.T) {
	out := renderSubAction(t, SubDetailView{
		Base: "/w/desa", ID: 9, Status: "Active", CanActivate: true,
	})
	if strings.Contains(out, "Aktifkan Langganan") {
		t.Errorf("tombol Aktifkan Langganan TAK boleh tampil saat status Active:\n%s", out)
	}
}

// TestSubActivate_HiddenWhenNotGated: Trial tetapi CanActivate false → tombol tak
// dirender (tanpa kapabilitas crm:renewals write).
func TestSubActivate_HiddenWhenNotGated(t *testing.T) {
	out := renderSubAction(t, SubDetailView{
		Base: "/w/desa", ID: 9, Status: "Trial", CanActivate: false,
	})
	if strings.Contains(out, "Aktifkan Langganan") {
		t.Errorf("tombol Aktifkan Langganan TAK boleh tampil tanpa gate:\n%s", out)
	}
}
