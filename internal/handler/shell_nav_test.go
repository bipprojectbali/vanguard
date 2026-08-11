package handler

import (
	"strings"
	"testing"
	"time"

	"go_starter/internal/appmode"
	"go_starter/internal/session"
	"go_starter/internal/ui"
)

// shell_nav_test.go — /notifications tak dimiliki panel manapun, jadi menunya
// dipilih dari role. Kalau salah pilih, user melihat menu yang tak boleh ia
// akses (atau kehilangan menu panelnya saat membuka notifikasi).

// navContainsSuffix mencari item ber-href berakhiran sub di SELURUH pohon menu
// (top-level + anak grup). Sejak menu jadi bersarang (wireframe: /members,
// /settings, /roles hidup sebagai anak grup Settings), pencarian datar tak lagi
// cukup — item yang dulu di level atas kini satu tingkat lebih dalam.
func navContainsSuffix(nav []ui.NavItem, sub string) bool {
	for _, it := range nav {
		if it.Href != "" && strings.HasSuffix(it.Href, sub) {
			return true
		}
		if len(it.Children) > 0 && navContainsSuffix(it.Children, sub) {
			return true
		}
	}
	return false
}

// Role tenant kini bermuara ke ruang kerja yang SAMA (0004): menu mereka
// menunjuk /w/{slug}, bukan /admin vs /user. Yang membedakan hanya entri
// "Pengaturan" (owner/platform) — lihat TestNavFor_PengaturanIkutIzin.
func TestNavFor_MenuIkutRole(t *testing.T) {
	cases := []struct {
		role      string
		wantFirst string // href item pertama = penanda ruang yang dipilih
	}{
		{"member", "/w/acme"},
		{"admin", "/w/acme"},
		{"owner", "/w/acme"},
		{"staff", "/dev/users"},
		{"super_admin", "/dev/users"},
	}
	for _, c := range cases {
		nav := navFor(ctxWithRole(t, c.role))
		if len(nav) == 0 {
			t.Errorf("role %s: menu kosong — halaman notifikasi akan tanpa navigasi", c.role)
			continue
		}
		if nav[0].Href != c.wantFirst {
			t.Errorf("role %s: menu mulai dari %q, want %q", c.role, nav[0].Href, c.wantFirst)
		}
	}
}

// TestNavFor_PengaturanIkutIzin: entri yang PASTI ditolak tak boleh ditampilkan.
// Member melihat pintu Pengaturan lalu ditolak 403 = menu hantu — persis yang
// dihindari TestNavFor_MemberTakDapatMenuAdmin sebelum peleburan.
func TestNavFor_PengaturanIkutIzin(t *testing.T) {
	if navContainsSuffix(navFor(ctxWithRole(t, "member")), "/settings") {
		t.Error("member tak boleh melihat menu Pengaturan (route akan menolaknya)")
	}
	if !navContainsSuffix(navFor(ctxWithRole(t, "owner")), "/settings") {
		t.Error("owner harus melihat menu Pengaturan")
	}
}

// TestNavFor_AnggotaIkutIzin: menu "Anggota" mengikuti izin yang SAMA dengan
// gerbang halamannya (canManageMembers). Menu yang tampil lalu ditolak 403
// adalah menu hantu — dan di sini ia lebih buruk daripada sekadar mengganggu:
// ia menjanjikan direktori orang yang memang sengaja tak dibuka untuk member.
func TestNavFor_AnggotaIkutIzin(t *testing.T) {
	// Berlaku di KEDUA mode: pembatasan ini soal siapa yang mengelola
	// keanggotaan, bukan soal bentuk aplikasinya.
	for _, mode := range []appmode.Mode{appmode.Single, appmode.Multi} {
		withMode(t, mode, func() {
			if navContainsSuffix(navFor(ctxWithRole(t, "member")), "/members") {
				t.Errorf("mode %v: member tak boleh melihat menu Anggota", mode)
			}
			for _, role := range []string{"admin", "owner"} {
				if !navContainsSuffix(navFor(ctxWithRole(t, role)), "/members") {
					t.Errorf("mode %v: %s harus melihat menu Anggota — ia yang mengelola", mode, role)
				}
			}
		})
	}
}

// TestNavFor_MemberTakDapatMenuDev: member membuka /notifications tak boleh
// mendapati menu panel platform (menu hantu).
func TestNavFor_MemberTakDapatMenuDev(t *testing.T) {
	for _, it := range navFor(ctxWithRole(t, "member")) {
		if strings.HasPrefix(it.Href, "/dev") {
			t.Errorf("member tak boleh melihat menu %q di halaman notifikasi", it.Href)
		}
	}
}

// TestBrandFor_SelarasDenganNav: brand sidebar harus cocok dengan menu yang
// tampil. Role tenant → NAMA WORKSPACE: setelah peleburan panel, yang
// membedakan konteks adalah workspace-nya, bukan panelnya.
//
// Brand platform diuji lewat devBrand(), BUKAN string harfiah: nama aplikasi
// kini datang dari APP_NAME, dan menuliskannya di sini akan mengunci test ke
// nama template — persis hardcode yang baru saja dicabut.
func TestBrandFor_SelarasDenganNav(t *testing.T) {
	cases := map[string]string{
		"member":      "Acme",
		"admin":       "Acme",
		"owner":       "Acme",
		"super_admin": devBrand(),
	}
	for role, want := range cases {
		if got := brandFor(ctxWithRole(t, role)); got != want {
			t.Errorf("role %s: brand %q, want %q", role, got, want)
		}
	}
}

// TestBrandFor_IkutAppName: brand platform WAJIB berubah bersama APP_NAME.
// Tanpa test ini, mengembalikan hardcode tak akan memerahkan apa pun — dan
// project turunan kembali memampangkan nama template di sidebarnya.
func TestBrandFor_IkutAppName(t *testing.T) {
	prev := AppName()
	SetAppName("Vanguard")
	t.Cleanup(func() { appName = prev })

	if got := brandFor(ctxWithRole(t, "super_admin")); got != "Vanguard /dev" {
		t.Errorf("brand platform harus mengikuti APP_NAME, got %q", got)
	}
	// Nama kosong TIDAK boleh menghapus nama yang sudah ada: env yang tak diisi
	// harus jatuh ke default, bukan menghasilkan sidebar berlabel " /dev".
	SetAppName("")
	if got := brandFor(ctxWithRole(t, "super_admin")); got != "Vanguard /dev" {
		t.Errorf("APP_NAME kosong tak boleh mengosongkan brand, got %q", got)
	}
}

// TestNotifBadge_TanpaUserNil: fail-soft lapis pertama — shell dirender juga di
// jalur tanpa user; badge tak boleh memaksa query atau panic.
func TestNotifBadge_TanpaUserNil(t *testing.T) {
	env, _ := setupTest(t)
	var got *ui.NavBadge
	env.withSession(t, 0, func(sc sessionCtx) {
		got = env.h.notifBadge(sc.ctx)
	})
	if got != nil {
		t.Errorf("tanpa user login badge harus nil, got %+v", got)
	}
}

// TestNotifBadge_JumlahGabungan: badge = peristiwa belum dibaca + undangan
// pending. Undangan ikut karena ia tugas yang belum ditindak.
func TestNotifBadge_JumlahGabungan(t *testing.T) {
	env, uid := setupTest(t)
	env.mkNotif(t, uid, "member.role.changed")
	env.mkNotif(t, uid, "member.removed")
	env.mkInvite(t, "tok-badge", "test@local", "member", time.Hour)

	var got *ui.NavBadge
	env.withSession(t, uid, func(sc sessionCtx) {
		session.SetIdentity(sc.ctx, uid, "test@local", "owner", false, env.tenantID, "Test", "test", "")
		got = env.h.notifBadge(sc.ctx)
	})
	if got == nil {
		t.Fatal("badge harus ada untuk user login")
	}
	if got.Count != 3 {
		t.Errorf("badge = 2 peristiwa + 1 undangan = 3, got %d", got.Count)
	}
	if got.Item.Href != "/notifications" {
		t.Errorf("href badge salah: %q", got.Item.Href)
	}
}
