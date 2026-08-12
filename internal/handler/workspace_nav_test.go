package handler

import (
	"strings"
	"testing"

	"go_starter/internal/ui"
)

// workspace_nav_test.go — menu ruang kerja yang diselaraskan dengan wireframe CRM:
// label Inggris, modul belum-jadi tampil disabled, Settings jadi grup bersarang.
// workspaceNav = fungsi murni atas argumen izin → diuji langsung tanpa authz.

// findItem mencari item top-level berlabel tertentu.
func findItem(nav []ui.NavItem, label string) (ui.NavItem, bool) {
	for _, it := range nav {
		if it.Label == label {
			return it, true
		}
	}
	return ui.NavItem{}, false
}

// TestWorkspaceNav_EnglishTopLevel: label & urutan top-level mengikuti wireframe
// (Dashboard, Accounts, Contacts, lalu modul disabled). Dashboard selalu ada dan
// jadi item pertama (penanda ruang kerja).
func TestWorkspaceNav_EnglishTopLevel(t *testing.T) {
	nav := workspaceNav("acme", true, true, true, true, true, true, true)
	if nav[0].Label != "Dashboard" || nav[0].Href != "/w/acme" {
		t.Fatalf("item pertama harus Dashboard→/w/acme, got %q→%q", nav[0].Label, nav[0].Href)
	}
	for _, label := range []string{"Accounts", "Contacts", "Sales", "Reports", "Settings"} {
		if _, ok := findItem(nav, label); !ok {
			t.Errorf("menu wireframe kurang item %q", label)
		}
	}
	// Label lama bahasa Indonesia tak boleh tersisa.
	for _, gone := range []string{"Beranda", "Desa", "Kontak", "Peran", "Anggota", "Pengaturan"} {
		if _, ok := findItem(nav, gone); ok {
			t.Errorf("label lama %q harus sudah diganti ke Inggris", gone)
		}
	}
}

// TestWorkspaceNav_DisabledModules: modul berwireframe tapi belum berbackend
// tampil TAPI mati — Disabled=true & tanpa Href (bukan link yang lalu 404).
func TestWorkspaceNav_DisabledModules(t *testing.T) {
	nav := workspaceNav("acme", true, true, true, true, true, true, true)
	// Sales BUKAN lagi item flat disabled — kini grup bersarang (diuji terpisah).
	for _, label := range []string{"Subscriptions", "Customer Success", "Activities", "Reports"} {
		it, ok := findItem(nav, label)
		if !ok {
			t.Errorf("modul %q harus tampil (disabled), bukan hilang", label)
			continue
		}
		if !it.Disabled {
			t.Errorf("modul %q harus Disabled (backend belum ada)", label)
		}
		if it.Href != "" {
			t.Errorf("modul disabled %q tak boleh punya Href, got %q", label, it.Href)
		}
	}
}

// TestWorkspaceNav_SalesGroup: Sales = grup bersarang yang SELALU tampil. Anak
// Leads/Deals/Quotes enabled mengikuti izin (Quotes ikut canDeals — quote mewarisi
// izin baca deal); Sales Activities tetap placeholder disabled.
func TestWorkspaceNav_SalesGroup(t *testing.T) {
	// Izin CRM → Leads, Deals & Quotes enabled dengan href benar.
	nav := workspaceNav("acme", false, false, false, false, true, true, false)
	grp, ok := findItem(nav, "Sales")
	if !ok {
		t.Fatal("grup Sales harus selalu tampil (peta jalan)")
	}
	if grp.Href != "" {
		t.Errorf("header grup Sales tak boleh jadi link, got Href %q", grp.Href)
	}
	want := map[string]string{
		"Leads":  "/w/acme/leads",
		"Deals":  "/w/acme/deals",
		"Quotes": "/w/acme/quotes",
	}
	for label, href := range want {
		ch, ok := findItem(grp.Children, label)
		if !ok {
			t.Errorf("grup Sales kurang anak %q", label)
			continue
		}
		if ch.Disabled || ch.Href != href {
			t.Errorf("anak %q harus enabled→%q, got disabled=%v href=%q", label, href, ch.Disabled, ch.Href)
		}
	}
	if ch, ok := findItem(grp.Children, "Sales Activities"); !ok || !ch.Disabled {
		t.Error("placeholder Sales \"Sales Activities\" harus ada & disabled")
	}

	// Tanpa izin → grup tetap tampil, tapi Leads/Deals disabled tanpa href
	// (menu tak menawarkan pintu yang lalu ditolak 403).
	navNone := workspaceNav("acme", false, false, false, false, false, false, false)
	grpNone, ok := findItem(navNone, "Sales")
	if !ok {
		t.Fatal("grup Sales tetap tampil walau tanpa izin CRM")
	}
	for _, label := range []string{"Leads", "Deals", "Quotes"} {
		ch, ok := findItem(grpNone.Children, label)
		if !ok {
			t.Errorf("anak %q harus tetap tampil (disabled)", label)
			continue
		}
		if !ch.Disabled || ch.Href != "" {
			t.Errorf("tanpa izin, anak %q harus disabled tanpa href, got disabled=%v href=%q", label, ch.Disabled, ch.Href)
		}
	}
}

// TestWorkspaceNav_SettingsGroupChildren: Settings = grup bersarang; anak enabled
// mengikuti izin masing-masing (sumber sama dengan gerbang halaman), plus dua
// placeholder disabled. Semua izin → keenam anak hadir dengan href benar.
func TestWorkspaceNav_SettingsGroupChildren(t *testing.T) {
	nav := workspaceNav("acme", true, true, true, true, true, true, true)
	grp, ok := findItem(nav, "Settings")
	if !ok {
		t.Fatal("grup Settings harus ada saat user punya izin pengaturan")
	}
	if grp.Href != "" {
		t.Errorf("header grup Settings tak boleh jadi link, got Href %q", grp.Href)
	}
	want := map[string]string{
		"Workspace Settings":  "/w/acme/settings",
		"User Management":     "/w/acme/members",
		"Roles & Permissions": "/w/acme/roles",
		"Customization":       "/w/acme/codes",
	}
	for label, href := range want {
		ch, ok := findItem(grp.Children, label)
		if !ok {
			t.Errorf("grup Settings kurang anak %q", label)
			continue
		}
		if ch.Href != href {
			t.Errorf("anak %q href %q, want %q", label, ch.Href, href)
		}
		if ch.Disabled {
			t.Errorf("anak %q seharusnya enabled", label)
		}
	}
	for _, label := range []string{"Automation / Workflow", "Integrations"} {
		ch, ok := findItem(grp.Children, label)
		if !ok || !ch.Disabled {
			t.Errorf("placeholder %q harus ada & disabled", label)
		}
	}
}

// TestWorkspaceNav_SettingsGatingPerIzin: tiap anak enabled muncul HANYA jika
// izinnya diberikan — menu tak menawarkan pintu yang lalu ditolak 403.
func TestWorkspaceNav_SettingsGatingPerIzin(t *testing.T) {
	// Hanya canRoles → grup ada, tapi cuma Roles yang enabled (+placeholder).
	nav := workspaceNav("acme", false, false, false, false, false, false, true)
	grp, ok := findItem(nav, "Settings")
	if !ok {
		t.Fatal("canRoles saja tetap memunculkan grup Settings")
	}
	if _, ok := findItem(grp.Children, "Roles & Permissions"); !ok {
		t.Error("canRoles harus memunculkan anak Roles & Permissions")
	}
	for _, forbidden := range []string{"Workspace Settings", "User Management", "Customization"} {
		if _, ok := findItem(grp.Children, forbidden); ok {
			t.Errorf("tanpa izinnya, anak %q tak boleh muncul", forbidden)
		}
	}
}

// TestWorkspaceNav_NoSettingsGroupWithoutPerms: member tanpa izin pengaturan tak
// melihat grup Settings sama sekali (grup kosong = tak menawarkan apa pun).
func TestWorkspaceNav_NoSettingsGroupWithoutPerms(t *testing.T) {
	nav := workspaceNav("acme", false, false, true, true, false, false, false)
	if _, ok := findItem(nav, "Settings"); ok {
		t.Error("tanpa izin pengaturan, grup Settings harus disembunyikan")
	}
	// Modul disabled tetap tampil untuk semua (peta jalan produk).
	if _, ok := findItem(nav, "Sales"); !ok {
		t.Error("modul disabled tetap tampil walau tanpa izin pengaturan")
	}
}

// TestWorkspaceNav_HrefSlugBenar: semua anchor enabled menunjuk slug yang benar
// (regresi wsPath — menu bergantung slug sejak 0004).
func TestWorkspaceNav_HrefSlugBenar(t *testing.T) {
	nav := workspaceNav("beta", true, true, true, true, true, true, true)
	var check func(items []ui.NavItem)
	check = func(items []ui.NavItem) {
		for _, it := range items {
			if len(it.Children) > 0 {
				check(it.Children)
				continue
			}
			if it.Disabled || it.Href == "" {
				continue
			}
			if !strings.HasPrefix(it.Href, "/w/beta") {
				t.Errorf("href %q (%s) tak berprefiks /w/beta", it.Href, it.Label)
			}
		}
	}
	check(nav)
}
