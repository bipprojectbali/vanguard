package panel

import (
	"strings"
	"testing"
)

// dealFormFixture = data form deal minimal untuk uji render (BL-9).
func dealFormFixture(edit bool) DealFormView {
	v := DealFormView{
		Base:   "/w/acme",
		Action: "/w/acme/deals",
		Types:  []string{"New Business", "Renewal"},
		Terms:  []string{"Bulanan", "Tahunan"},
		Accounts: []AccountMemberOption{
			{ID: 7, Label: "DSA-007 — Desa Sukamaju"},
			{ID: 12, Label: "DSA-012 — Desa Mekarsari"},
		},
	}
	if edit {
		v.IsEdit = true
		v.Action = "/w/acme/deals/99"
		v.Fields = DealFormFields{
			DealName:             "Langganan 2026",
			AccountID:            "12",
			SelectedAccountLabel: "DSA-012 — Desa Mekarsari",
		}
	}
	return v
}

// TestDealForm_DesaTypeahead: pemilih desa harus typeahead CSP-safe (datalist +
// input teks + hidden account_id + skrip same-origin), BUKAN <select> polos lagi.
func TestDealForm_DesaTypeahead(t *testing.T) {
	out := renderLeads(t, DealForm(dealFormFixture(false)))

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
		"Desa Sukamaju", "Desa Mekarsari", // label desa terlihat
		`/static/accountpicker.js`, // skrip typeahead same-origin dimuat
	} {
		if !strings.Contains(out, want) {
			t.Errorf("pemilih desa deal typeahead harus memuat %q:\n%s", want, out)
		}
	}

	// Regresi: account_id tak boleh lagi <select> polos.
	if strings.Contains(out, `<select id="f-account_id"`) {
		t.Errorf("account_id tak boleh lagi <select> polos:\n%s", out)
	}
}

// TestDealForm_EditPreselect: mode edit mengisi input teks dgn nama desa terpilih
// & hidden dgn id-nya, agar sync klien tak mengosongkan pilihan yang sah.
func TestDealForm_EditPreselect(t *testing.T) {
	out := renderLeads(t, DealForm(dealFormFixture(true)))

	// Hidden account_id preselect id 12; input teks tampak preselect labelnya.
	if !strings.Contains(out, `name="account_id" data-account-value="" value="12"`) {
		t.Errorf("hidden account_id harus preselect id 12:\n%s", out)
	}
	if !strings.Contains(out, `value="DSA-012 — Desa Mekarsari"`) {
		t.Errorf("input teks harus preselect nama desa terpilih:\n%s", out)
	}
}

// TestDealForm_EditPreselectBeyondList: desa terpilih di LUAR daftar picker harus
// tetap disisipkan ke datalist + input teks terisi labelnya (tak hilang).
func TestDealForm_EditPreselectBeyondList(t *testing.T) {
	v := dealFormFixture(true)
	v.Fields.AccountID = "999"
	v.Fields.SelectedAccountLabel = "DSA-999 — Desa Terpencil"
	out := renderLeads(t, DealForm(v))

	for _, want := range []string{
		`data-account-id="999"`,            // opsi terpilih disisipkan meski di luar daftar
		"Desa Terpencil",                   // labelnya muncul di datalist
		`value="DSA-999 — Desa Terpencil"`, // input teks terisi label preselect
	} {
		if !strings.Contains(out, want) {
			t.Errorf("preselect luar-daftar harus memuat %q:\n%s", want, out)
		}
	}
}

// TestDealForm_NoDefaultHelp: BL-85 — baris bantuan default "Ketik untuk
// mencari, lalu pilih desa…" tak lagi dirender di bawah pemilih Desa (picker
// tak menyetel Help eksplisit → tak ada fallback global).
func TestDealForm_NoDefaultHelp(t *testing.T) {
	out := renderLeads(t, DealForm(dealFormFixture(false)))
	if strings.Contains(out, "Ketik untuk mencari, lalu pilih desa dari daftar yang muncul.") {
		t.Errorf("baris bantuan default tak boleh dirender lagi (BL-85):\n%s", out)
	}
}
