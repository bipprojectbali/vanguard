package panel

import (
	"strings"
	"testing"
)

// role_edit_test.go — BL-145 subtask 7: halaman detail peran KUSTOM jadi SATU
// form (identitas+matriks+Pengaturan Tambahan+Field Security) + kartu "Zona
// Berbahaya" (Hapus Peran) TERPISAH. Yang dijaga:
//
//   - Matriks (<thead>) tak lagi punya kolom "Setujui"/"Lihat ARR".
//   - "Pengaturan Tambahan" muncul saat ada approve/arr/FLS, TIDAK saat kosong.
//   - Satu <form> mencakup input matriks DAN checkbox view/edit + fsec_present.
//   - Zona Berbahaya (Hapus Peran) render sbg <form> & kartu TERPISAH.

func sampleCustomRoleCard() RoleCard {
	return RoleCard{
		Name: "finance", DisplayName: "Keuangan", DataScope: "own",
		Modules: []RoleModulePerm{
			{Obj: "crm:accounts", Label: "Accounts", Level: "read"},
			{Obj: "crm:renewals", Label: "Renewals", Level: "write", CanApprove: true, Approve: true},
			{Obj: "crm:subscriptions", Label: "Subscriptions", Level: "write", CanARR: true, ARR: true},
			{Obj: "crm:contacts", Label: "Kontak", Level: "write"},
			{Obj: "crm:leads", Label: "Prospek", Level: "write"},
		},
	}
}

func TestRoleEdit_MatrixHeaderHasNoApproveOrARRColumn(t *testing.T) {
	var out strings.Builder
	RoleEdit("/w/acme", sampleCustomRoleCard(), nil, true, "", "", nil).Render(&out)
	body := out.String()
	if !strings.Contains(body, "Modul") || !strings.Contains(body, "Akses") {
		t.Fatal("header matriks harus tetap punya Modul & Akses")
	}
	if strings.Contains(body, "Setujui") {
		t.Error("kolom 'Setujui' tak boleh lagi ada di matriks — pindah ke Pengaturan Tambahan")
	}
	if !strings.Contains(body, "MRR dan ARR") {
		t.Fatal("prasyarat: label 'Lihat MRR dan ARR pelanggan' harus tetap muncul (di Pengaturan Tambahan)")
	}
}

func TestRoleEdit_AdditionalSettingsRendersApproveAndARR(t *testing.T) {
	var out strings.Builder
	RoleEdit("/w/acme", sampleCustomRoleCard(), nil, true, "", "", nil).Render(&out)
	body := out.String()
	if !strings.Contains(body, "Pengaturan Tambahan") {
		t.Fatal("heading 'Pengaturan Tambahan' harus muncul saat ada approve/arr")
	}
	if !strings.Contains(body, "Keuangan") {
		t.Error("sub-judul 'Keuangan' harus mengelompokkan approve/ARR")
	}
	if !strings.Contains(body, "Boleh menyetujui Renewals") {
		t.Error("checklist approve Renewals harus muncul")
	}
	if !strings.Contains(body, "Lihat MRR dan ARR pelanggan") {
		t.Error("checklist ARR harus berlabel 'Lihat MRR dan ARR pelanggan' (bukan lagi diselipi nama modul)")
	}
	if strings.Contains(body, "Nilai Kontrak/MRR") {
		t.Error("catatan 'Nilai Kontrak/MRR' terpisah harus sudah dibuang (perbaikan tampilan 2026-09-17)")
	}
}

func TestRoleEdit_AdditionalSettingsAbsentWhenNoApproveNoARRNoFsec(t *testing.T) {
	rc := RoleCard{
		Name: "support", DisplayName: "Dukungan", DataScope: "own",
		Modules: []RoleModulePerm{
			{Obj: "crm:accounts", Label: "Accounts", Level: "read"},
		},
	}
	var out strings.Builder
	RoleEdit("/w/acme", rc, nil, true, "", "", nil).Render(&out)
	if strings.Contains(out.String(), "Pengaturan Tambahan") {
		t.Error("heading 'Pengaturan Tambahan' tak boleh muncul tanpa approve/arr/FLS")
	}
}

func TestRoleEdit_MergedFormIncludesMatrixAndFieldSecurity(t *testing.T) {
	fsec := &FieldSecurityRoleView{
		Base: "/w/acme", Name: "finance", CanEdit: true,
		CanViewPhone: true, CanEditPhone: false, Reactive: true,
	}
	var out strings.Builder
	RoleEdit("/w/acme", sampleCustomRoleCard(), nil, true, "", "", fsec).Render(&out)
	body := out.String()

	if !strings.Contains(body, `action="/w/acme/roles/finance"`) {
		t.Fatal("form gabungan harus POST ke /roles/{name}")
	}
	if !strings.Contains(body, `name="level.crm:accounts"`) {
		t.Error("form gabungan harus memuat input matriks")
	}
	if !strings.Contains(body, `name="view"`) || !strings.Contains(body, `name="edit"`) {
		t.Error("form gabungan harus memuat checkbox view/edit Field Security")
	}
	if !strings.Contains(body, `name="fsec_present"`) || !strings.Contains(body, `value="1"`) {
		t.Error("form gabungan harus memuat sentinel hidden fsec_present=1")
	}
	if !strings.Contains(body, "Kontak") {
		t.Error("sub-judul 'Kontak' harus mengelompokkan checklist Field Security")
	}
	if strings.Contains(body, "Quotes mengikuti izin Deals") {
		t.Error("catatan 'Quotes mengikuti izin Deals' harus sudah dibuang (label modul kini 'Deals dan Quotes')")
	}
	// HANYA SATU <form> di halaman (form gabungan) + form Hapus Peran terpisah = dua total.
	if got := strings.Count(body, "<form"); got != 2 {
		t.Errorf("harus persis 2 <form> (gabungan + hapus peran), got %d", got)
	}
}

func TestRoleEdit_FieldSecurityAbsentWhenFsecNil(t *testing.T) {
	var out strings.Builder
	RoleEdit("/w/acme", sampleCustomRoleCard(), nil, true, "", "", nil).Render(&out)
	body := out.String()
	if strings.Contains(body, `name="fsec_present"`) {
		t.Error("fsec nil: sentinel fsec_present tak boleh dirender sama sekali")
	}
	if strings.Contains(body, "Lihat nomor HP/WhatsApp penuh") {
		t.Error("fsec nil: checklist Field Security tak boleh dirender")
	}
}

func TestRoleEdit_ZonaBerbahayaIsSeparateCardAndForm(t *testing.T) {
	var out strings.Builder
	RoleEdit("/w/acme", sampleCustomRoleCard(), nil, true, "", "", nil).Render(&out)
	body := out.String()

	if !strings.Contains(body, "Zona Berbahaya") {
		t.Fatal("harus ada heading 'Zona Berbahaya'")
	}
	if !strings.Contains(body, `action="/w/acme/roles/finance/delete"`) {
		t.Error("form Hapus Peran harus POST ke /roles/{name}/delete, terpisah dari form sunting")
	}
	// Dua card .card bg-base-100 terpisah: form sunting & Zona Berbahaya.
	if got := strings.Count(body, "card bg-base-100"); got != 2 {
		t.Errorf("harus persis 2 card terpisah (sunting + zona berbahaya), got %d", got)
	}
}

func TestRoleEdit_ZonaBerbahayaHiddenWhenNotEditable(t *testing.T) {
	var out strings.Builder
	RoleEdit("/w/acme", sampleCustomRoleCard(), nil, false, "", "", nil).Render(&out)
	body := out.String()
	if strings.Contains(body, "Zona Berbahaya") {
		t.Error("canEdit=false: Zona Berbahaya (hapus peran) tak boleh dirender")
	}
	if strings.Contains(body, "/delete") {
		t.Error("canEdit=false: form hapus peran tak boleh dirender")
	}
}

func TestRoleEdit_SystemRoleKeepsSeparateFieldSecurityForm(t *testing.T) {
	rc := RoleCard{Name: "admin", DisplayName: "Admin", IsSystem: true}
	fsec := &FieldSecurityRoleView{
		Base: "/w/acme", Name: "admin", CanEdit: true,
		CanViewPhone: true, CanEditPhone: true, Reactive: false,
	}
	var out strings.Builder
	RoleEdit("/w/acme", rc, nil, true, "", "", fsec).Render(&out)
	body := out.String()
	if !strings.Contains(body, `action="/w/acme/roles/admin/field-security"`) {
		t.Error("peran sistem harus tetap punya form Field Security TERPISAH")
	}
	if strings.Contains(body, "Pengaturan Tambahan") {
		t.Error("peran sistem tak punya matriks — tak boleh ada 'Pengaturan Tambahan'")
	}
}
