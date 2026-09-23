package handler

import (
	"strings"
	"testing"

	"go_starter/internal/appmode"
	"go_starter/internal/authz"
)

// members_hide_role_test.go — BL-135: halaman anggota workspace tak lagi memuat
// select "Role" tenant per baris. Ubah role tenant kini HANYA lewat /dev/users;
// halaman workspace tinggal menyisakan sumbu CRM ("Peran CRM") + Simpan.
//
// Ini menjaga AKIBAT di HTML, bukan sekadar tampilan: select yang cuma
// disembunyikan CSS tetap terkirim ke browser dan tetap bisa di-POST. Tanda
// pembeda per-baris = `class="select select-sm" name="role"` (memberRoleSelect).
// Form UNDANG tak lagi punya select Role tenant sama sekali (BL-170 mengganti
// id=invite-role dengan id=invite-kind + Peran CRM cascading) — lihat
// TestMembers_UndangJenisAnggotaAda.

const perRowTenantSelect = `class="select select-sm" name="role"`

// TestMembers_TenantRoleSelectHilang: baris yang dapat dikelola tak lagi meng-emit
// select role tenant, sementara select "Peran CRM" tetap ada.
func TestMembers_TenantRoleSelectHilang(t *testing.T) {
	env, uid := setupTest(t)
	seedMemberNamed(t, env, "rekankerja@contoh.com", "Budi Santoso", authz.RoleNameMember)

	// Berlaku di KEDUA mode — penyembunyian ini soal UI kelola anggota, bukan
	// soal bentuk aplikasinya.
	for _, mode := range []appmode.Mode{appmode.Single, appmode.Multi} {
		withMode(t, mode, func() {
			html := renderMembers(t, env, uid, authz.RoleNameOwner)
			if strings.Contains(html, perRowTenantSelect) {
				t.Errorf("mode %v: select role tenant per baris masih terkirim — "+
					"ubah role tenant harusnya lewat /dev/users saja", mode)
			}
			if !strings.Contains(html, `name="business_role"`) {
				t.Errorf("mode %v: select Peran CRM harus tetap ada", mode)
			}
		})
	}
}

// TestMembers_UndangJenisAnggotaAda: BL-170 mengganti select Role tenant pada
// form Undang (id=invite-role, keputusan BL-135) dengan select Jenis Anggota
// (id=invite-kind) + Peran CRM cascading — role tenant undangan kini SELALU
// "member" (promosi admin terjadi pasca-join lewat panel anggota). Test ini
// menggantikan TestMembers_UndangRoleTetap lama yang mengunci bentuk BL-135;
// bentuk itu sudah sengaja dibongkar BL-170, bukan regresi.
func TestMembers_UndangJenisAnggotaAda(t *testing.T) {
	env, uid := setupTest(t)

	withMode(t, appmode.Multi, func() {
		html := renderMembers(t, env, uid, authz.RoleNameOwner)
		if strings.Contains(html, `id="invite-role"`) {
			t.Error("select Role tenant lama pada form Undang harus sudah tak ada (BL-170)")
		}
		if !strings.Contains(html, `id="invite-kind"`) {
			t.Error("select Jenis Anggota pada form Undang harus ada (BL-170)")
		}
	})
}
