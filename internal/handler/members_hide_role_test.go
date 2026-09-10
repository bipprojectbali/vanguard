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
// pembeda per-baris = `class="select select-sm" name="role"` (memberRoleSelect);
// select role pada form UNDANG (`class="select" name="role"`, id=invite-role)
// SENGAJA dibiarkan, jadi kehadiran `name="role"` telanjang bukan pelanggaran.

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

// TestMembers_UndangRoleTetap: keputusan desain BL-135 — HANYA select per baris
// yang disembunyikan; role awal saat mengundang tetap bisa dipilih. Test ini
// mengunci batas itu agar penghapusan tak merembet ke form undang.
func TestMembers_UndangRoleTetap(t *testing.T) {
	env, uid := setupTest(t)

	withMode(t, appmode.Multi, func() {
		html := renderMembers(t, env, uid, authz.RoleNameOwner)
		if !strings.Contains(html, `id="invite-role"`) {
			t.Error("select Role pada form Undang harus tetap ada (di luar cakupan BL-135)")
		}
	})
}
