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
