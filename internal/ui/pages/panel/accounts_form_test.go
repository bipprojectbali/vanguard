package panel

import (
	"strings"
	"testing"
)

// accounts_form_test.go — regresi BL-60: penyederhanaan form Tambah/Sunting Desa.
//   (1) input "Kode Sistem" (entity_code) dilepas dari UI (kode dipakai =
//       village_code Kemendagri; entity_code tetap otomatis di backend).
//   (2) field "Teritori" dilepas dari UI (data lama dipertahankan handler).
//   (3) Anggaran (APBDes) jadi input UANG: prefix "Rp" + data-numgroup +
//       inputmode numeric, dan form memuat numgroup.js. BUKAN teks bebas.

func renderAccountForm(t *testing.T, v AccountFormView) string {
	t.Helper()
	var sb strings.Builder
	if err := AccountForm(v).Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

// baseFormView = view minimal utk render form (tambah).
func baseFormView() AccountFormView {
	return AccountFormView{
		Base:            "/w/desa",
		Action:          "/w/desa/accounts",
		IsEdit:          false,
		RegionsJSON:     "[]",
		PhoneEditable:   true,
		Types:           []string{"prospect", "customer"},
		Statuses:        []string{"Desa"},
		Classifications: []string{"Maju"},
	}
}

// TestAccountForm_DropsSystemCodeAndTerritory: input kode sistem & teritori TAK
// boleh ada di form (add maupun edit).
func TestAccountForm_DropsSystemCodeAndTerritory(t *testing.T) {
	for _, isEdit := range []bool{false, true} {
		v := baseFormView()
		v.IsEdit = isEdit
		out := renderAccountForm(t, v)

		for _, banned := range []string{
			`name="entity_code"`, // input kode sistem
			"Kode Sistem",        // label kode sistem
			`name="territory"`,   // input teritori
			"Teritori",           // label teritori
		} {
			if strings.Contains(out, banned) {
				t.Errorf("isEdit=%v: form TAK boleh memuat %q (dilepas BL-60):\n%s", isEdit, banned, out)
			}
		}
	}
}

// TestAccountForm_BudgetIsMoneyField: Anggaran (APBDes) = input uang ber-prefix
// "Rp" + kait pengelompokan ribuan, form memuat numgroup.js, dan field itu tak
// memakai type="number" (yang menolak nilai terformat).
func TestAccountForm_BudgetIsMoneyField(t *testing.T) {
	out := renderAccountForm(t, baseFormView())

	for _, want := range []string{
		`name="village_budget"`, // field anggaran tetap ada
		`data-numgroup`,         // kait pengelompokan ribuan
		`inputmode="numeric"`,   // keypad angka mobile
		`pattern="[0-9.]*"`,     // pola uang (titik ribuan ditoleransi)
		">Rp<",                  // afiks Rp di kiri input
		`/static/numgroup.js`,   // skrip format+normalisasi termuat
	} {
		if !strings.Contains(out, want) {
			t.Errorf("form harus memuat %q utk Anggaran (APBDes):\n%s", want, out)
		}
	}
}

// TestAccountForm_BudgetPrefillNoDecimal: nilai prefill anggaran dirender apa
// adanya (digit polos, tanpa ".00") — numgroup.js membuang non-digit, jadi
// ".00" akan salah tampil. Handler memberi moneyRupiahStr; view hanya menaruh.
func TestAccountForm_BudgetPrefillNoDecimal(t *testing.T) {
	v := baseFormView()
	v.IsEdit = true
	v.Fields.VillageBudget = "750000000" // seperti moneyRupiahStr hasilkan
	out := renderAccountForm(t, v)

	if !strings.Contains(out, `value="750000000"`) {
		t.Errorf("prefill anggaran harus digit polos:\n%s", out)
	}
	if strings.Contains(out, `value="750000000.00"`) {
		t.Errorf("prefill anggaran TAK boleh membawa desimal (.00):\n%s", out)
	}
}
