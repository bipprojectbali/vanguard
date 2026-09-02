package panel

import (
	"strings"
	"testing"
)

// success_plans_form_test.go — regresi render form Success Plan (BL-29).
//
// Mode create harus merender pemilih desa TYPEAHEAD (native <datalist> + input
// teks + hidden account_id) yang dipandu static/accountpicker.js — pola SAMA
// dengan BL-5 (Kontak) & BL-9 (Deal), CSP-safe tanpa lib pihak-ketiga. Mode
// edit TETAP menampilkan nama desa statis (read-only) tanpa picker/skrip.

func successPlanCreateFixture() SuccessPlanFormView {
	return SuccessPlanFormView{
		Base:   "/w/desa",
		Action: "/w/desa/success-plans",
		Accounts: []SuccessPlanAccountOption{
			{ID: 7, Name: "Desa Sukamaju"},
			{ID: 12, Name: "Desa Mekarsari"},
		},
		Members:  []SuccessPlanMemberOption{{ID: 3, Name: "Budi"}},
		Statuses: []string{"Draft", "Active", "Achieved", "At-Risk", "Cancelled"},
	}
}

func TestSuccessPlanForm_CreateDesaTypeahead(t *testing.T) {
	out := renderLeads(t, SuccessPlanForm(successPlanCreateFixture()))

	for _, want := range []string{
		`data-account-picker`,             // wadah pemilih
		`<datalist id="account-options">`, // sumber opsi native (CSP-safe)
		`list="account-options"`,          // input tampak terhubung ke datalist
		`data-account-search`,             // kait input ketik
		`name="account_id"`,               // nilai id yang ter-submit
		`type="hidden"`,                   // id disimpan di input tersembunyi
		`data-account-value`,              // kait input hidden
		`data-account-id="7"`,             // opsi membawa id numerik
		`data-account-id="12"`,
		"Desa Sukamaju", "Desa Mekarsari", // label desa tetap terlihat
		`/static/accountpicker.js`, // skrip typeahead same-origin dimuat
	} {
		if !strings.Contains(out, want) {
			t.Errorf("pemilih desa typeahead harus memuat %q:\n%s", want, out)
		}
	}

	// Regresi: account_id tak boleh lagi dirender sebagai <select> polos.
	if strings.Contains(out, `<select name="account_id"`) {
		t.Errorf("account_id tak boleh lagi <select> polos:\n%s", out)
	}
}

func TestSuccessPlanForm_EditStaysReadOnly(t *testing.T) {
	v := successPlanCreateFixture()
	v.Action = "/w/desa/success-plans/42"
	v.AccountName = "Desa Sukamaju" // AccountName terisi → mode edit
	v.Accounts = nil                // mode edit tak mengirim daftar desa
	out := renderLeads(t, SuccessPlanForm(v))

	// Mode edit: nama desa statis, tanpa picker maupun skrip typeahead.
	for _, unwanted := range []string{
		`data-account-picker`,
		`data-account-search`,
		`name="account_id"`,
		`/static/accountpicker.js`,
	} {
		if strings.Contains(out, unwanted) {
			t.Errorf("mode edit tak boleh merender %q (desa read-only):\n%s", unwanted, out)
		}
	}
	if !strings.Contains(out, "Desa Sukamaju") {
		t.Errorf("mode edit harus menampilkan nama desa statis:\n%s", out)
	}
}
