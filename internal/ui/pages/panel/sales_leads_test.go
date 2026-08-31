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

// TestLeadForm_FieldNumerik — regresi BL-2: HP/WhatsApp/Nilai Estimasi harus
// "berasa angka" (inputmode numeric + pattern) TANPA type="number" (yang membuang
// leading zero & prefix +62). Estimasi bertanda data-numgroup (kait numgroup.js)
// & form memuat numgroup.js. Atribut ini jaring klien; backend tetap penjaga.
func TestLeadForm_FieldNumerik(t *testing.T) {
	out := renderLeads(t, LeadForm(LeadFormView{
		Base:        "/w/desa",
		Action:      "/w/desa/leads/new",
		Statuses:    []string{"New"},
		Ratings:     []string{"Hot"},
		RegionsJSON: "[]",
	}))

	for _, want := range []string{
		`inputmode="numeric"`,    // keypad angka mobile
		`data-numgroup`,          // kait pengelompokan ribuan (estimasi)
		`pattern="[0-9.]*"`,      // pola uang (titik ribuan ditoleransi)
		`pattern="[0-9+ -]*"`,    // pola telepon (digit/+/pemisah)
		`name="estimated_value"`, //
		`name="mobile_phone"`,    //
		`name="whatsapp"`,        //
		`/static/numgroup.js`,    // skrip format+normalisasi termuat
	} {
		if !strings.Contains(out, want) {
			t.Errorf("form lead harus memuat %q:\n%s", want, out)
		}
	}

	// type="number" TAK boleh dipakai (merusak leading zero & '+').
	if strings.Contains(out, `type="number"`) {
		t.Errorf("form lead TAK boleh pakai type=\"number\" utk field numerik:\n%s", out)
	}
}

// TestLeadForm_LegendaEnum — regresi BL-3: dropdown Status & Rating harus disertai
// legenda makna tiap opsi (pengguna baru tak tahu beda Contacted vs Qualified,
// Hot vs Cold). Legenda statis (bukan input user) → CSP-safe. Kita jaga bahwa
// makna kunci tiap enum ter-render bersama istilahnya.
func TestLeadForm_LegendaEnum(t *testing.T) {
	out := renderLeads(t, LeadForm(LeadFormView{
		Base:        "/w/desa",
		Action:      "/w/desa/leads/new",
		Statuses:    []string{"New", "Contacted", "Qualified", "Unqualified"},
		Ratings:     []string{"Hot", "Warm", "Cold"},
		RegionsJSON: "[]",
	}))

	// Istilah + potongan makna (tanpa '&' agar tak terpengaruh escape g.Text).
	for _, want := range []string{
		"Contacted:", "Sudah dihubungi",
		"Qualified:", "siap dikonversi",
		"Unqualified:", "Tak cocok",
		"Hot:", "siap closing",
		"Warm:", "perlu tindak lanjut",
		"Cold:", "Belum tertarik",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("legenda enum lead harus memuat %q:\n%s", want, out)
		}
	}
}
