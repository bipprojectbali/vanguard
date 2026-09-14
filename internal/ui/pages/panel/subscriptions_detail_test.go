package panel

import (
	"strings"
	"testing"
)

// subscriptions_detail_test.go — view-layer (BL-154): grid 2-kolom + 3 kartu
// baru (Status & Lifecycle, Renewal, System & Audit). Fixture SubDetailView
// dirakit langsung (murni-data, tak butuh handler) — pola sama
// TestDealDetail_SystemAuditCard di sales_deals_detail_test.go. "&" di judul
// kartu di-escape g.Text jadi "&amp;" — assert bentuk yang benar dirender.

// TestSubDetail_CardsGrid: grid luar mobile-first (1→2 kolom) dan kelima kartu
// (Identitas, Status & Lifecycle, Financials, Renewal, System & Audit) render
// judulnya. Field ❌ yang DIPUTUSKAN drop dari v1 (Setup Fee/Payment Method/
// Last Invoice, docs/crm/tasks.md BL-154) tak boleh muncul di struct maupun body.
func TestSubDetail_CardsGrid(t *testing.T) {
	v := SubDetailView{Base: "/w/acme", ID: 1, AccountID: 9, Village: "Desa Uji"}
	out := renderLeads(t, SubDetail(v))

	if !strings.Contains(out, "md:grid-cols-2") {
		t.Errorf("grid kartu detail harus mobile-first (1 kolom < md, 2 kolom md+):\n%s", out)
	}
	for _, title := range []string{
		"Identitas &amp; Langganan", "Status &amp; Lifecycle", "Financials",
		"Renewal", "System &amp; Audit",
	} {
		if !strings.Contains(out, title) {
			t.Errorf("kartu %q harus render:\n%s", title, out)
		}
	}
	for _, dropped := range []string{"Setup Fee", "Payment Method", "Metode Pembayaran", "Last Invoice", "Invoice Terakhir"} {
		if strings.Contains(out, dropped) {
			t.Errorf("field %q sudah DIPUTUSKAN drop dari v1 (BL-154), tak boleh muncul:\n%s", dropped, out)
		}
	}
}

// TestSubStatusLifecycleCard_HealthBadge: badge Health hanya dirender saat
// HealthLabel terisi (baris customer_success ada); kosong → "—" polos, tanpa
// badge kelas apa pun bocor ke output.
func TestSubStatusLifecycleCard_HealthBadge(t *testing.T) {
	withHealth := SubDetailView{HealthLabel: "Sehat", HealthBadgeClass: "badge-success"}
	out := renderLeads(t, subStatusLifecycleCard(withHealth))
	if !strings.Contains(out, `class="badge badge-success"`) || !strings.Contains(out, "Sehat") {
		t.Errorf("Health terisi harus render badge badge-success 'Sehat':\n%s", out)
	}

	empty := SubDetailView{}
	out = renderLeads(t, subStatusLifecycleCard(empty))
	if strings.Contains(out, "badge-success") || strings.Contains(out, "badge-warning") || strings.Contains(out, "badge-error") {
		t.Errorf("tanpa Health, tak boleh ada badge status Health bocor:\n%s", out)
	}
}

// TestSubStatusLifecycleCard_Suspended: "Ditangguhkan?" hanya "Ya" saat status
// persis "Suspended" (turunan boolean, bukan kolom DB tersendiri).
func TestSubStatusLifecycleCard_Suspended(t *testing.T) {
	cases := map[string]string{"Suspended": "Ya", "Active": "Tidak", "Trial": "Tidak"}
	for status, want := range cases {
		out := renderLeads(t, subStatusLifecycleCard(SubDetailView{Status: status}))
		if !strings.Contains(out, want) {
			t.Errorf("status %q → Ditangguhkan=%q harus tampil:\n%s", status, want, out)
		}
	}
}

// TestSubRenewalCard_StatusBadge: badge Status Renewal render kelas dari
// RenewalStatusClass (REUSE derivasi handler); kosong → fallback badge-ghost
// via "—" (bukan kelas kosong tanpa render).
func TestSubRenewalCard_StatusBadge(t *testing.T) {
	v := SubDetailView{RenewalStatusLabel: "Akan Jatuh Tempo", RenewalStatusClass: "badge badge-warning"}
	out := renderLeads(t, subRenewalCard(v))
	if !strings.Contains(out, `class="badge badge-warning"`) || !strings.Contains(out, "Akan Jatuh Tempo") {
		t.Errorf("badge Status Renewal harus pakai kelas dari RenewalStatusClass:\n%s", out)
	}

	empty := renderLeads(t, subRenewalCard(SubDetailView{}))
	if !strings.Contains(empty, "badge badge-ghost") {
		t.Errorf("RenewalStatusClass kosong harus fallback badge-ghost:\n%s", empty)
	}
}

// TestSubSystemAuditCard_SourceLink: SourceDealHref terisi → "Sumber" jadi
// tautan (link link-hover) ke href tsb; kosong → teks polos "—" tanpa tautan.
func TestSubSystemAuditCard_SourceLink(t *testing.T) {
	withDeal := SubDetailView{SourceDealLabel: "DEAL-0007", SourceDealHref: "/w/acme/deals/7"}
	out := renderLeads(t, subSystemAuditCard(withDeal))
	if !strings.Contains(out, `href="/w/acme/deals/7"`) || !strings.Contains(out, "DEAL-0007") {
		t.Errorf("Sumber harus menaut ke deal asal saat SourceDealHref terisi:\n%s", out)
	}

	noDeal := SubDetailView{}
	out = renderLeads(t, subSystemAuditCard(noDeal))
	if strings.Contains(out, `href="/w/acme/deals/`) {
		t.Errorf("tanpa SourceDealHref, 'Sumber' tak boleh menaut ke deal apa pun:\n%s", out)
	}
}
