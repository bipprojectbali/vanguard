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

// TestLeadForm_LegendaEnum — regresi BL-3 + BL-69: dropdown Status & Rating tetap
// disertai legenda makna tiap opsi (pengguna baru tak tahu beda Contacted vs
// Qualified, Hot vs Cold), TAPI sejak BL-69 legenda pindah ke balik ikon ⓘ
// tap-friendly di label (enumFieldHinted, pola BL-65) — bukan lagi baris statis di
// bawah select. Legenda tetap statis (bukan input user) → CSP-safe. Kita jaga
// bahwa (a) makna kunci tiap enum tetap ter-render (terjangkau lewat reveal) &
// (b) ia berada di dalam reveal label, di ATAS select-nya.
func TestLeadForm_LegendaEnum(t *testing.T) {
	out := renderLeads(t, LeadForm(LeadFormView{
		Base:        "/w/desa",
		Action:      "/w/desa/leads/new",
		Statuses:    []string{"New", "Contacted", "Qualified", "Unqualified"},
		Ratings:     []string{"Hot", "Warm", "Cold"},
		RegionsJSON: "[]",
	}))

	// (a) Istilah + potongan makna tetap ada (terjangkau; tanpa '&' agar tak
	// terpengaruh escape g.Text).
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

	// (b) BL-69: legenda kini di balik reveal ⓘ (details.hint-reveal + summary +
	// ikon), bukan baris statis di bawah select.
	for _, want := range []string{
		`class="hint-reveal`,  // pembungkus reveal
		`class="hint-summary`, // baris label yg bisa di-tap
		`aria-hidden="true"`,  // ikon ⓘ (dekoratif)
		`size-4`,              // lucide.Info
	} {
		if !strings.Contains(out, want) {
			t.Errorf("BL-69: reveal ikon ⓘ harus memuat %q:\n%s", want, out)
		}
	}

	// (b') Struktur: makna Status berada SEBELUM select lead_status (di dalam
	// reveal label, bukan baris statis SESUDAH select seperti enumField lama).
	iMakna := strings.Index(out, "Sudah dihubungi")
	iSelect := strings.Index(out, `name="lead_status"`)
	if iMakna < 0 || iSelect < 0 || iMakna > iSelect {
		t.Errorf("BL-69: legenda Status harus di dalam reveal label (sebelum select), "+
			"bukan baris statis di bawahnya (iMakna=%d, iSelect=%d):\n%s",
			iMakna, iSelect, out)
	}
}

// TestLeadForm_AlasanUnqualifiedKondisional — regresi BL-80: field "Alasan
// Unqualified" hanya tampil saat Status = Unqualified. Diwujudkan Datastar:
// <select> Status di-bind ke signal $leadstatus (data.Bind), textarea alasan
// dibungkus data-show="$leadstatus == 'Unqualified'", dan signal diinisialisasi
// dari nilai tersimpan (no-FOUC). data-show = jaring UX klien; backend tetap
// penegak (uji terpisah di handler). Kita jaga markup reaktifnya ter-render.
func TestLeadForm_AlasanUnqualifiedKondisional(t *testing.T) {
	out := renderLeads(t, LeadForm(LeadFormView{
		Base:        "/w/desa",
		Action:      "/w/desa/leads/new",
		Statuses:    []string{"New", "Contacted", "Qualified", "Unqualified"},
		Ratings:     []string{"Hot", "Warm", "Cold"},
		RegionsJSON: "[]",
	}))

	for _, want := range []string{
		`data-bind="leadstatus"`, // select Status menyetir $leadstatus
		`data-signals=`,          // signal $leadstatus diinisialisasi di form
		// textarea alasan dibungkus data-show pada ekspresi status Unqualified
		// (kutip di-escape g.Text jadi &#39;).
		`data-show="$leadstatus == &#39;Unqualified&#39;"`,
		`name="unqualified_reason"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("BL-80: form lead harus memuat %q:\n%s", want, out)
		}
	}

	// Struktur: pembungkus data-show berada SEBELUM textarea unqualified_reason
	// (textarea ada DI DALAM region kondisional, bukan di luarnya).
	iShow := strings.Index(out, `data-show="$leadstatus == &#39;Unqualified&#39;"`)
	iTextarea := strings.Index(out, `name="unqualified_reason"`)
	if iShow < 0 || iTextarea < 0 || iShow > iTextarea {
		t.Errorf("BL-80: textarea alasan harus di dalam pembungkus data-show "+
			"(iShow=%d, iTextarea=%d):\n%s", iShow, iTextarea, out)
	}
}
