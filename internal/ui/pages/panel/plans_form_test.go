package panel

import (
	"strings"
	"testing"
)

// plans_form_test.go — regresi BL-89: form Plan (Tambah/Sunting) merender Harga
// Dasar & Biaya Setup sebagai moneyField (data-numgroup) DAN memuat numgroup.js.
// Tanpa skrip itu, reformat() (yang membuang non-digit tiap input) tak pernah
// jalan → field terasa menerima huruf di desktop sampai ditolak backend saat
// submit. Ini jaring klien; backend cleanThousands/optNumeric tetap penjaga.

func renderPlanForm(t *testing.T, v PlanFormView) string {
	t.Helper()
	var sb strings.Builder
	if err := PlanForm(v).Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

func basePlanFormView() PlanFormView {
	return PlanFormView{
		Base:           "/w/acme",
		Action:         "/w/acme/plans",
		Categories:     []string{"Langganan"},
		BillingOptions: []string{"monthly", "yearly"},
	}
}

// TestPlanForm_PriceFieldsAreMoneyFields: Harga Dasar & Biaya Setup = input uang
// (type text + inputmode numeric + pattern + data-numgroup), BUKAN teks bebas
// atau type="number" (yang menolak nilai terformat "5.000.000").
func TestPlanForm_PriceFieldsAreMoneyFields(t *testing.T) {
	out := renderPlanForm(t, basePlanFormView())

	for _, want := range []string{
		`name="base_price"`,   // Harga Dasar tetap ada
		`name="setup_fee"`,    // Biaya Setup tetap ada
		`inputmode="numeric"`, // keypad angka mobile
		`pattern="[0-9.]*"`,   // pola uang (titik ribuan ditoleransi)
	} {
		if !strings.Contains(out, want) {
			t.Errorf("form Plan harus memuat %q utk field harga:\n%s", want, out)
		}
	}

	// Kedua field harga membawa kait pengelompokan ribuan.
	if n := strings.Count(out, "data-numgroup"); n != 2 {
		t.Errorf("harap 2 field data-numgroup (base_price+setup_fee), dapat %d:\n%s", n, out)
	}

	// type="number" akan menolak leading zero / nilai terformat — pastikan tak dipakai.
	if strings.Contains(out, `name="base_price" type="number"`) ||
		strings.Contains(out, `name="setup_fee" type="number"`) {
		t.Errorf("field harga tak boleh type=number (menolak nilai terformat):\n%s", out)
	}
}

// TestPlanForm_LoadsNumgroupScript: form WAJIB memuat /static/numgroup.js — inti
// perbaikan BL-89. Skrip inilah yang membuang non-digit saat diketik.
func TestPlanForm_LoadsNumgroupScript(t *testing.T) {
	for _, edit := range []bool{false, true} {
		v := basePlanFormView()
		v.IsEdit = edit
		out := renderPlanForm(t, v)
		if !strings.Contains(out, "/static/numgroup.js") {
			t.Errorf("form Plan (IsEdit=%v) harus memuat /static/numgroup.js (BL-89):\n%s", edit, out)
		}
	}
}
