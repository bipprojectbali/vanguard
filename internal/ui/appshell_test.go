package ui

import (
	"strings"
	"testing"

	lucide "github.com/eduardolat/gomponents-lucide"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

func renderShell(currentPath string) string {
	d := ShellData{
		Title:       "Dev",
		BrandLabel:  "go_starter /dev",
		CurrentPath: currentPath,
		UserEmail:   "owner@x.com",
		Nav: []NavItem{
			{Label: "Users", Href: "/dev/users", Icon: lucide.Users(h.Class("size-4"))},
		},
	}
	var sb strings.Builder
	AppShell(d, g.Text("konten")).Render(&sb)
	return sb.String()
}

func TestAppShell_ActiveLink(t *testing.T) {
	out := renderShell("/dev/users")
	// Menu Users harus aktif (aria-current) saat path cocok.
	if !strings.Contains(out, `aria-current="page"`) {
		t.Errorf("menu aktif harus punya aria-current:\n%s", out)
	}
	if !strings.Contains(out, `href="/dev/users"`) {
		t.Errorf("menu Users harus ada:\n%s", out)
	}
	// Konten & shell dasar.
	for _, want := range []string{"konten", "go_starter /dev", "sidebarOpen", "Keluar"} {
		if !strings.Contains(out, want) {
			t.Errorf("shell kurang %q", want)
		}
	}
	// REGRESI: data-class key ber-hyphen HARUS ter-quote (kalau tidak, ekspresi
	// JS invalid → Datastar mati: "Unexpected token '-'").
	if !strings.Contains(out, `{&#39;translate-x-0&#39;: $sidebarOpen}`) {
		t.Errorf("data-class key harus ter-quote (hyphen tak valid tanpa kutip):\n%s", out)
	}
}

func TestAppShell_InactiveWhenPathDiffers(t *testing.T) {
	out := renderShell("/dev/other")
	if strings.Contains(out, `aria-current="page"`) {
		t.Errorf("menu tak boleh aktif saat path beda:\n%s", out)
	}
}

// TestActiveNavHref: longest-match menang — sub-route TIDAK mengaktifkan parent
// (regresi bug "/admin/workspace" bikin Dashboard + Workspace dua-duanya menyala).
func TestActiveNavHref(t *testing.T) {
	nav := []NavItem{
		{Label: "Dashboard", Href: "/admin"},
		{Label: "Workspace", Href: "/admin/workspace"},
	}
	cases := []struct{ path, want string }{
		{"/admin", "/admin"},                       // exact → Dashboard
		{"/admin/workspace", "/admin/workspace"},   // sub-route → Workspace SAJA (bukan /admin)
		{"/admin/workspace/x", "/admin/workspace"}, // lebih dalam → tetap Workspace
		{"/adminX", ""},                            // batas segmen: /adminX BUKAN di bawah /admin
		{"/other", ""},                             // tak cocok
	}
	for _, c := range cases {
		if got := activeNavHref(nav, c.path); got != c.want {
			t.Errorf("activeNavHref(%q)=%q, want %q", c.path, got, c.want)
		}
	}
}

// TestActiveNavHref_Root: item "/" hanya aktif pada path persis "/" (tak cocok semua).
func TestActiveNavHref_Root(t *testing.T) {
	nav := []NavItem{{Label: "Home", Href: "/"}}
	if got := activeNavHref(nav, "/"); got != "/" {
		t.Errorf("root exact harus aktif, got %q", got)
	}
	if got := activeNavHref(nav, "/dev"); got != "" {
		t.Errorf("root TAK boleh aktif di /dev, got %q", got)
	}
}

// TestAppShell_RailCollapseHooks (BL-64): header & footer sidebar membawa kait
// collapse (app-shellhead/app-shelluser) agar input.css bisa men-tengah-kan
// tombol collapse dan menumpuk avatar+tema saat rail 4rem. Regresi: menghapus
// kait mengembalikan tombol mepet tepi / tombol tema terpotong keluar rail.
func TestAppShell_RailCollapseHooks(t *testing.T) {
	out := renderShell("/dev/users")
	for _, want := range []string{"app-shellhead", "app-shelluser"} {
		if !strings.Contains(out, want) {
			t.Errorf("shell harus membawa kait collapse %q (BL-64):\n%s", want, out)
		}
	}
}

func TestWhen(t *testing.T) {
	var yes, no strings.Builder
	When(true, g.Text("TAMPIL")).Render(&yes)
	When(false, g.Text("TAMPIL")).Render(&no)
	if !strings.Contains(yes.String(), "TAMPIL") {
		t.Error("When(true) harus render node")
	}
	if strings.Contains(no.String(), "TAMPIL") {
		t.Error("When(false) tak boleh render node")
	}
}
