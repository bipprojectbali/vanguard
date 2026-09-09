package panel

import (
	"strings"
	"testing"
)

// sales_convert_test.go — BL-122: field "Nilai (Rp)" di halaman konversi memakai
// moneyField (pengelompokan ribuan data-numgroup) selaras form Lead & Deal, dan
// numgroup.js dimuat agar tampilan terformat + ternormalisasi saat submit.

func TestLeadConvert_NilaiBerformatRibuan(t *testing.T) {
	v := LeadConvertView{
		Base:         "/w/desa",
		Action:       "/w/desa/leads/7/convert",
		BackURL:      "/w/desa/leads/7",
		LeadName:     "Desa Contoh",
		AccountTypes: []string{"Desa"},
		Fields:       ConvertFormFields{DealName: "Deal Contoh", Amount: "5000000"},
	}
	out := renderLeads(t, LeadConvert(v))

	for _, want := range []string{
		`name="amount"`, // field nilai deal
		`data-numgroup`, // kait pengelompokan ribuan (moneyField)
		`inputmode="numeric"`,
		`/static/numgroup.js`, // script pemformat dimuat
	} {
		if !strings.Contains(out, want) {
			t.Errorf("halaman konversi harus memuat %q utk Nilai berformat ribuan:\n%s", want, out)
		}
	}
}
