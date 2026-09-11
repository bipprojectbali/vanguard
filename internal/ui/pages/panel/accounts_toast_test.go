package panel

import (
	"strings"
	"testing"
)

// TestAccountsList_ToastNotAlert: BL-156a POC — feedback err/ok akun harus
// toast mengambang (ui.Toast: fixed+pointer-events:none+toast-flash), bukan
// lagi ui.Alert inline statis.
func TestAccountsList_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	AccountsList(AccountsListView{Err: "Kode desa sudah dipakai."}).Render(&errOut)
	AccountsList(AccountsListView{Msg: "Desa ditambahkan."}).Render(&okOut)

	for _, want := range []string{"fixed", "pointer-events:none", "toast-flash", "alert-error", "Kode desa sudah dipakai."} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("toast err kurang %q:\n%s", want, errOut.String())
		}
	}
	for _, want := range []string{"fixed", "pointer-events:none", "toast-flash", "alert-success", "Desa ditambahkan."} {
		if !strings.Contains(okOut.String(), want) {
			t.Errorf("toast ok kurang %q:\n%s", want, okOut.String())
		}
	}
}

// TestAccountDetail_ToastNotAlert: BL-156b — gap ditemukan saat verifikasi
// manual: create/update desa redirect PRG ke halaman DETAIL (bukan daftar),
// tapi AccountDetailView sebelumnya tak punya slot Err/Msg sama sekali (bahkan
// alert lama pun tak pernah tampil di sini). Toast harus muncul di detail juga.
func TestAccountDetail_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	AccountDetail(AccountDetailView{Err: "Gagal menyimpan."}).Render(&errOut)
	AccountDetail(AccountDetailView{Msg: "Desa ditambahkan."}).Render(&okOut)

	for _, want := range []string{"fixed", "pointer-events:none", "toast-flash", "alert-error", "Gagal menyimpan."} {
		if !strings.Contains(errOut.String(), want) {
			t.Errorf("toast err (detail) kurang %q:\n%s", want, errOut.String())
		}
	}
	for _, want := range []string{"fixed", "pointer-events:none", "toast-flash", "alert-success", "Desa ditambahkan."} {
		if !strings.Contains(okOut.String(), want) {
			t.Errorf("toast ok (detail) kurang %q:\n%s", want, okOut.String())
		}
	}
}
