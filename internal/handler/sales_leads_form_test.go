package handler

import "testing"

// sales_leads_form_test.go — unit parseLeadForm untuk BL-2 (field numerik lead).
// Menjaga kontrak backend sebagai penjaga sesungguhnya: Nilai Estimasi menerima
// input terkelompok "5.000.000" (dari numgroup.js maupun ketikan manual) sebagai
// 5000000; HP/WhatsApp mempertahankan leading zero & prefix +62 apa adanya, tapi
// menolak yang bukan-nomor. parseLeadForm murni (tak sentuh DB) → uji langsung.

// fvFromMap membangun fungsi pembaca form dari map (nilai tak ada → "").
func fvFromMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestParseLeadForm_EstimasiTerkelompokDiterima(t *testing.T) {
	cases := map[string]struct {
		in   string
		want string // numericStr yang diharapkan ("" = NULL)
	}{
		"terkelompok titik": {"5.000.000", "5000000"},
		"digit polos":       {"5000000", "5000000"},
		"dengan spasi":      {"1 500 000", "1500000"},
		"kosong → NULL":     {"", ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f, code := parseLeadForm(fvFromMap(map[string]string{
				"lead_name":       "Desa Contoh",
				"lead_status":     "New",
				"estimated_value": c.in,
			}))
			if code != "" {
				t.Fatalf("estimasi %q ditolak (code=%q), harusnya diterima", c.in, code)
			}
			if got := numericStr(f.EstimatedValue); got != c.want {
				t.Errorf("estimasi %q → numericStr=%q, mau %q", c.in, got, c.want)
			}
		})
	}
}

func TestParseLeadForm_EstimasiBukanAngkaDitolak(t *testing.T) {
	_, code := parseLeadForm(fvFromMap(map[string]string{
		"lead_name":       "Desa Contoh",
		"lead_status":     "New",
		"estimated_value": "abc",
	}))
	if code != "estimated" {
		t.Errorf("estimasi non-angka: code=%q, mau %q", code, "estimated")
	}
}

func TestParseLeadForm_TeleponFormatDipertahankan(t *testing.T) {
	cases := map[string]struct{ mobile, wa string }{
		"leading zero + prefix +62": {"0812345678", "+62812345678"},
		"dengan pemisah":            {"0812-3456-789", "+62 812 3456 789"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f, code := parseLeadForm(fvFromMap(map[string]string{
				"lead_name":    "Desa Contoh",
				"lead_status":  "New",
				"mobile_phone": c.mobile,
				"whatsapp":     c.wa,
			}))
			if code != "" {
				t.Fatalf("telepon %q/%q ditolak (code=%q)", c.mobile, c.wa, code)
			}
			if f.MobilePhone == nil || *f.MobilePhone != c.mobile {
				t.Errorf("mobile_phone tak dipertahankan: %v, mau %q", f.MobilePhone, c.mobile)
			}
			if f.Whatsapp == nil || *f.Whatsapp != c.wa {
				t.Errorf("whatsapp tak dipertahankan: %v, mau %q", f.Whatsapp, c.wa)
			}
		})
	}
}

func TestParseLeadForm_TeleponKosongJadiNull(t *testing.T) {
	f, code := parseLeadForm(fvFromMap(map[string]string{
		"lead_name":   "Desa Contoh",
		"lead_status": "New",
	}))
	if code != "" {
		t.Fatalf("form tanpa telepon ditolak (code=%q)", code)
	}
	if f.MobilePhone != nil || f.Whatsapp != nil {
		t.Errorf("telepon kosong harus NULL: mobile=%v whatsapp=%v", f.MobilePhone, f.Whatsapp)
	}
}

func TestParseLeadForm_TeleponBukanNomorDitolak(t *testing.T) {
	// HP huruf → code "mobile_phone".
	if _, code := parseLeadForm(fvFromMap(map[string]string{
		"lead_name": "Desa Contoh", "lead_status": "New", "mobile_phone": "hub-saya",
	})); code != "mobile_phone" {
		t.Errorf("HP non-nomor: code=%q, mau %q", code, "mobile_phone")
	}
	// WhatsApp terlalu pendek (<6 digit) → code "whatsapp".
	if _, code := parseLeadForm(fvFromMap(map[string]string{
		"lead_name": "Desa Contoh", "lead_status": "New", "whatsapp": "123",
	})); code != "whatsapp" {
		t.Errorf("WA terlalu pendek: code=%q, mau %q", code, "whatsapp")
	}
	// '+' bukan di depan → ditolak.
	if _, code := parseLeadForm(fvFromMap(map[string]string{
		"lead_name": "Desa Contoh", "lead_status": "New", "mobile_phone": "0812+345678",
	})); code != "mobile_phone" {
		t.Errorf("'+' di tengah: code=%q, mau %q", code, "mobile_phone")
	}
}
