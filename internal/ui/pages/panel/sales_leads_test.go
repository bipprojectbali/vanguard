package panel

import (
	"strings"
	"testing"

	g "maragu.dev/gomponents"
)

// sales_leads_test.go — regresi BL-1: tab "Lead Saya" redundan saat cakupan
// aktor 'own' (filter dasar sudah mengunci lead_owner=uid → "Semua" ≡ "Lead
// Saya"). Keputusan sembunyi/tampil diambil HANDLER (LeadsListView.HideMyTab);
// di sini kita jaga bahwa view menghormatinya. Render → assert string tab.

func renderLeads(t *testing.T, node g.Node) string {
	t.Helper()
	var sb strings.Builder
	if err := node.Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

// TestLeadsList_TabSaya_TampilSaatScopeAll: cakupan luas (HideMyTab=false) →
// ketiga tab tampil, termasuk "Lead Saya".
func TestLeadsList_TabSaya_TampilSaatScopeAll(t *testing.T) {
	out := renderLeads(t, LeadsList(LeadsListView{Base: "/w/desa", HideMyTab: false}))
	for _, want := range []string{"Semua", "Lead Saya", "Unqualified"} {
		if !strings.Contains(out, want) {
			t.Errorf("tab %q harus tampil saat HideMyTab=false:\n%s", want, out)
		}
	}
	// Link tab "my" harus ada.
	if !strings.Contains(out, "/leads?tab=my") {
		t.Errorf("link tab ?tab=my harus ada saat HideMyTab=false:\n%s", out)
	}
}

// TestLeadsList_TabSaya_SembunyiSaatScopeOwn: cakupan 'own' (HideMyTab=true) →
// tab "Lead Saya" hilang, tapi "Semua" & "Unqualified" tetap ada.
func TestLeadsList_TabSaya_SembunyiSaatScopeOwn(t *testing.T) {
	out := renderLeads(t, LeadsList(LeadsListView{Base: "/w/desa", HideMyTab: true}))
	if strings.Contains(out, "Lead Saya") {
		t.Errorf("tab \"Lead Saya\" TAK boleh tampil saat HideMyTab=true (redundan):\n%s", out)
	}
	if strings.Contains(out, "/leads?tab=my") {
		t.Errorf("link ?tab=my TAK boleh ada saat HideMyTab=true:\n%s", out)
	}
	for _, want := range []string{"Semua", "Unqualified"} {
		if !strings.Contains(out, want) {
			t.Errorf("tab %q harus tetap tampil saat HideMyTab=true:\n%s", want, out)
		}
	}
}
