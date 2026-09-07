package handler

import "testing"

// account_picker_label_test.go — BL-76: label picker Desa seragam.
// Verifikasi keputusan (i) village_code Kemendagri di depan, (ii) NULL → nama saja.
func TestAccountPickerLabel(t *testing.T) {
	str := func(s string) *string { return &s }
	empty := ""

	cases := []struct {
		name string
		code *string
		vill string
		want string
	}{
		{"village_code ada → 'kode — nama'", str("3201052008"), "Sukamaju", "3201052008 — Sukamaju"},
		{"village_code nil (akun lama) → nama saja", nil, "Cibeureum", "Cibeureum"},
		{"village_code string kosong → nama saja (tanpa em-dash menggantung)", &empty, "Mekarsari", "Mekarsari"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := accountPickerLabel(c.code, c.vill); got != c.want {
				t.Errorf("accountPickerLabel() = %q, mau %q", got, c.want)
			}
		})
	}
}
