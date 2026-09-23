package panel

import (
	"strings"
	"testing"
)

// roles_members_toast_test.go — BL-156f: feedback err/ok di Roles, RoleEdit, dan
// Members harus toast mengambang (ui.Toast: fixed+pointer-events:none+
// toast-flash), bukan lagi ui.Alert inline statis. Meniru pola
// accounts_toast_test.go (BL-156a).

// assertToast: lihat toast_assert_test.go (helper bersama paket ini).

func TestRoles_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	Roles("/w/acme", nil, nil, true, "Nama peran itu sudah dipakai di workspace ini.", "", nil).Render(&errOut)
	Roles("/w/acme", nil, nil, true, "", "Peran ditambahkan.", nil).Render(&okOut)

	assertToast(t, errOut.String(), "err", "Nama peran itu sudah dipakai di workspace ini.")
	assertToast(t, okOut.String(), "ok", "Peran ditambahkan.")
}

func TestRoleEdit_ToastNotAlert(t *testing.T) {
	rc := RoleCard{Name: "finance", DisplayName: "Keuangan"}
	var errOut, okOut strings.Builder
	RoleEdit("/w/acme", rc, nil, true, "Role tidak valid.", "", nil).Render(&errOut)
	RoleEdit("/w/acme", rc, nil, true, "", "Peran disimpan.", nil).Render(&okOut)

	assertToast(t, errOut.String(), "err", "Role tidak valid.")
	assertToast(t, okOut.String(), "ok", "Peran disimpan.")
}

func TestMembers_ToastNotAlert(t *testing.T) {
	var errOut, okOut strings.Builder
	Members("/w/acme", nil, nil, nil, nil, true, 1, "Tak bisa menurunkan atau mengeluarkan owner terakhir workspace.", "").Render(&errOut)
	Members("/w/acme", nil, nil, nil, nil, true, 1, "", "Peran CRM anggota diperbarui.").Render(&okOut)

	assertToast(t, errOut.String(), "err", "Tak bisa menurunkan atau mengeluarkan owner terakhir workspace.")
	assertToast(t, okOut.String(), "ok", "Peran CRM anggota diperbarui.")
}
