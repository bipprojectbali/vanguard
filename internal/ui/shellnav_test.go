package ui

import (
	"strings"
	"testing"

	g "maragu.dev/gomponents"
)

// shellnav_test.go — render bentuk-bentuk NavItem baru (item mati & grup
// bersarang). Menu template di-clone; regresi di sini menular ke turunan.

func renderNav(items []NavItem, path string) string {
	var sb strings.Builder
	navList(items, path).Render(&sb)
	return sb.String()
}

// TestNavDisabled_BukanLink: item Disabled dirender <span>, BUKAN <a> (link mati
// menyesatkan & bisa difokus keyboard), teredam saja — TANPA label "Segera hadir"
// (keputusan user: cukup disable, jangan tempel badge peta jalan).
func TestNavDisabled_BukanLink(t *testing.T) {
	out := renderNav([]NavItem{{Label: "Sales", Disabled: true}}, "/w/acme")
	if !strings.Contains(out, "Sales") {
		t.Error("label item disabled harus tetap tampil")
	}
	if strings.Contains(out, "Segera hadir") {
		t.Errorf("item disabled TAK boleh menempel label 'Segera hadir':\n%s", out)
	}
	if !strings.Contains(out, "text-base-content/40") || !strings.Contains(out, "cursor-not-allowed") {
		t.Errorf("item disabled harus teredam (text-base-content/40 + cursor-not-allowed):\n%s", out)
	}
	if strings.Contains(out, "href=") {
		t.Errorf("item disabled tak boleh jadi <a href>:\n%s", out)
	}
	if !strings.Contains(out, `aria-disabled="true"`) {
		t.Error("item disabled harus aria-disabled untuk aksesibilitas")
	}
}

// TestNavGroup_HeaderTombolDanAnak: grup dirender header TOMBOL (bukan link) +
// anak-anaknya (href anak muncul). Anak disabled di dalam grup ikut teredam,
// tanpa link & tanpa label "Segera hadir".
func TestNavGroup_HeaderTombolDanAnak(t *testing.T) {
	items := []NavItem{{
		Label: "Settings",
		Children: []NavItem{
			{Label: "User Management", Href: "/w/acme/members"},
			{Label: "Integrations", Disabled: true},
		},
	}}
	out := renderNav(items, "/other")
	if !strings.Contains(out, `type="button"`) {
		t.Error("header grup harus tombol (bukan link — grup tak punya halaman)")
	}
	if !strings.Contains(out, `href="/w/acme/members"`) {
		t.Error("anak enabled harus dirender sebagai link")
	}
	if !strings.Contains(out, "Integrations") {
		t.Error("anak disabled dalam grup harus tetap tampil")
	}
	if strings.Contains(out, "Segera hadir") {
		t.Errorf("anak disabled TAK boleh menempel label 'Segera hadir':\n%s", out)
	}
	// Signal collapsible diturunkan dari label → nav_settings.
	if !strings.Contains(out, "nav_settings") {
		t.Errorf("grup harus punya signal collapsible (nav_settings):\n%s", out)
	}
}

// TestNavGroup_DefaultTertutup: tanpa anak aktif, kontainer anak disembunyikan
// inline (display:none) agar tak berkedip terbuka sebelum Datastar aktif.
func TestNavGroup_DefaultTertutup(t *testing.T) {
	items := []NavItem{{
		Label:    "Settings",
		Children: []NavItem{{Label: "User Management", Href: "/w/acme/members"}},
	}}
	out := renderNav(items, "/w/acme") // tak ada anak yang cocok
	if !strings.Contains(out, "display:none") {
		t.Errorf("grup tanpa anak aktif harus tertutup (display:none):\n%s", out)
	}
}

// TestNavGroup_AktifSaatAnakAktif: bila path cocok anak, anak itu aktif
// (aria-current) dan grup terbuka (tanpa display:none inline).
func TestNavGroup_AktifSaatAnakAktif(t *testing.T) {
	items := []NavItem{
		{Label: "Dashboard", Href: "/w/acme"},
		{Label: "Settings", Children: []NavItem{
			{Label: "Roles & Permissions", Href: "/w/acme/roles"},
		}},
	}
	out := renderNav(items, "/w/acme/roles")
	if !strings.Contains(out, `aria-current="page"`) {
		t.Error("anak yang cocok path harus aktif (aria-current)")
	}
	if strings.Contains(out, "display:none") {
		t.Errorf("grup dengan anak aktif harus terbuka (tanpa display:none):\n%s", out)
	}
	// Longest-match: Dashboard (/w/acme) TAK ikut menyala saat /w/acme/roles.
	if strings.Count(out, `aria-current="page"`) != 1 {
		t.Errorf("hanya satu item boleh aktif (longest-match):\n%s", out)
	}
}

// TestFlattenNav_LewatiHeaderDanDisabled: perhitungan active hanya atas item yang
// benar-benar bisa dituju — header grup (Href "") tak boleh salah cocok sebagai
// prefix segala path.
func TestFlattenNav_LewatiHeaderDanDisabled(t *testing.T) {
	items := []NavItem{
		{Label: "Group", Children: []NavItem{
			{Label: "Real", Href: "/w/acme/x"},
			{Label: "Dead", Disabled: true},
		}},
	}
	flat := flattenNav(items)
	if len(flat) != 1 || flat[0].Href != "/w/acme/x" {
		t.Errorf("flattenNav harus hanya menyisakan link nyata, got %+v", flat)
	}
	// Sanity: node kosong tetap merender tanpa panic.
	var sb strings.Builder
	g.Group([]g.Node{navList(items, "/")}).Render(&sb)
}
