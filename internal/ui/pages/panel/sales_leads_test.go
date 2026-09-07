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
		Base:          "/w/desa",
		Action:        "/w/desa/leads/new",
		Ratings:       []string{"Hot"},
		RegionsJSON:   "[]",
		PhoneEditable: true,
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

// TestLeadForm_LegendaRating — regresi BL-3 + BL-69: dropdown Rating disertai
// legenda makna tiap opsi (pengguna baru tak tahu beda Hot vs Cold) di balik ikon
// ⓘ tap-friendly di label (enumFieldHinted, pola BL-65). Legenda tetap statis
// (bukan input user) → CSP-safe. Sejak BL-83 legenda STATUS pindah ke kontrol
// "Ubah Status" di detail lead (uji terpisah), jadi form profil hanya berisi Rating.
func TestLeadForm_LegendaRating(t *testing.T) {
	out := renderLeads(t, LeadForm(LeadFormView{
		Base:        "/w/desa",
		Action:      "/w/desa/leads/new",
		Ratings:     []string{"Hot", "Warm", "Cold"},
		RegionsJSON: "[]",
	}))

	// (a) Istilah + potongan makna Rating tetap ada (terjangkau lewat reveal;
	// tanpa '&' agar tak terpengaruh escape g.Text).
	for _, want := range []string{
		"Hot:", "siap closing",
		"Warm:", "perlu tindak lanjut",
		"Cold:", "Belum tertarik",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("legenda Rating lead harus memuat %q:\n%s", want, out)
		}
	}

	// (b) BL-69: legenda di balik reveal ⓘ (details.hint-reveal + summary + ikon).
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
}

// TestLeadForm_TanpaStatus — BL-83: STATUS lead dipindah dari form profil ke
// kontrol "Ubah Status" tersendiri di detail lead. Form profil (Tambah/Sunting)
// TAK boleh lagi memuat select lead_status, textarea alasan Unqualified, maupun
// signal $leadstatus — agar sunting profil tak diam-diam menyentuh status.
func TestLeadForm_TanpaStatus(t *testing.T) {
	out := renderLeads(t, LeadForm(LeadFormView{
		Base:        "/w/desa",
		Action:      "/w/desa/leads/new",
		Ratings:     []string{"Hot", "Warm", "Cold"},
		RegionsJSON: "[]",
	}))

	for _, forbidden := range []string{
		`name="lead_status"`,        // select status
		`name="unqualified_reason"`, // textarea alasan
		`data-bind="leadstatus"`,    // binding signal status
	} {
		if strings.Contains(out, forbidden) {
			t.Errorf("BL-83: form profil lead TAK boleh lagi memuat %q (pindah ke kontrol status):\n%s",
				forbidden, out)
		}
	}
}

// TestLeadForm_PhoneLiveFilter — BL-81: HP & WhatsApp membawa kait
// data-phonenum (dibaca static/phonenum.js) yang menyaring karakter tak-diizinkan
// SAAT DIKETIK, dan form memuat skripnya. Perilaku ketik (menyaring input event)
// tak bisa di-unit-test tanpa DOM; di sini kita jaga kontraknya: kedua field
// telepon punya kait & skrip termuat. Backend optPhone tetap penolak saat submit.
func TestLeadForm_PhoneLiveFilter(t *testing.T) {
	out := renderLeads(t, LeadForm(LeadFormView{
		Base:          "/w/desa",
		Action:        "/w/desa/leads/new",
		Ratings:       []string{"Hot"},
		RegionsJSON:   "[]",
		PhoneEditable: true,
	}))

	if !strings.Contains(out, "/static/phonenum.js") {
		t.Errorf("form lead harus memuat skrip penyaring telepon /static/phonenum.js:\n%s", out)
	}
	// Kait data-phonenum WAJIB ada di KEDUA field telepon (HP + WhatsApp) → 2×.
	if n := strings.Count(out, "data-phonenum"); n != 2 {
		t.Errorf("kait data-phonenum harus muncul 2× (HP + WhatsApp), dapat %d:\n%s", n, out)
	}
	// BL-2 dipertahankan: tetap tanpa type="number".
	if strings.Contains(out, `type="number"`) {
		t.Errorf("form lead TAK boleh pakai type=\"number\":\n%s", out)
	}
}

// TestLeadForm_SumberLeadDropdown — BL-82: "Sumber Lead" jadi dropdown enum
// terkunci (bukan input teks bebas). Render harus memuat <select
// name="lead_source"> berisi 7 opsi + opsi kosong (opsional). Backend
// (parseLeadForm) tetap penegak himpunan.
func TestLeadForm_SumberLeadDropdown(t *testing.T) {
	sources := []string{"Referral", "Event", "Website", "Cold Call", "Tender", "Dinas PMD", "Lainnya"}
	out := renderLeads(t, LeadForm(LeadFormView{
		Base:        "/w/desa",
		Action:      "/w/desa/leads/new",
		Ratings:     []string{"Hot"},
		Sources:     sources,
		RegionsJSON: "[]",
	}))

	// Harus <select> utk lead_source, BUKAN <input type=text name="lead_source">.
	if !strings.Contains(out, `name="lead_source"`) {
		t.Fatalf("field lead_source harus ada:\n%s", out)
	}
	if strings.Contains(out, `<input`) && strings.Contains(out, `name="lead_source" type="text"`) {
		t.Errorf("lead_source TAK boleh lagi input teks bebas:\n%s", out)
	}
	// Ketujuh opsi ter-render.
	for _, s := range sources {
		if !strings.Contains(out, `<option value="`+s+`"`) {
			t.Errorf("opsi Sumber Lead %q harus ter-render:\n%s", s, out)
		}
	}
}

// TestLeadForm_PhoneLocked — BL-84: bila PhoneEditable=false (role tak berhak
// sunting nomor, mis. Manager), field HP/WhatsApp DIKUNCI: input disabled TANPA
// atribut name (mask "•••" tak ikut ter-submit) & tanpa kait data-phonenum.
// Mencegah bug submit mask gagal validasi (parseLeadForm menolak "•••"). Nilai
// mask tetap tampil (terbaca), keterangan menjelaskan pembatasan.
func TestLeadForm_PhoneLocked(t *testing.T) {
	out := renderLeads(t, LeadForm(LeadFormView{
		Base:          "/w/desa",
		Action:        "/w/desa/leads/1",
		IsEdit:        true,
		Ratings:       []string{"Hot"},
		RegionsJSON:   "[]",
		PhoneEditable: false,
		Fields:        LeadFormFields{MobilePhone: "•••", Whatsapp: "•••"},
	}))

	if strings.Contains(out, `name="mobile_phone"`) || strings.Contains(out, `name="whatsapp"`) {
		t.Errorf("HP/WA terkunci TAK boleh ber-name (mask akan ter-submit → BL-84):\n%s", out)
	}
	if strings.Contains(out, "data-phonenum") {
		t.Errorf("HP/WA terkunci tak perlu kait data-phonenum (tak dapat diketik):\n%s", out)
	}
	if n := strings.Count(out, "disabled"); n < 2 {
		t.Errorf("kedua field telepon terkunci harus disabled (≥2), dapat %d:\n%s", n, out)
	}
}
