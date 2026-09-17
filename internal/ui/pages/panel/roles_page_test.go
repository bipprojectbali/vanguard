package panel

import (
	"strings"
	"testing"
)

// roles_page_test.go — render halaman /roles: kolom "Permission Set" + aksi
// Lihat/Edit/Hapus tabel peran (blok A), dan gerbang render blok B/C lewat
// nil-guard psv (BL-145 subtask 6). Field Security & Nilai Kontrak/MRR (dulu
// blok D di sini) pindah ke halaman detail peran (BL-145 subtask 3) — lihat
// field_security_test.go/contract_value_test.go.

func sampleRoleRows() []RoleRow {
	return []RoleRow{
		{Name: "admin", DisplayName: "Administrator", ScopeLabel: "Semua desa di workspace", MemberCount: 1, IsSystem: true},
		{Name: "finance", DisplayName: "Keuangan", Description: "Peran kustom", ScopeLabel: "Hanya desa yang ditugaskan", MemberCount: 0, IsSystem: false},
	}
}

// TestRolesTable_PermissionSetColumnAndActions: kolom badge Permission Set +
// aksi Lihat (anchor blok B)/Edit (halaman detail)/Hapus (kustom saja).
func TestRolesTable_PermissionSetColumnAndActions(t *testing.T) {
	var out strings.Builder
	err := Roles("/w/acme", sampleRoleRows(), nil, true, "", "", nil).Render(&out)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	body := out.String()

	if !strings.Contains(body, "Permission Set") {
		t.Error("header tabel harus memuat kolom Permission Set")
	}
	if !strings.Contains(body, "Hanya desa yang ditugaskan") {
		t.Error("badge Permission Set harus menampilkan ScopeLabel baris")
	}
	if !strings.Contains(body, `href="/w/acme/roles?role=finance#permission-sets"`) {
		t.Error("aksi Lihat harus link ke ?role=X#permission-sets")
	}
	if !strings.Contains(body, `href="/w/acme/roles/finance"`) {
		t.Error("aksi Edit harus link ke halaman detail")
	}
	if !strings.Contains(body, ">Lihat<") || !strings.Contains(body, ">Edit<") {
		t.Error("label aksi harus Lihat & Edit (bukan lagi Detail)")
	}
	if strings.Contains(body, ">Detail<") {
		t.Error("label lama Detail tak boleh tersisa")
	}
}

// TestRolesTable_HapusOnlyCustomWhenEditable: Hapus hanya muncul untuk peran
// kustom saat canEdit; peran sistem & workspace read-only tak boleh punya form
// hapus sama sekali (bukan disembunyikan CSS).
func TestRolesTable_HapusOnlyCustomWhenEditable(t *testing.T) {
	var editable, readOnly strings.Builder
	if err := Roles("/w/acme", sampleRoleRows(), nil, true, "", "", nil).Render(&editable); err != nil {
		t.Fatalf("render editable: %v", err)
	}
	if err := Roles("/w/acme", sampleRoleRows(), nil, false, "", "", nil).Render(&readOnly); err != nil {
		t.Fatalf("render read-only: %v", err)
	}

	if !strings.Contains(editable.String(), `action="/w/acme/roles/finance/delete"`) {
		t.Error("peran kustom + canEdit harus punya form hapus")
	}
	if strings.Contains(editable.String(), `action="/w/acme/roles/admin/delete"`) {
		t.Error("peran sistem tak boleh punya form hapus")
	}
	if strings.Contains(readOnly.String(), "/delete") {
		t.Error("workspace read-only tak boleh punya form hapus sama sekali")
	}
}

// TestRoles_BlockNilGuards: psv nil → blok B/C tak dirender; terisi → blok
// tampil. Menjaga kontrak "nil = peninjau tak berwenang / data tak tersedia"
// yang dipakai handler.
func TestRoles_BlockNilGuards(t *testing.T) {
	rows := sampleRoleRows()

	var without strings.Builder
	if err := Roles("/w/acme", rows, nil, true, "", "", nil).Render(&without); err != nil {
		t.Fatalf("render tanpa blok: %v", err)
	}
	body := without.String()
	for _, absent := range []string{"Permission Sets", "Record Ownership Rules"} {
		if strings.Contains(body, absent) {
			t.Errorf("nil guard: %q tak boleh muncul saat view-model nil", absent)
		}
	}

	psv := &PermissionSetView{
		Base:               "/w/acme",
		Roles:              []RoleSwitchOption{{Name: "admin", DisplayName: "Administrator", Selected: true}},
		SelectedRole:       "Administrator",
		SelectedScopeValue: "all",
		SelectedScopeLabel: "Semua desa di workspace",
		Rows:               []PermissionSetRow{{Label: "Dashboard", View: PermMarkFull, Create: PermMarkFull, Edit: PermMarkFull, Delete: PermMarkFull}},
	}

	var with strings.Builder
	if err := Roles("/w/acme", rows, nil, true, "", "", psv).Render(&with); err != nil {
		t.Fatalf("render dgn blok: %v", err)
	}
	body = with.String()
	for _, present := range []string{"Permission Sets", "Record Ownership Rules"} {
		if !strings.Contains(body, present) {
			t.Errorf("dgn view-model terisi: %q harus muncul", present)
		}
	}
}
