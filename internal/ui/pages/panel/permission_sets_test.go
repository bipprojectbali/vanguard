package panel

import (
	"strings"
	"testing"
)

// permission_sets_test.go — render blok B (Permission Sets) & blok C (Record
// Ownership Rules), BL-145 subtask 6. Murni-data: view-model dirakit langsung
// di sini (tak lewat handler/DB) — pola sepadan roles_members_toast_test.go.

// permView membangun PermissionSetView minimal siap-render untuk test.
func permView() PermissionSetView {
	return PermissionSetView{
		Base: "/w/acme",
		Roles: []RoleSwitchOption{
			{Name: "admin", DisplayName: "Administrator", Selected: false},
			{Name: "sales", DisplayName: "Sales", Selected: true},
		},
		SelectedRole:       "Sales",
		SelectedScopeValue: "own",
		SelectedScopeLabel: "Hanya desa yang ditugaskan",
		Rows: []PermissionSetRow{
			{Label: "Accounts (Desa)", View: PermMarkOwnOnly, Create: PermMarkOwnOnly, Edit: PermMarkOwnOnly, Delete: PermMarkOwnOnly},
			{Label: "Playbooks", View: PermMarkNone, Create: PermMarkNone, Edit: PermMarkNone, Delete: PermMarkNone},
		},
	}
}

func TestPermissionSetsSection_MarksAndModal(t *testing.T) {
	var out strings.Builder
	if err := permissionSetsSection(permView()).Render(&out); err != nil {
		t.Fatalf("render: %v", err)
	}
	body := out.String()

	if !strings.Contains(body, `id="permission-sets"`) {
		t.Error("container harus id=permission-sets (target anchor blok A)")
	}
	if !strings.Contains(body, `class="modal"`) {
		t.Error("container harus class=modal (trigger :target daisyUI, tanpa JS baru)")
	}
	if !strings.Contains(body, "Peran: Sales") {
		t.Error("heading statis harus menyebut peran tersorot (Sales)")
	}
	if strings.Contains(body, "<details") || strings.Contains(body, `?role=admin`) {
		t.Error("role switcher (details/summary + link ?role=X) harus sudah dicabut")
	}
	if !strings.Contains(body, "Kelola (Buat/Ubah/Hapus)") {
		t.Error("header kolom harus diringkas jadi Kelola (Buat/Ubah/Hapus)")
	}
	if strings.Contains(body, ">Buat<") || strings.Contains(body, ">Ubah<") || strings.Contains(body, ">Hapus<") {
		t.Error("kolom Buat/Ubah/Hapus terpisah tak boleh tersisa sbg header sendiri")
	}
	if !strings.Contains(body, "Accounts (Desa)") || !strings.Contains(body, "Playbooks") {
		t.Error("baris modul harus dirender")
	}
	if !strings.Contains(body, string(PermMarkOwnOnly)) {
		t.Error("sel granted+own harus tanda ~")
	}
	if !strings.Contains(body, `href="#"`) {
		t.Error("tombol Tutup harus link href=# (melepas :target, tanpa JS)")
	}

	psIdx := strings.Index(body, `id="permission-sets"`)
	rorIdx := strings.Index(body, "Record Ownership Rules")
	if psIdx < 0 || rorIdx < 0 || rorIdx < psIdx {
		t.Error("Record Ownership Rules (blok C) harus dirender setelah container modal dibuka (di dalamnya)")
	}
}

func TestRecordOwnershipCard_TextPerScope(t *testing.T) {
	cases := []struct {
		scope, want string
	}{
		{"all", "SEMUA desa workspace"},
		{"own", "DITUGASKAN padanya"},
		{"none", "tak melihat data lewat kepemilikan desa"},
	}
	for _, c := range cases {
		t.Run(c.scope, func(t *testing.T) {
			var out strings.Builder
			if err := RecordOwnershipCard(c.scope, "Label X").Render(&out); err != nil {
				t.Fatalf("render: %v", err)
			}
			body := out.String()
			if !strings.Contains(body, "Label X") {
				t.Error("badge harus menampilkan label cakupan yang dioper")
			}
			if !strings.Contains(body, c.want) {
				t.Errorf("scope %q harus menyebut %q, got:\n%s", c.scope, c.want, body)
			}
		})
	}
}
