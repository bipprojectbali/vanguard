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
