package panel

import (
	"strings"
	"testing"
)

// contacts_form_test.go — regresi BL-4: form kontak wajib menjelaskan makna SEMUA
// pilihan. Tiga dropdown (Jabatan Kategori, Peran, Kanal Pilihan) disertai legenda
// makna tiap opsi; empat checkbox penanda (Kontak Utama, Kontak Teknis, Opt-out
// email, Jangan hubungi) disertai keterangan efeknya. Semua statis (bukan input
// user) → CSP-safe. Render → assert istilah + potongan makna ter-render bersama.

// contactFormViewFixture = ContactFormView terisi opsi enum & penanda aktif,
// cukup untuk merender seluruh legenda & keterangan checkbox.
func contactFormViewFixture() ContactFormView {
	return ContactFormView{
		AccountBase:   "/w/desa/accounts/1",
		AccountName:   "Desa Contoh",
		Action:        "/w/desa/accounts/1/contacts/new",
		PhoneEditable: true,
		Positions:     []string{"Kepala Desa", "Sekdes", "Kaur", "Kasi", "Operator", "Bendahara", "BPD", "Lainnya"},
		Roles:         []string{"Decision Maker", "Influencer", "User", "Finance", "Gatekeeper"},
		Channels:      []string{"WhatsApp", "Telepon", "Email", "Kunjungan"},
	}
}

// TestContactForm_LegendaEnum — dropdown enum kontak harus disertai legenda makna
// tiap opsi (pengguna baru tak tahu beda Kaur vs Kasi, Influencer vs Gatekeeper,
// atau apa arti "Kanal Pilihan"). Jaga istilah + potongan maknanya ter-render.
func TestContactForm_LegendaEnum(t *testing.T) {
	out := renderLeads(t, ContactForm(contactFormViewFixture()))

	// Potongan tanpa karakter khusus (g.Text meng-escape &, <, >).
	for _, want := range []string{
		// Jabatan (kategori)
		"Kepala Desa:", "pengambil keputusan tertinggi",
		"Sekdes:", "koordinator administrasi",
		"Operator:", "Pengelola sistem/aplikasi desa",
		"BPD:", "unsur pengawas",
		// Peran (buying role)
		"Decision Maker:", "keputusan akhir pembelian",
		"Influencer:", "bukan penentu",
		"Gatekeeper:", "Penjaga akses",
		// Kanal Pilihan
		"WhatsApp:", "lewat WhatsApp",
		"Kunjungan:", "menemui langsung",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("legenda enum kontak harus memuat %q:\n%s", want, out)
		}
	}
}

// TestContactForm_KeteranganCheckbox — tiap checkbox penanda harus disertai
// keterangan efeknya (mencentang "Jangan hubungi" atau "Opt-out email" punya
// akibat nyata; "Kontak Utama" menghubungkan ke Deal). Jaga keterangan ter-render.
func TestContactForm_KeteranganCheckbox(t *testing.T) {
	out := renderLeads(t, ContactForm(contactFormViewFixture()))

	for _, want := range []string{
		"Penghubung utama desa",
		"kontak default saat lead dikonversi jadi Deal",
		"urusan teknis/implementasi produk",
		"kontak menolak email",
		"kontak minta tak dihubungi sama sekali",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("keterangan checkbox kontak harus memuat %q:\n%s", want, out)
		}
	}
}

// contactFormGlobalFixture = mode GLOBAL (Accounts terisi) → merender pemilih
// desa induk yang bisa diketik (BL-5). Beda dgn fixture nested di atas yang
// AccountBase-nya terisi & tanpa Accounts.
func contactFormGlobalFixture() ContactFormView {
	f := contactFormViewFixture()
	f.AccountBase = ""
	f.Action = "/w/desa/contacts"
	f.ListHref = "/w/desa/contacts"
	f.Accounts = []AccountOption{
		{ID: 7, Name: "Desa Sukamaju"},
		{ID: 12, Name: "Desa Mekarsari"},
	}
	return f
}

// TestContactForm_GlobalDesaTypeahead — di mode global, pemilih desa induk WAJIB
// bisa diketik/dicari (BL-5): input teks + <datalist> berisi nama desa + input
// hidden name="account_id" (nilai ter-submit) + skrip same-origin accountpicker.js.
// BUKAN lagi <select name="account_id"> polos.
func TestContactForm_GlobalDesaTypeahead(t *testing.T) {
	out := renderLeads(t, ContactForm(contactFormGlobalFixture()))

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
	if strings.Contains(out, `<select id="f-account_id"`) {
		t.Errorf("account_id tak boleh lagi <select> polos:\n%s", out)
	}
}
