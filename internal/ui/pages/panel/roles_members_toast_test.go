package panel

import (
	"strings"
	"testing"
)

// roles_members_toast_test.go — BL-156f: feedback err/ok di Roles, RoleEdit, dan
// Members harus toast mengambang (ui.Toast: fixed+pointer-events:none+
// toast-flash), bukan lagi ui.Alert inline statis. Meniru pola
// accounts_toast_test.go (BL-156a).

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

func TestRoles_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	Roles("/w/acme", nil, nil, true, "Nama peran itu sudah dipakai di workspace ini.", "", nil).Render(&errOut)
	Roles("/w/acme", nil, nil, true, "", "Peran ditambahkan.", nil).Render(&okOut)

	assertToast(t, errOut.String(), "Nama peran itu sudah dipakai di workspace ini.", "alert-error")
	assertToast(t, okOut.String(), "Peran ditambahkan.", "alert-success")
}

func TestRoleEdit_ToastNotAlert(t *testing.T) {
	rc := RoleCard{Name: "finance", DisplayName: "Keuangan"}
	var errOut, okOut strings.Builder
	RoleEdit("/w/acme", rc, nil, true, "Role tidak valid.", "").Render(&errOut)
	RoleEdit("/w/acme", rc, nil, true, "", "Peran disimpan.").Render(&okOut)

	assertToast(t, errOut.String(), "Role tidak valid.", "alert-error")
	assertToast(t, okOut.String(), "Peran disimpan.", "alert-success")
}

func TestMembers_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	Members("/w/acme", nil, nil, nil, true, 1, "Tak bisa menurunkan atau mengeluarkan owner terakhir workspace.", "").Render(&errOut)
	Members("/w/acme", nil, nil, nil, true, 1, "", "Peran CRM anggota diperbarui.").Render(&okOut)

	assertToast(t, errOut.String(), "Tak bisa menurunkan atau mengeluarkan owner terakhir workspace.", "alert-error")
	assertToast(t, okOut.String(), "Peran CRM anggota diperbarui.", "alert-success")
}
