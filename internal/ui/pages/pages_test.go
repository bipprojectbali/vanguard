package pages

import (
	"strings"
	"testing"

	g "maragu.dev/gomponents"
)

func render(t *testing.T, node g.Node) string {
	t.Helper()
	var sb strings.Builder
	if err := node.Render(&sb); err != nil {
		t.Fatalf("render: %v", err)
	}
	return sb.String()
}

func TestLogin_GoogleOnly_NoPasswordForm(t *testing.T) {
	out := render(t, Login("", "Acme")) // Google-only: tak ada form password sama sekali
	if !strings.Contains(out, "/api/auth/google") || !strings.Contains(out, "Lanjutkan dengan Google") {
		t.Errorf("tombol Google harus selalu ada:\n%s", out)
	}
	if strings.Contains(out, `type="password"`) {
		t.Errorf("halaman login tak lagi punya form password:\n%s", out)
	}
}

// TestLogin_RendersError: ?err=inactive juga datang dari jalur Google di
// produksi (oauth_google.go) — errMsg wajib selalu dirender sebagai alert.
func TestLogin_RendersError(t *testing.T) {
	out := render(t, Login("Akun tidak aktif. Hubungi administrator.", "Acme"))
	if !strings.Contains(out, "Akun tidak aktif. Hubungi administrator.") {
		t.Errorf("errMsg harus dirender sebagai alert:\n%s", out)
	}
}

// TestLogin_RendersBrandAndCopy: elemen mockup Penpot "Auth Flow — 2. Landing"
// (logo+brand, badge internal, headline, subtext, disclaimer) harus ada.
// Brand DIOPER dari handler (APP_NAME), tak ditulis view — tanpa ini setiap
// project turunan template memampangkan nama project lain.
func TestLogin_RendersBrandAndCopy(t *testing.T) {
	out := render(t, Login("", "Vanguard CRM"))
	for _, want := range []string{
		"Vanguard CRM",
		"INTERNAL USE ONLY",
		"Kelola relasi pelanggan desa dengan satu platform",
		"Sales, Customer Success, Support",
		"Dengan melanjutkan, Anda menyetujui penggunaan data sesuai kebijakan internal.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("login kurang %q:\n%s", want, out)
		}
	}
}

func TestRegister(t *testing.T) {
	out := render(t, Register("", true))
	for _, want := range []string{`method="post"`, `action="/register"`, `type="password"`, `name="workspace"`, "/login"} {
		if !strings.Contains(out, want) {
			t.Errorf("register kurang %q:\n%s", want, out)
		}
	}
}

// TestRegister_TanpaFieldWorkspace: REGRESI (ditemukan lewat browser, bukan test).
// Field "Nama Workspace" dulu selalu tampil, sementara handler menolak nama
// kosong — jadi di mode single pendaftaran MUSTAHIL diselesaikan: formnya
// menanyakan sesuatu yang tak relevan, lalu server menolak karena tak diisi.
//
// Gejalanya menyesatkan: redirect ke /register?err=workspace pada form yang
// tampak lengkap. Ini juga alasan verifikasi UI harus mencakup SUBMIT, bukan
// cuma render.
func TestRegister_TanpaFieldWorkspace(t *testing.T) {
	out := render(t, Register("", false))
	if strings.Contains(out, `name="workspace"`) {
		t.Errorf("mode single tak boleh menanyakan nama workspace:\n%s", out)
	}
	// Sisanya harus tetap utuh — yang hilang hanya satu field.
	for _, want := range []string{`action="/register"`, `type="password"`, `type="email"`} {
		if !strings.Contains(out, want) {
			t.Errorf("register kurang %q:\n%s", want, out)
		}
	}
}

func TestLanding(t *testing.T) {
	// Login → CTA ke home per-role (homePath yang dioper), teks "Buka aplikasi".
	in := render(t, Landing(true, "/dev", "Acme"))
	if !strings.Contains(in, `href="/dev"`) || !strings.Contains(in, "Buka aplikasi") {
		t.Errorf("landing login harus CTA ke homePath:\n%s", in)
	}
	// Anonim → CTA /login, homePath diabaikan.
	anon := render(t, Landing(false, "/dev", "Acme"))
	if !strings.Contains(anon, `href="/login"`) {
		t.Errorf("landing anonim harus CTA /login:\n%s", anon)
	}
	if strings.Contains(anon, `href="/dev"`) {
		t.Errorf("landing anonim tak boleh link ke home:\n%s", anon)
	}
	// Nama aplikasi DIOPER, tak ditulis view: tanpa ini setiap project turunan
	// memampangkan nama template di halaman depannya.
	if !strings.Contains(in, "Acme") {
		t.Errorf("landing harus menampilkan nama aplikasi yang dioper:\n%s", in)
	}
}
