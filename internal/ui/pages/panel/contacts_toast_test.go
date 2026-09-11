package panel

import (
	"strings"
	"testing"
)

// TestContactsList_ToastNotAlert: BL-156b rollout — feedback err/ok kontak
// (nested & global) harus toast mengambang (ui.Toast: fixed+pointer-events:none+
// toast-flash), bukan lagi ui.Alert inline statis. Pola sama accounts_toast_test.go.
func TestContactsList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	ContactsList(ContactsListView{Err: "Nomor HP tidak valid."}).Render(&errOut)
	ContactsList(ContactsListView{Msg: "Kontak ditambahkan."}).Render(&okOut)

	for _, want := range []string{"fixed", "pointer-events:none", "toast-flash", "alert-error", "Nomor HP tidak valid."} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("toast err (nested) kurang %q:\n%s", want, errOut.String())
		}
	}
	for _, want := range []string{"fixed", "pointer-events:none", "toast-flash", "alert-success", "Kontak ditambahkan."} {
		if !strings.Contains(okOut.String(), want) {
			t.Errorf("toast ok (nested) kurang %q:\n%s", want, okOut.String())
		}
	}
}

// TestContactsAll_ToastNotAlert: sama seperti di atas, untuk daftar kontak
// GLOBAL (lintas-desa).
func TestContactsAll_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	ContactsAll(ContactsAllView{Err: "Nomor HP tidak valid."}).Render(&errOut)
	ContactsAll(ContactsAllView{Msg: "Kontak ditambahkan."}).Render(&okOut)

	for _, want := range []string{"fixed", "pointer-events:none", "toast-flash", "alert-error", "Nomor HP tidak valid."} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("toast err (global) kurang %q:\n%s", want, errOut.String())
		}
	}
	for _, want := range []string{"fixed", "pointer-events:none", "toast-flash", "alert-success", "Kontak ditambahkan."} {
		if !strings.Contains(okOut.String(), want) {
			t.Errorf("toast ok (global) kurang %q:\n%s", want, okOut.String())
		}
	}
}

// TestAccountForm_ToastNotAlert: form desa (BL-156b) — galat validasi via
// ?err= (PRG, lihat accounts.go/accounts_update.go) harus toast, bukan alert.
func TestAccountForm_ToastNotAlert(t *testing.T) {
	var out strings.Builder
	AccountForm(AccountFormView{Err: "Kode desa sudah dipakai."}).Render(&out)

	for _, want := range []string{"fixed", "pointer-events:none", "toast-flash", "alert-error", "Kode desa sudah dipakai."} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("toast form desa kurang %q:\n%s", want, out.String())
		}
	}
}

// TestContactForm_ToastNotAlert: form kontak (BL-156b) — galat validasi via
// ?err= harus toast, bukan alert.
func TestContactForm_ToastNotAlert(t *testing.T) {
	var out strings.Builder
	ContactForm(ContactFormView{Err: "Nomor HP tidak valid."}).Render(&out)

	for _, want := range []string{"fixed", "pointer-events:none", "toast-flash", "alert-error", "Nomor HP tidak valid."} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("toast form kontak kurang %q:\n%s", want, out.String())
		}
	}
}

// TestAccountDetail_ToastNotAlert dan TestContactDetail_ToastNotAlert: BL-156b
// — gap ditemukan saat verifikasi manual: create/update kontak redirect PRG ke
// halaman DETAIL (bukan daftar), tapi ContactDetailView sebelumnya tak punya
// slot Err/Msg sama sekali. Toast harus muncul di detail juga.
func TestContactDetail_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	ContactDetail(ContactDetailView{Err: "Gagal menyimpan."}).Render(&errOut)
	ContactDetail(ContactDetailView{Msg: "Kontak ditambahkan."}).Render(&okOut)

	for _, want := range []string{"fixed", "pointer-events:none", "toast-flash", "alert-error", "Gagal menyimpan."} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("toast err (detail) kurang %q:\n%s", want, errOut.String())
		}
	}
	for _, want := range []string{"fixed", "pointer-events:none", "toast-flash", "alert-success", "Kontak ditambahkan."} {
		if !strings.Contains(okOut.String(), want) {
			t.Errorf("toast ok (detail) kurang %q:\n%s", want, okOut.String())
		}
	}
}
