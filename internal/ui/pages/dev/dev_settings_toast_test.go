package dev

import (
	"strings"
	"testing"
)

// dev_settings_toast_test.go — BL-156h: feedback err/ok di panel platform
// /dev (Settings, Workspaces, Workspace Detail, Users kuota) harus toast
// mengambang (ui.Toast: fixed+pointer-events:none+toast-flash), bukan lagi
// ui.Alert inline statis. Meniru pola accounts_toast_test.go (BL-156a) /
// customer_success_toast_test.go (BL-156g).

// assertToast didefinisikan LOKAL (bukan diimpor): tiap branch rollout lepas
// dari main sendiri-sendiri, jadi tak ada helper bersama antar branch.
func assertToast(t *testing.T, out, wantText, wantAlertClass string) {
	t.Helper()
	for _, want := range []string{"fixed", "pointer-events:none", "toast-flash", wantAlertClass, wantText} {
		if !strings.Contains(out, want) {
			t.Errorf("toast kurang %q:\n%s", want, out)
		}
	}
}

func TestSettings_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	Settings(SettingsView{Err: "Kuota harus antara 1 dan 100"}).Render(&errOut)
	Settings(SettingsView{Msg: "Pengaturan disimpan dan langsung berlaku"}).Render(&okOut)

	assertToast(t, errOut.String(), "Kuota harus antara 1 dan 100", "alert-error")
	assertToast(t, okOut.String(), "Pengaturan disimpan dan langsung berlaku", "alert-success")
}

func TestWorkspaces_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	Workspaces(nil, 1, 0, 20, "Alasan wajib diisi.").Render(&errOut)

	assertToast(t, errOut.String(), "Alasan wajib diisi.", "alert-error")
}

func TestWorkspaceDetail_ToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	WorkspaceDetail(WorkspaceDetailView{
		ID: 1, Name: "Acme", Slug: "acme",
		ErrMsg: "Workspace ini adalah rumah aplikasi — tak bisa diarsipkan maupun dihapus.",
	}).Render(&errOut)

	assertToast(t, errOut.String(),
		"Workspace ini adalah rumah aplikasi — tak bisa diarsipkan maupun dihapus.", "alert-error")
}

// TestUsersPage_QuotaErrToastNotAlert — jalur BARU (gap ditemukan sesi ini):
// DevUserQuota/DevUserQuotaReset redirect PRG native ke /dev/users?err=CODE,
// beda dari aksi role/status/hapus di halaman yang sama yang lewat SSE Flash().
// UsersView sebelumnya tak punya slot Err sama sekali → kode hilang senyap.
func TestUsersPage_QuotaErrToastNotAlert(t *testing.T) {
	var errOut strings.Builder
	UsersPage(UsersView{Err: "Kuota harus antara 1 dan 100"}).Render(&errOut)

	assertToast(t, errOut.String(), "Kuota harus antara 1 dan 100", "alert-error")
}
