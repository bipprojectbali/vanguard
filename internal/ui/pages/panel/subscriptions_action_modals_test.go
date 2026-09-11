package panel

import (
	"strings"
	"testing"
)

// subscriptions_action_modals_test.go — regresi BL-125: tiap aksi detail langganan
// (Perpanjang/Approve/Activate/Churn) = tombol pemicu (di header) + modal CSP-safe
// (checkbox-toggle), bukan kartu "Tindakan" berisi form selalu-tampil. Form di
// dalam modal tetap NATIVE POST → 303.

// TestSubActions_RenewOpensModal: status Active + CanRenew → tombol "Perpanjang" =
// <label for=sub-renew>, checkbox modal-toggle #sub-renew ada, form POST /renew.
func TestSubActions_RenewOpensModal(t *testing.T) {
	out := renderSubAction(t, SubDetailView{
		Base: "/w/desa", ID: 9, Status: "Active", CanRenew: true,
	})
	if !strings.Contains(out, `for="sub-renew"`) {
		t.Errorf("pemicu Perpanjang harus <label for=sub-renew>:\n%s", out)
	}
	if !strings.Contains(out, `id="sub-renew"`) || !strings.Contains(out, "modal-toggle") {
		t.Errorf("modal renew (checkbox-toggle) harus dirender:\n%s", out)
	}
	if !strings.Contains(out, `action="/w/desa/subscriptions/9/renew"`) {
		t.Errorf("form modal harus POST ke /renew:\n%s", out)
	}
}

// TestSubActions_ApproveModalHasBothDecisions: PendingApproval + CanApprove → satu
// pemicu tinjau + modal berisi DUA form terpisah (approve & reject).
func TestSubActions_ApproveModalHasBothDecisions(t *testing.T) {
	out := renderSubAction(t, SubDetailView{
		Base: "/w/desa", ID: 9, Status: "PendingApproval", CanApprove: true,
	})
	if !strings.Contains(out, `for="sub-approve"`) || !strings.Contains(out, `id="sub-approve"`) {
		t.Errorf("pemicu & modal approve harus dirender:\n%s", out)
	}
	if !strings.Contains(out, `action="/w/desa/subscriptions/9/approve"`) ||
		!strings.Contains(out, `action="/w/desa/subscriptions/9/reject"`) {
		t.Errorf("modal approve harus punya form /approve DAN /reject:\n%s", out)
	}
}

// TestSubActions_ChurnOpensModal: Active + CanChurn → pemicu "Tandai Churn" + modal
// #sub-churn + form POST /churn.
func TestSubActions_ChurnOpensModal(t *testing.T) {
	out := renderSubAction(t, SubDetailView{
		Base: "/w/desa", ID: 9, Status: "Active", CanChurn: true,
	})
	if !strings.Contains(out, `for="sub-churn"`) || !strings.Contains(out, `id="sub-churn"`) {
		t.Errorf("pemicu & modal churn harus dirender:\n%s", out)
	}
	if !strings.Contains(out, `action="/w/desa/subscriptions/9/churn"`) {
		t.Errorf("form modal harus POST ke /churn:\n%s", out)
	}
	// BL-153: alasan/tipe churn stack 1 kolom penuh — bukan lagi 2 kolom sempit.
	if strings.Contains(out, "sm:grid-cols-2") {
		t.Errorf("modal churn tak boleh lagi pakai sm:grid-cols-2 (BL-153):\n%s", out)
	}
	// BL-153: label Alasan/Tipe churn harus punya tap-info ⓘ (pola BL-65/BL-69).
	for _, want := range []string{
		`class="hint-reveal`,
		`class="hint-summary`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("modal churn harus memakai pola ikon ⓘ tap (%s, BL-153):\n%s", want, out)
		}
	}
}

// TestSubActions_HiddenActionNoTriggerNoModal: aksi yang tak diizinkan tak
// memunculkan pemicu MAUPUN modal (view murni-data). Active tanpa flag → tak ada
// pemicu dan tak ada modal-toggle sama sekali.
func TestSubActions_HiddenActionNoTriggerNoModal(t *testing.T) {
	if got := len(subActionTriggers(SubDetailView{Base: "/w/desa", ID: 9, Status: "Active"})); got != 0 {
		t.Errorf("tanpa kapabilitas tak boleh ada pemicu, dapat %d", got)
	}
	out := renderSubAction(t, SubDetailView{
		Base: "/w/desa", ID: 9, Status: "Active",
	})
	if strings.Contains(out, "modal-toggle") {
		t.Errorf("tanpa kapabilitas, tak boleh ada modal sama sekali:\n%s", out)
	}
}
